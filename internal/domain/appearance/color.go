// Package appearance models how subtitles look: colors, fonts, outline,
// shadow, background, placement and the built-in presets.
package appearance

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidColor reports a color that is not "#RRGGBB" or "#RRGGBBAA".
var ErrInvalidColor = errors.New("invalid color")

// Color is an sRGB color with alpha. It is stored as "#RRGGBB" or "#RRGGBBAA".
type Color struct {
	R, G, B, A uint8
}

const opaque = 0xFF

// RGB returns an opaque color.
func RGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, A: opaque}
}

// WithAlpha returns the color with a new alpha.
func (c Color) WithAlpha(alpha uint8) Color {
	c.A = alpha
	return c
}

// String returns "#RRGGBB" for opaque colors and "#RRGGBBAA" otherwise.
func (c Color) String() string {
	if c.A == opaque {
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}

// MarshalText implements encoding.TextMarshaler.
func (c Color) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *Color) UnmarshalText(text []byte) error {
	parsed, err := ParseColor(string(text))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// ParseColor reads "#RRGGBB" or "#RRGGBBAA".
func ParseColor(value string) (Color, error) {
	hex, found := strings.CutPrefix(strings.TrimSpace(value), "#")
	if !found || (len(hex) != 6 && len(hex) != 8) {
		return Color{}, fmt.Errorf("%w: %q", ErrInvalidColor, value)
	}
	channels := make([]uint8, 0, 4)
	for i := 0; i < len(hex); i += 2 {
		channel, err := strconv.ParseUint(hex[i:i+2], 16, 8)
		if err != nil {
			return Color{}, fmt.Errorf("%w: %q", ErrInvalidColor, value)
		}
		channels = append(channels, uint8(channel))
	}
	color := Color{R: channels[0], G: channels[1], B: channels[2], A: opaque}
	if len(channels) == 4 {
		color.A = channels[3]
	}
	return color, nil
}
