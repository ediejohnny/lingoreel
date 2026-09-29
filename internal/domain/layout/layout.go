// Package layout places the two subtitle lines over the video. It is pure
// geometry: text measurement comes from a Measurer, so every resolution, DPI
// and text length can be tested without a window.
package layout

import (
	"math"
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

// Compute places both subtitles. The translation takes its space from its
// text whether it is visible or not, so the primary line never moves when the
// translation is shown or hidden.
func Compute(in Input, m Measurer) Frame {
	video := in.Video
	if !video.Valid() {
		return Frame{}
	}
	area := safeArea(video, in.Layout)
	primary := wrapBlock(in.Primary, in, area, m)
	translation := wrapBlock(in.Translation, in, area, m)

	if primary != nil && translation != nil {
		gap := minGapLines * translation.lineHeight
		placeBoth(primary, translation, video, area, gap)
	} else {
		for _, b := range []*shaped{primary, translation} {
			if b != nil {
				b.bottom = clampBottom(anchor(b.style, video), b.height(), area)
			}
		}
	}
	return Frame{Primary: primary.finish(area), Translation: translation.finish(area)}
}

// shaped is a block whose lines are wrapped but not yet placed vertically.
type shaped struct {
	style      appearance.SubtitleStyle
	font       Font
	scale      float64
	lines      []string
	widths     []float64
	lineHeight float64
	padding    float64
	bottom     float64
}

func (s *shaped) height() float64 {
	return float64(len(s.lines))*s.lineHeight + 2*s.padding
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
	maxWidth := math.Max(area.W-2*padding, 1)
	maxLines := max(style.MaxLines, 1)

	font := Font{Family: style.FontFamily, Size: size, Bold: style.Bold, Italic: style.Italic}
	lines := wrapAll(paragraphs, font, maxWidth, m)
	for factor := 1 - shrinkStep; len(lines) > maxLines && factor >= minShrink-1e-9; factor -= shrinkStep {
		smaller := font
		smaller.Size = max(size*factor, minPixels)
		lines = wrapAll(paragraphs, smaller, maxWidth, m)
		font = smaller
	}

	widths := make([]float64, len(lines))
	for i, line := range lines {
		widths[i] = m.Width(line, font)
	}
	return &shaped{
		style: style, font: font, scale: scale, lines: lines, widths: widths,
		lineHeight: m.LineHeight(font), padding: padding,
	}
}

// placeBoth resolves the two anchors so the blocks never overlap: the
// translation moves away from the primary first, and the primary only moves
// when the safe area leaves no other room.
func placeBoth(primary, translation *shaped, video, area Rect, gap float64) {
	pBottom := clampBottom(anchor(primary.style, video), primary.height(), area)
	tBottom := clampBottom(anchor(translation.style, video), translation.height(), area)
	pTop := pBottom - primary.height()
	tTop := tBottom - translation.height()

	if pBottom <= tBottom {
		// Translation below the primary.
		if overlap := pBottom + gap - tTop; overlap > 0 {
			shift := math.Max(math.Min(overlap, area.Bottom()-tBottom), 0)
			tBottom += shift
			tTop += shift
			if pBottom+gap > tTop {
				pBottom = math.Max(tTop-gap, area.Y+primary.height())
			}
		}
	} else if overlap := tBottom + gap - pTop; overlap > 0 {
		// Translation above the primary.
		shift := math.Max(math.Min(overlap, tTop-area.Y), 0)
		tBottom -= shift
		if tBottom+gap > pTop {
			pBottom = math.Min(tBottom+gap+primary.height(), area.Bottom())
		}
	}
	primary.bottom = pBottom
	translation.bottom = tBottom
}

func anchor(style appearance.SubtitleStyle, video Rect) float64 {
	return video.Y + style.VerticalPosition*video.H
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

// safeArea is the part of the video where text may go: inside the side and
// bottom margins, and no wider than the maximum width, centered.
func safeArea(video Rect, layout appearance.Layout) Rect {
	left := video.X + layout.MarginLeft*video.W
	right := video.Right() - layout.MarginRight*video.W
	width := math.Max(right-left, 0)
	if limit := layout.MaxWidth * video.W; limit > 0 && limit < width {
		left += (width - limit) / 2
		width = limit
	}
	return Rect{X: left, Y: video.Y, W: width, H: video.H - layout.MarginBottom*video.H}
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
		n := 1
		for n < len(runes) && m.Width(string(runes[:n+1]), font) <= maxWidth {
			n++
		}
		pieces = append(pieces, string(runes[:n]))
		runes = runes[n:]
	}
	return pieces
}

func isFinitePositive(x float64) bool {
	return x > 0 && !math.IsInf(x, 1)
}
