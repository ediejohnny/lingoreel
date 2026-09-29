// Package layout places the two subtitle lines over the video. It is pure
// geometry: text measurement comes from a Measurer, so every resolution, DPI
// and text length can be tested without a window.
package layout

import (
	"math"
	"sort"
	"strings"

	"lingoreel/internal/domain/appearance"
)

const (
	// baseDPI is the display density at 100% scaling.
	baseDPI = 96
	// minFontPixels keeps text readable on tiny windows (at 100% scaling).
	minFontPixels = 10
	// minShrink is how far a block may shrink to fit its maximum lines before
	// it takes extra lines instead; text is never cut.
	minShrink = 0.8
	// shrinkStep is the size reduction tried at each step toward minShrink.
	shrinkStep = 0.05
	// minGapLines is the smallest gap between the two blocks, in translation
	// line heights.
	minGapLines = 0.2
	// maxTextAreaShare is the most of the picture height one block may take
	// (PRD §34: long subtitles must not cover the screen); past it the block
	// keeps shrinking down to the minimum size.
	maxTextAreaShare = 0.35
	// sizeEpsilon absorbs float error when comparing sizes and positions.
	sizeEpsilon = 1e-9
)

// Font is a concrete font in pixels.
type Font struct {
	Family string
	Size   float64
	Bold   bool
	Italic bool
}

// Measurer measures text in a font. The renderer implements it, so layout and
// drawing agree on every width.
type Measurer interface {
	// Width returns the advance width of a single line of text.
	Width(text string, font Font) float64
	// LineHeight returns the distance between the tops of consecutive lines.
	LineHeight(font Font) float64
}

// Text is one subtitle line to place: its style and what it says. An empty
// Content means there is nothing on that line.
type Text struct {
	Style   appearance.SubtitleStyle
	Content string
}

// Input is everything the layout depends on.
type Input struct {
	// Video is the picture area, in overlay pixels.
	Video Rect
	// DPI is the density of the display showing the video (96 = 100%).
	DPI         int
	Layout      appearance.Layout
	Primary     Text
	Translation Text
}

// Line is one wrapped line of text; X and Y are its top left corner.
type Line struct {
	Text   string
	X, Y   float64
	Width  float64
	Height float64
}

// Block is a placed subtitle.
type Block struct {
	Font Font
	// Scale converts style sizes (outline, shadow, padding) into pixels.
	Scale float64
	Lines []Line
	// Bounds covers every line plus the background padding.
	Bounds Rect
}

// Frame is the result of a layout; a nil block has nothing to show.
type Frame struct {
	Primary     *Block
	Translation *Block
}

// Compute places both subtitles. The primary is placed from its own style
// alone, so it never moves when a translation appears, disappears or grows
// (DESIGN §23); the translation finds room around it. The translation also
// takes its space from its text whether it is visible or not.
func Compute(in Input, m Measurer) Frame {
	video := in.Video
	if !video.Valid() {
		return Frame{}
	}
	area := safeArea(video, in.Layout)
	primary := wrapBlock(in.Primary, in, area, m)
	translation := wrapBlock(in.Translation, in, area, m)

	for _, b := range []*shaped{primary, translation} {
		if b != nil {
			b.bottom = clampBottom(b.anchor, b.height(), area)
		}
	}
	if primary != nil && translation != nil {
		placeTranslation(primary, translation, area, m)
	}
	return Frame{Primary: primary.finish(area), Translation: translation.finish(area)}
}

// shaped is a block whose lines are wrapped but not yet placed vertically.
type shaped struct {
	style appearance.SubtitleStyle
	font  Font
	scale float64
	// anchor is where the style asks the block's bottom to be.
	anchor     float64
	paragraphs []string
	maxWidth   float64
	// baseSize is the style's size in pixels, minSize the readable minimum
	// and fittedSize the size after the block's own limits.
	baseSize   float64
	minSize    float64
	fittedSize float64
	lines      []string
	widths     []float64
	lineHeight float64
	padding    float64
	bottom     float64
}

func (s *shaped) height() float64 {
	return float64(len(s.lines))*s.lineHeight + 2*s.padding
}

func (s *shaped) top() float64 {
	return s.bottom - s.height()
}

// resize wraps the text again at a new font size, never below the minimum.
func (s *shaped) resize(size float64, m Measurer) {
	s.font.Size = max(size, s.minSize)
	s.lines = wrapAll(s.paragraphs, s.font, s.maxWidth, m)
	s.widths = make([]float64, len(s.lines))
	for i, line := range s.lines {
		s.widths[i] = m.Width(line, s.font)
	}
	s.lineHeight = m.LineHeight(s.font)
}

// shrinkTo lowers the font in steps of the base size until fits holds or the
// size reaches floor; it reports whether fits holds at the end.
func (s *shaped) shrinkTo(floor float64, m Measurer, fits func() bool) bool {
	floor = max(floor, s.minSize)
	for !fits() {
		if s.font.Size <= floor+sizeEpsilon {
			return false
		}
		s.resize(max(s.font.Size-shrinkStep*s.baseSize, floor), m)
	}
	return true
}

// scaleFor converts style units into pixels: responsive styles follow the
// video height, fixed ones follow the display density.
func scaleFor(style appearance.SubtitleStyle, in Input) float64 {
	if style.SizeMode == appearance.SizeResponsive {
		return in.Video.H / appearance.ReferenceVideoHeight
	}
	if in.DPI <= 0 {
		return 1
	}
	return float64(in.DPI) / baseDPI
}

func wrapBlock(text Text, in Input, area Rect, m Measurer) *shaped {
	paragraphs := paragraphsOf(text.Content)
	if len(paragraphs) == 0 {
		return nil
	}
	style := text.Style
	scale := scaleFor(style, in)
	minPixels := minFontPixels * max(float64(in.DPI), baseDPI) / baseDPI
	size := max(style.FontSize*scale, minPixels)
	if !isFinitePositive(size) || !isFinitePositive(scale) {
		return nil
	}
	padding := 0.0
	if style.Background.Enabled {
		padding = style.Background.Padding * scale
	}
	s := &shaped{
		style: style, scale: scale, anchor: in.Video.Y + style.VerticalPosition*in.Video.H,
		font:       Font{Family: style.FontFamily, Bold: style.Bold, Italic: style.Italic},
		paragraphs: paragraphs, maxWidth: math.Max(area.W-2*padding, 1),
		baseSize: size, minSize: minPixels, padding: padding,
	}
	s.resize(size, m)
	maxLines := max(style.MaxLines, 1)
	s.shrinkTo(size*minShrink, m, func() bool { return len(s.lines) <= maxLines })
	// Text is never cut: past the line limit it takes extra lines, but it
	// keeps shrinking while it covers too much of the picture.
	s.shrinkTo(minPixels, m, func() bool { return s.height() <= maxTextAreaShare*in.Video.H+sizeEpsilon })
	s.fittedSize = s.font.Size
	return s
}

// placeTranslation puts the translation next to the primary, which stays
// where its style put it. The translation keeps its side (below or above the
// primary, as the styles say) and shrinks down to the usual 80% to fit there,
// then tries the other side, then shrinks down to the minimum size. When
// nothing fits even then, it stays on its side at the edge of the safe area
// and may overlap the primary, which still does not move.
func placeTranslation(primary, translation *shaped, area Rect, m Measurer) {
	sides := []bool{translation.bottom >= primary.bottom, translation.bottom < primary.bottom}
	for _, floor := range []float64{translation.baseSize * minShrink, translation.minSize} {
		for _, below := range sides {
			translation.resize(translation.fittedSize, m)
			if translation.shrinkTo(floor, m, func() bool { return translation.fitBeside(primary, below, area) }) {
				return
			}
		}
	}
	if sides[0] {
		translation.bottom = area.Bottom()
	} else {
		translation.bottom = math.Min(area.Y+translation.height(), area.Bottom())
	}
}

// fitBeside places the block below or above the primary, as close to its own
// anchor as the gap allows, and reports whether it stays inside the area.
func (s *shaped) fitBeside(primary *shaped, below bool, area Rect) bool {
	gap := minGapLines * s.lineHeight
	anchored := clampBottom(s.anchor, s.height(), area)
	if below {
		s.bottom = math.Max(anchored, primary.bottom+gap+s.height())
		return s.bottom <= area.Bottom()+sizeEpsilon
	}
	s.bottom = math.Min(anchored, primary.top()-gap)
	return s.top() >= area.Y-sizeEpsilon
}

// clampBottom keeps a block inside the safe area, favoring its bottom edge
// when the block is taller than the area.
func clampBottom(bottom, height float64, area Rect) float64 {
	bottom = math.Min(bottom, area.Bottom())
	return math.Max(bottom, math.Min(area.Y+height, area.Bottom()))
}

func (s *shaped) finish(area Rect) *Block {
	if s == nil {
		return nil
	}
	top := s.bottom - s.height() + s.padding
	block := &Block{Font: s.font, Scale: s.scale, Lines: make([]Line, len(s.lines))}
	widest := 0.0
	for i, text := range s.lines {
		width := s.widths[i]
		widest = math.Max(widest, width)
		block.Lines[i] = Line{
			Text: text, X: alignX(s.style.Alignment, width, area, s.padding),
			Y: top + float64(i)*s.lineHeight, Width: width, Height: s.lineHeight,
		}
	}
	left := alignX(s.style.Alignment, widest, area, s.padding)
	block.Bounds = Rect{
		X: left - s.padding, Y: top - s.padding,
		W: widest + 2*s.padding, H: s.height(),
	}
	return block
}

func alignX(alignment appearance.Alignment, width float64, area Rect, padding float64) float64 {
	switch alignment {
	case appearance.AlignLeft:
		return area.X + padding
	case appearance.AlignRight:
		return area.Right() - padding - width
	default:
		return area.X + (area.W-width)/2
	}
}

// safeArea is the part of the video where text may go: inside the side, top
// and bottom margins, and no wider than the maximum width, centered.
func safeArea(video Rect, layout appearance.Layout) Rect {
	left := video.X + layout.MarginLeft*video.W
	right := video.Right() - layout.MarginRight*video.W
	width := math.Max(right-left, 0)
	if limit := layout.MaxWidth * video.W; limit > 0 && limit < width {
		left += (width - limit) / 2
		width = limit
	}
	top := video.Y + layout.MarginTop*video.H
	height := math.Max(video.H*(1-layout.MarginTop-layout.MarginBottom), 0)
	return Rect{X: left, Y: top, W: width, H: height}
}

// paragraphsOf splits the text at its explicit line breaks, dropping empty ones.
func paragraphsOf(content string) []string {
	var paragraphs []string
	for part := range strings.SplitSeq(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if words := strings.Fields(part); len(words) > 0 {
			paragraphs = append(paragraphs, strings.Join(words, " "))
		}
	}
	return paragraphs
}

func wrapAll(paragraphs []string, font Font, maxWidth float64, m Measurer) []string {
	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, wrap(paragraph, font, maxWidth, m)...)
	}
	return lines
}

// wrap breaks a paragraph greedily at spaces; a word wider than the line is
// broken between characters.
func wrap(paragraph string, font Font, maxWidth float64, m Measurer) []string {
	var lines []string
	current := ""
	for word := range strings.FieldsSeq(paragraph) {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if m.Width(candidate, font) <= maxWidth {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		pieces := breakWord(word, font, maxWidth, m)
		lines = append(lines, pieces[:len(pieces)-1]...)
		current = pieces[len(pieces)-1]
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// breakWord splits a word into pieces that fit, at least one character each.
func breakWord(word string, font Font, maxWidth float64, m Measurer) []string {
	if m.Width(word, font) <= maxWidth {
		return []string{word}
	}
	var pieces []string
	runes := []rune(word)
	for len(runes) > 0 {
		// The longest prefix that fits, found by binary search: measuring
		// every prefix would be quadratic on a hostile thousand-letter word.
		fits := sort.Search(len(runes), func(i int) bool {
			return m.Width(string(runes[:i+1]), font) > maxWidth
		})
		n := max(fits, 1)
		pieces = append(pieces, string(runes[:n]))
		runes = runes[n:]
	}
	return pieces
}

func isFinitePositive(x float64) bool {
	return x > 0 && !math.IsInf(x, 1)
}
