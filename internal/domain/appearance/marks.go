package appearance

import "strings"

// Ranges accepted for the notices, the selection outline and the karaoke
// mode's indicator.
const (
	MaxNoticeFontSize = 72.0
	// MaxNoticeMargin and the widths are shares of the picture.
	MaxNoticeMargin   = 0.2
	MinNoticeMaxWidth = 0.2
	MinSelectionWidth = 0.5
	MaxSelectionWidth = 8.0
	MaxSelectionGap   = 20.0
	MaxSelectionCurve = 20.0
	MinCueThickness   = 1.0
	MaxCueThickness   = 20.0
	MinCountdownSize  = 0.4
	MaxCountdownSize  = 2.0
)

// NoticeLook is how the short notices over the video look, such as which
// subtitle a sync shortcut moved: small text on a see-through box, at the
// top of the picture, far from the subtitles (the overlay shows only
// subtitles, except these notices).
type NoticeLook struct {
	FontFamily string  `json:"fontFamily"`
	FontSize   float64 `json:"fontSize"`
	Color      Color   `json:"color"`
	Background Color   `json:"background"`
	// Side is where along the top the notice sits.
	Side Alignment `json:"side"`
	// Margin keeps the notice off the picture's edges, as a share of its
	// height; MaxWidth is the most of its width the notice may take.
	Margin   float64 `json:"margin"`
	MaxWidth float64 `json:"maxWidth"`
}

// noticeBackgroundAlpha keeps the notice box discreet over the picture.
const noticeBackgroundAlpha = 0xA0

// noticePadding is the room around the notice's text, in style units.
const noticePadding = 8

// DefaultNoticeLook is white app font text on a dark box, in the top right
// corner, where the eye goes first: in the left one notices went unseen.
func DefaultNoticeLook() NoticeLook {
	return NoticeLook{
		FontFamily: AppFont, FontSize: 22, Color: white, Background: black.WithAlpha(noticeBackgroundAlpha),
		Side: AlignRight, Margin: 0.03, MaxWidth: 0.6,
	}
}

// levelNoticeScale is how much larger a notice with a level is than the
// others: at 1.5 it still went unnoticed over a wide picture.
const levelNoticeScale = 2

// WithLevel is the look of a notice with a level under its text, such as the
// volume's: larger than the others, as it is read at a glance while the
// sound changes.
func (n NoticeLook) WithLevel() NoticeLook {
	n.FontSize *= levelNoticeScale
	return n
}

// Style is the notice as a subtitle style the layout and the renderer
// take: it follows the video size and wraps at two lines.
func (n NoticeLook) Style() SubtitleStyle {
	return SubtitleStyle{
		FontFamily:  n.FontFamily,
		FontSize:    n.FontSize,
		SizeMode:    SizeResponsive,
		Color:       n.Color,
		Opacity:     1,
		Background:  Background{Enabled: true, Color: n.Background, Padding: noticePadding},
		Alignment:   n.Side,
		MaxLines:    2,
		LineSpacing: 1,
	}
}

// Validate checks every field against its allowed range.
func (n NoticeLook) Validate() error {
	family := strings.TrimSpace(n.FontFamily)
	return firstFailure([]check{
		{family != "" && len([]rune(family)) <= MaxFontNameRunes, "notice.fontFamily"},
		{inRange(n.FontSize, MinFontSize, MaxNoticeFontSize), "notice.fontSize"},
		{n.Side == AlignLeft || n.Side == AlignCenter || n.Side == AlignRight, "notice.side"},
		{inRange(n.Margin, 0, MaxNoticeMargin), "notice.margin"},
		{inRange(n.MaxWidth, MinNoticeMaxWidth, 1), "notice.maxWidth"},
	}, ErrInvalidStyle)
}

// SelectionLook is the frame around the line picked for a correction: a
// thin line in Color with a dark edge just outside it, so it shows on bright
// and dark pictures alike. Sizes are in pixels at 100% display scaling.
type SelectionLook struct {
	Color Color `json:"color"`
	// Edge is the dark hairline just outside Color.
	Edge  Color   `json:"edge"`
	Width float64 `json:"width"`
	// Gap is the room between the text and the frame; Radius rounds its
	// corners.
	Gap    float64 `json:"gap"`
	Radius float64 `json:"radius"`
}

// DefaultSelectionLook is a sharp line in the app's blue with a half-dark
// edge outside: blue reads as "selected", sits apart from white or yellow
// text and from the karaoke mode's fill, and the edge carries it over bright
// pictures.
func DefaultSelectionLook() SelectionLook {
	return SelectionLook{Color: RGB(0x5B, 0x9B, 0xFF), Edge: black.WithAlpha(0x80), Width: 1, Gap: 4, Radius: 1.5}
}

// Validate checks every field against its allowed range.
func (s SelectionLook) Validate() error {
	return firstFailure([]check{
		{inRange(s.Width, MinSelectionWidth, MaxSelectionWidth), "selection.width"},
		{inRange(s.Gap, 0, MaxSelectionGap), "selection.gap"},
		{inRange(s.Radius, 0, MaxSelectionCurve), "selection.radius"},
	}, ErrInvalidStyle)
}

// CueStyle is how the karaoke mode shows the time left before a line: a bar
// above it, or the seconds in a smaller copy of its font.
type CueStyle struct {
	// BarThickness is in style units, like the outline.
	BarThickness float64 `json:"barThickness"`
	BarColor     Color   `json:"barColor"`
	// CountdownSize is the seconds' size as a share of the line's.
	CountdownSize float64 `json:"countdownSize"`
}

// DefaultCueStyle is a 5 unit bar in the given color and seconds at 80% of
// the line.
func DefaultCueStyle(bar Color) CueStyle {
	return CueStyle{BarThickness: 5, BarColor: bar, CountdownSize: 0.8}
}

// Validate checks every field against its allowed range.
func (c CueStyle) Validate() error {
	return firstFailure([]check{
		{inRange(c.BarThickness, MinCueThickness, MaxCueThickness), "cue.barThickness"},
		{inRange(c.CountdownSize, MinCountdownSize, MaxCountdownSize), "cue.countdownSize"},
	}, ErrInvalidStyle)
}
