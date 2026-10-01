package appearance

import (
	"errors"
	"fmt"
)

// ErrUnknownPreset reports a preset name that does not exist.
var ErrUnknownPreset = errors.New("unknown subtitle preset")

// Preset names a built-in look. Presets change colors and effects only; font,
// size and placement stay as the user set them.
type Preset string

const (
	PresetClassic      Preset = "classic"
	PresetFilm         Preset = "film"
	PresetHighContrast Preset = "high-contrast"
	PresetMinimal      Preset = "minimal"
)

// Presets lists the built-in presets in display order.
func Presets() []Preset {
	return []Preset{PresetClassic, PresetFilm, PresetHighContrast, PresetMinimal}
}

var (
	white          = RGB(0xFF, 0xFF, 0xFF)
	black          = RGB(0x00, 0x00, 0x00)
	warmWhite      = RGB(0xF5, 0xF0, 0xE1)
	contrastYellow = RGB(0xFF, 0xE6, 0x00)
)

// Alpha values used by the presets.
const (
	softShadowAlpha = 0xB0
	backgroundAlpha = 0xC8
)

// Apply returns style with the preset's colors and effects.
func (p Preset) Apply(style SubtitleStyle) (SubtitleStyle, error) {
	switch p {
	case PresetClassic:
		style.Color, style.Opacity = white, 1
		style.Outline = Outline{Enabled: true, Color: black, Width: 3}
		style.Shadow = Shadow{Enabled: true, Color: black, OffsetX: 2, OffsetY: 2}
		style.Background = Background{}
	case PresetFilm:
		style.Color, style.Opacity = warmWhite, 1
		style.Outline = Outline{Enabled: true, Color: black, Width: 1.5}
		style.Shadow = Shadow{Enabled: true, Color: black.WithAlpha(softShadowAlpha), OffsetX: 1, OffsetY: 2, Blur: 4}
		style.Background = Background{}
	case PresetHighContrast:
		style.Color, style.Opacity = contrastYellow, 1
		style.Outline = Outline{Enabled: true, Color: black, Width: 2}
		style.Shadow = Shadow{}
		style.Background = Background{Enabled: true, Color: black.WithAlpha(backgroundAlpha), Padding: 8}
	case PresetMinimal:
		style.Color, style.Opacity = white, 1
		style.Outline = Outline{Enabled: true, Color: black, Width: 1}
		style.Shadow = Shadow{}
		style.Background = Background{}
	default:
		return style, fmt.Errorf("%w: %q", ErrUnknownPreset, p)
	}
	return style, nil
}

// DefaultPrimaryStyle is the look of the studied language (PRD: Arial 32,
// white, 3 px black outline, 2 px shadow, at 82% of the height).
func DefaultPrimaryStyle() SubtitleStyle {
	style, _ := PresetClassic.Apply(SubtitleStyle{
		FontFamily:       "Arial",
		FontSize:         32,
		SizeMode:         SizeResponsive,
		VerticalPosition: 0.82,
		Alignment:        AlignCenter,
		MaxLines:         2,
	})
	return style
}

// translationOpacity keeps the translation a step behind the primary line.
const translationOpacity = 0.9

// DefaultTranslationStyle is the look of the translation: same preset,
// smaller, lower and slightly see-through, so it never competes with the
// primary line (DESIGN §10).
func DefaultTranslationStyle() SubtitleStyle {
	style := DefaultPrimaryStyle()
	style.FontSize = 28
	style.VerticalPosition = 0.91
	style.Opacity = translationOpacity
	return style
}
