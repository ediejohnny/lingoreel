package appearance

import "strings"

// Ranges accepted for the controls over the video.
const (
	MinBarSize      = 0.75
	MaxBarSize      = 2.0
	MinEditHintSize = 9.0
	MaxEditHintSize = 24.0
)

// BarStyle is how the control bar over the video and its menus look. The
// bar always sits over a picture, so its defaults are the dark theme's.
type BarStyle struct {
	Panel  Color `json:"panel"`
	Menu   Color `json:"menu"`
	Accent Color `json:"accent"`
	// Icon is the icons' and the texts' color.
	Icon Color `json:"icon"`
	// Loud is the volume past 100%, where the player amplifies.
	Loud       Color  `json:"loud"`
	FontFamily string `json:"fontFamily"`
	// Size multiplies the bar's size at the display's scaling, menus
	// included.
	Size float64 `json:"size"`
}

// DefaultBarStyle is the dark theme's panel and menus, its blue accent and
// the app's font, at the display's size.
func DefaultBarStyle() BarStyle {
	return BarStyle{
		Panel: RGB(0x0E, 0x12, 0x17).WithAlpha(0xB8), Menu: RGB(0x1B, 0x22, 0x2B).WithAlpha(0xF0),
		Accent: RGB(0x5B, 0x9B, 0xFF), Icon: RGB(0xE7, 0xEA, 0xEF), Loud: RGB(0xF5, 0xA5, 0x24),
		FontFamily: AppFont, Size: 1,
	}
}

// Validate checks the font and the size.
func (b BarStyle) Validate() error {
	family := strings.TrimSpace(b.FontFamily)
	return firstFailure([]check{
		{family != "" && len([]rune(family)) <= MaxFontNameRunes, "bar.fontFamily"},
		{inRange(b.Size, MinBarSize, MaxBarSize), "bar.size"},
	}, ErrInvalidStyle)
}

// EditStyle is how the box for correcting a line in the video looks; its
// text takes the line's own font.
type EditStyle struct {
	Background Color `json:"background"`
	Text       Color `json:"text"`
	Hint       Color `json:"hint"`
	// HintSize is the hint's font size, in pixels at 100% scaling.
	HintSize float64 `json:"hintSize"`
}

// DefaultEditStyle is white text on a dark box, with a grey hint.
func DefaultEditStyle() EditStyle {
	return EditStyle{Background: RGB(0x1C, 0x1C, 0x1E), Text: RGB(0xFF, 0xFF, 0xFF), Hint: RGB(0xA8, 0xA8, 0xB0), HintSize: 13}
}

// Validate checks the hint's size.
func (e EditStyle) Validate() error {
	return firstFailure([]check{{inRange(e.HintSize, MinEditHintSize, MaxEditHintSize), "edit.hintSize"}}, ErrInvalidStyle)
}
