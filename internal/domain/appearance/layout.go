package appearance

import (
	"errors"
)

// ErrInvalidLayout reports layout margins outside their allowed ranges.
var ErrInvalidLayout = errors.New("invalid subtitle layout")

// MaxMarginFraction keeps at least half of the video usable for text.
const MaxMarginFraction = 0.25

// MaxLineGap is the widest gap between the two subtitles, in lines.
const MaxLineGap = 2.0

// Layout holds the safe area shared by both subtitle lines. Every value is a
// fraction of the video size, so the layout survives resizing and DPI changes.
type Layout struct {
	MarginLeft  float64 `json:"marginLeft"`
	MarginRight float64 `json:"marginRight"`
	// MarginTop keeps subtitles placed near the top off the picture's edge.
	MarginTop    float64 `json:"marginTop"`
	MarginBottom float64 `json:"marginBottom"`
	// MaxWidth caps the text width as a fraction of the video width.
	MaxWidth float64 `json:"maxWidth"`
	// LineGap is the least room kept between the two subtitles when they
	// come close, in line heights of the one that gives way (each still sits
	// at its own height otherwise).
	LineGap float64 `json:"lineGap"`
}

// DefaultLayout returns the layout used before the user changes anything.
func DefaultLayout() Layout {
	return Layout{MarginLeft: 0.05, MarginRight: 0.05, MarginTop: 0.05, MarginBottom: 0.04, MaxWidth: 0.9, LineGap: 0.2}
}

// Validate checks the margins and the maximum width.
func (l Layout) Validate() error {
	field := ""
	switch {
	case !inRange(l.MarginLeft, 0, MaxMarginFraction):
		field = "marginLeft"
	case !inRange(l.MarginRight, 0, MaxMarginFraction):
		field = "marginRight"
	case !inRange(l.MarginTop, 0, MaxMarginFraction):
		field = "marginTop"
	case !inRange(l.MarginBottom, 0, MaxMarginFraction):
		field = "marginBottom"
	// Written as "not inside" so NaN, which fails every comparison, is rejected.
	case !(l.MaxWidth > 0 && l.MaxWidth <= 1):
		field = "maxWidth"
	case !inRange(l.LineGap, 0, MaxLineGap):
		field = "lineGap"
	default:
		return nil
	}
	return &FieldError{Field: field, kind: ErrInvalidLayout}
}
