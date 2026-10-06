// Package layout places the two subtitle lines over the video. It is pure
// geometry: text measurement comes from a Measurer, so every resolution, DPI
// and text length can be tested without a window.
package layout

import (
	"math"
	"sort"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
)

const (
	// BaseDPI is the display density at 100% scaling.
	BaseDPI = 96
	// minFontPixels keeps text readable on tiny windows (at 100% scaling).
	minFontPixels = 10
	// minShrink is how far a block may shrink to fit its maximum lines before
	// it takes extra lines instead; text is never cut.
	minShrink = 0.8
	// shrinkStep is the size reduction tried at each step toward minShrink.
	shrinkStep = 0.05
	// maxTextAreaShare is the most of the picture height one block may take
	// (long subtitles must not cover the screen); past it the block
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

// Notice is a short message along the top of the picture, on the side its
// style's alignment names.
type Notice struct {
	Text
	// Margin keeps it off the picture's edges, as a share of the picture
	// height; MaxWidth is the most of the picture width it may take.
	Margin, MaxWidth float64
	// Meter is a level drawn under the text, inside the notice's box; nil
	// draws none.
	Meter *Meter
}

// Meter is a level, such as the volume's. Level and Mark are shares of the
// bar's length: how far it is filled, and where the usual range ends (100% of
// the sound); a mark at the end or past it marks nothing.
type Meter struct {
	Level, Mark float64
}

// PlacedMeter is a meter laid out: its bar, under its notice's text.
type PlacedMeter struct {
	Meter
	Bar Rect
}

// The meter's geometry: its gap below the text and its thickness in style
// units, the thinnest bar in pixels, and its shortest length in ems of the
// notice's font, so the box keeps its width while the level changes.
const (
	MeterGap         = 6.0
	MeterThickness   = 6.0
	MeterMinPixels   = 3.0
	meterMinLengthEm = 7.0
)

// Input is everything the layout depends on.
type Input struct {
	// Video is the picture area, in overlay pixels.
	Video Rect
	// DPI is the density of the display showing the video (96 = 100%).
	DPI         int
	Layout      appearance.Layout
	Primary     Text
	Translation Text
	Notice      Notice
	// Floor is the lowest y the subtitles may reach, in overlay pixels, so
	// controls shown at the bottom never cover them; zero means no limit.
	Floor float64
	// Cue is the karaoke mode's indicator over a previewed primary line, nil
	// when none shows: the translation keeps clear of it.
	Cue *CueRoom
}

// CueRoom is the indicator drawn above a previewed primary line: the seconds
// left, CountdownSize times the line's font, or a bar BarThickness style units
// thick. Either sits CueGap style units above the text.
type CueRoom struct {
	Countdown     bool
	CountdownSize float64
	BarThickness  float64
}

// The indicator's geometry, shared with the renderer so the room kept is the
// room drawn: its gap above the text in style units, and the thinnest bar in
// pixels.
const (
	CueGap          = 8.0
	CueBarMinPixels = 3.0
)

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
	Notice      *Block
	// Meter is the notice's level, nil when it has none.
	Meter *PlacedMeter
}

// Compute places both subtitles. Alone, each sits where its style puts it;
// together they keep the order the layout names, whatever their sizes, so a
// long translation never jumps to the other side of the study line. The
// translation takes its space from its text whether it is visible or not.
func Compute(in Input, m Measurer) Frame {
	video := in.Video
	if !video.Valid() {
		return Frame{}
	}
	area := safeArea(video, in.Layout)
	if in.Floor > 0 && in.Floor < area.Bottom() {
		area.H = math.Max(in.Floor-area.Y, 0)
	}
	// The edge is as low as a line may go when nothing else fits: the
	// picture's own, above the controls.
	edge := video.Bottom()
	if in.Floor > 0 {
		edge = math.Min(edge, in.Floor)
	}
	edge = math.Max(edge, area.Bottom())
	primary := wrapBlock(in.Primary, in, area, m)
	translation := wrapBlock(in.Translation, in, area, m)

	for _, b := range []*shaped{primary, translation} {
		if b != nil {
			b.bottom = clampBottom(b.anchor, b.height(), area)
		}
	}
	if primary != nil && translation != nil {
		placePair(primary, translation, in, area, edge, m)
	}
	notice, meter := placeNotice(in, m)
	return Frame{Primary: primary.finish(area), Translation: translation.finish(area), Notice: notice, Meter: meter}
}

// placeNotice puts the notice at the top of the picture, on its side, away
// from the subtitles whatever their margins, with its meter under the text.
func placeNotice(in Input, m Measurer) (*Block, *PlacedMeter) {
	video := in.Video
	margin := in.Notice.Margin * video.H
	width := math.Min(in.Notice.MaxWidth*video.W, math.Max(video.W-2*margin, 0))
	left := video.X + margin
	switch in.Notice.Style.Alignment {
	case appearance.AlignCenter:
		left = video.X + (video.W-width)/2
	case appearance.AlignRight:
		left = video.Right() - margin - width
	}
	area := Rect{X: left, Y: video.Y + margin, W: width, H: math.Max(video.H-2*margin, 0)}
	notice := wrapBlock(in.Notice.Text, in, area, m)
	if notice == nil {
		return nil, nil
	}
	var gap, thickness float64
	if in.Notice.Meter != nil {
		gap, thickness = MeterGap*notice.scale, math.Max(MeterThickness*notice.scale, MeterMinPixels)
		notice.below = gap + thickness
		notice.minWidth = math.Min(meterMinLengthEm*notice.font.Size, notice.maxWidth)
	}
	notice.bottom = clampBottom(area.Y+notice.height(), notice.height(), area)
	block := notice.finish(area)
	if in.Notice.Meter == nil {
		return block, nil
	}
	inside := block.Bounds
	bar := Rect{
		X: inside.X + notice.padding, Y: inside.Bottom() - notice.padding - thickness,
		W: math.Max(inside.W-2*notice.padding, 0), H: thickness,
	}
	return block, &PlacedMeter{Meter: *in.Notice.Meter, Bar: bar}
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
	// baseSize is the style's size in pixels and minSize the readable
	// minimum.
	baseSize float64
	minSize  float64
	lines    []string
	widths   []float64
	// lineHeight is the distance between lines, the font's own (natural)
	// times the style's line spacing.
	lineHeight float64
	natural    float64
	padding    float64
	bottom     float64
	// below is room kept under the lines inside the box, and minWidth the
	// narrowest the text inside it may be, for a notice's meter.
	below, minWidth float64
}

func (s *shaped) height() float64 {
	return float64(len(s.lines))*s.lineHeight + s.below + 2*s.padding
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
	s.natural = m.LineHeight(s.font)
	s.lineHeight = s.natural * spacingOf(s.style)
}

// spacingOf is the style's line spacing; a validated style always has one,
// anything else keeps the font's own.
func spacingOf(style appearance.SubtitleStyle) float64 {
	if !isFinitePositive(style.LineSpacing) {
		return 1
	}
	return style.LineSpacing
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
	return float64(in.DPI) / BaseDPI
}

func wrapBlock(text Text, in Input, area Rect, m Measurer) *shaped {
	paragraphs := paragraphsOf(text.Content)
	if len(paragraphs) == 0 {
		return nil
	}
	style := text.Style
	scale := scaleFor(style, in)
	minPixels := minFontPixels * max(float64(in.DPI), BaseDPI) / BaseDPI
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
	return s
}

// placePair puts the two lines in the layout's order. The lower one sits at
// the lower of the two styles' heights; the upper one stays at its own height,
// or rises just above the lower one (by the gap, and the karaoke indicator's
// room) when they would touch. When the upper one would leave the top of the
// area, the lower one shrinks (to the usual 80%, then to the minimum) and then
// goes down to the edge of the picture; after that the upper one shrinks too.
// Only when nothing fits even so do they overlap, the upper one still on top.
func placePair(primary, translation *shaped, in Input, area Rect, edge float64, m Measurer) {
	upper, lower := primary, translation
	if in.Layout.TranslationOnTop {
		upper, lower = translation, primary
	}
	cue := primary.headroom(in.Cue, m)
	base, room := math.Max(primary.anchor, translation.anchor), area
	stack := func() bool {
		lower.bottom = clampBottom(base, lower.height(), room)
		apart := in.Layout.LineGap * translation.lineHeight
		top := 0.0
		if lower == primary {
			apart += cue
		} else {
			top = cue
		}
		upper.bottom = math.Min(clampBottom(upper.anchor, upper.height(), area), lower.top()-apart)
		return upper.top()-top >= area.Y-sizeEpsilon
	}
	if stack() {
		return
	}
	for _, floor := range []float64{lower.baseSize * minShrink, lower.minSize} {
		if lower.shrinkTo(floor, m, stack) {
			return
		}
	}
	base, room.H = edge, math.Max(edge-area.Y, area.H)
	if stack() {
		return
	}
	for _, floor := range []float64{upper.baseSize * minShrink, upper.minSize} {
		if upper.shrinkTo(floor, m, stack) {
			return
		}
	}
	upper.bottom = math.Min(area.Y+upper.height(), lower.bottom)
}

// headroom is the room the karaoke indicator takes above a previewed line,
// measured as the renderer draws it; zero without one.
func (s *shaped) headroom(cue *CueRoom, m Measurer) float64 {
	if s == nil || cue == nil {
		return 0
	}
	gap := CueGap * s.scale
	if cue.Countdown {
		font := s.font
		font.Size *= cue.CountdownSize
		return gap + m.LineHeight(font)
	}
	return gap + math.Max(cue.BarThickness*s.scale, CueBarMinPixels)
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
	// The room the line spacing adds goes half above and half below each
	// line, as CSS does.
	leading := (s.lineHeight - s.natural) / 2
	widest := 0.0
	for i, text := range s.lines {
		width := s.widths[i]
		widest = math.Max(widest, width)
		block.Lines[i] = Line{
			Text: text, X: alignX(s.style.Alignment, width, area, s.padding),
			Y: top + float64(i)*s.lineHeight + leading, Width: width, Height: s.natural,
		}
	}
	widest = math.Max(widest, s.minWidth)
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
