package appearance

import (
	"errors"
	"fmt"
)

// ErrInvalidLayout reports layout margins outside their allowed ranges.
var ErrInvalidLayout = errors.New("invalid subtitle layout")

// maxMarginFraction keeps at least half of the video usable for text.
const maxMarginFraction = 0.25

// Layout holds the safe area shared by both subtitle lines. Every value is a
// fraction of the video size, so the layout survives resizing and DPI changes.
type Layout struct {
	MarginLeft   float64 `json:"marginLeft"`
	MarginRight  float64 `json:"marginRight"`
	MarginBottom float64 `json:"marginBottom"`
	// MaxWidth caps the text width as a fraction of the video width.
	MaxWidth float64 `json:"maxWidth"`
}

// DefaultLayout returns the layout used before the user changes anything.
func DefaultLayout() Layout {
	return Layout{MarginLeft: 0.05, MarginRight: 0.05, MarginBottom: 0.04, MaxWidth: 0.9}
}

// Validate checks the margins and the maximum width.
func (l Layout) Validate() error {
	switch {
	case !inRange(l.MarginLeft, 0, maxMarginFraction):
		return fmt.Errorf("%w: marginLeft", ErrInvalidLayout)
	case !inRange(l.MarginRight, 0, maxMarginFraction):
		return fmt.Errorf("%w: marginRight", ErrInvalidLayout)
	case !inRange(l.MarginBottom, 0, maxMarginFraction):
		return fmt.Errorf("%w: marginBottom", ErrInvalidLayout)
	// Written as "not inside" so NaN, which fails every comparison, is rejected.
	case !(l.MaxWidth > 0 && l.MaxWidth <= 1):
		return fmt.Errorf("%w: maxWidth", ErrInvalidLayout)
	}
	return nil
}
