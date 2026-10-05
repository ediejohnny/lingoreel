package appearance

import (
	"errors"
	"strings"
)

// ErrInvalidStyle reports a subtitle style outside its allowed ranges.
var ErrInvalidStyle = errors.New("invalid subtitle style")

// FieldError names the field of a style or layout that is out of range; it
// matches ErrInvalidStyle or ErrInvalidLayout with errors.Is.
type FieldError struct {
	Field string
	kind  error
}

func (e *FieldError) Error() string { return e.kind.Error() + ": " + e.Field }

func (e *FieldError) Unwrap() error { return e.kind }

// Alignment is the horizontal alignment of a subtitle block.
type Alignment string

const (
	AlignLeft   Alignment = "left"
	AlignCenter Alignment = "center"
	AlignRight  Alignment = "right"
)

// SizeMode decides whether the font size follows the video size.
type SizeMode string

const (
	// SizeFixed keeps FontSize in pixels at 100% display scaling, regardless of
	// the video size; it grows with the display DPI.
	SizeFixed SizeMode = "fixed"
	// SizeResponsive scales FontSize from ReferenceVideoHeight to the actual video height.
	SizeResponsive SizeMode = "responsive"
)

// ReferenceVideoHeight is the height at which responsive sizes are specified.
const ReferenceVideoHeight = 1080

// Ranges accepted for a style.
const (
	MinFontSize      = 8.0
	MaxFontSize      = 200.0
	MaxOutlineWidth  = 20.0
	MaxShadowOffset  = 20.0
	MaxShadowBlur    = 20.0
	MaxPadding       = 40.0
	MinLines         = 1
	MaxLines         = 5
	MaxFontNameRunes = 100
	// The line spacing multiplies the font's own distance between lines.
	MinLineSpacing = 0.8
	MaxLineSpacing = 2.0
)

// Outline draws a stroke around each glyph.
type Outline struct {
	Enabled bool    `json:"enabled"`
	Color   Color   `json:"color"`
	Width   float64 `json:"width"`
}

// Shadow draws an offset, optionally blurred copy of the text behind it.
type Shadow struct {
	Enabled bool    `json:"enabled"`
	Color   Color   `json:"color"`
	OffsetX float64 `json:"offsetX"`
	OffsetY float64 `json:"offsetY"`
	Blur    float64 `json:"blur"`
}

// Background draws a box behind the text block.
type Background struct {
	Enabled bool    `json:"enabled"`
	Color   Color   `json:"color"`
	Padding float64 `json:"padding"`
}

// SubtitleStyle is the full appearance of one subtitle line (primary or
// translation). Sizes are in pixels at ReferenceVideoHeight.
type SubtitleStyle struct {
	FontFamily string     `json:"fontFamily"`
	FontSize   float64    `json:"fontSize"`
	SizeMode   SizeMode   `json:"sizeMode"`
	Bold       bool       `json:"bold"`
	Italic     bool       `json:"italic"`
	Color      Color      `json:"color"`
	Opacity    float64    `json:"opacity"`
	Outline    Outline    `json:"outline"`
	Shadow     Shadow     `json:"shadow"`
	Background Background `json:"background"`
	// VerticalPosition is where the bottom edge of the block sits, as a
	// fraction of the video height measured from the top.
	VerticalPosition float64   `json:"verticalPosition"`
	Alignment        Alignment `json:"alignment"`
	MaxLines         int       `json:"maxLines"`
	// LineSpacing spreads the lines of a block: 1 keeps the font's own
	// spacing, and the extra room is split above and below each line.
	LineSpacing float64 `json:"lineSpacing"`
}

// Validate checks every field against its allowed range.
func (s SubtitleStyle) Validate() error {
	family := strings.TrimSpace(s.FontFamily)
	checks := []check{
		{family != "" && len([]rune(family)) <= MaxFontNameRunes, "fontFamily"},
		{s.FontSize >= MinFontSize && s.FontSize <= MaxFontSize, "fontSize"},
		{s.SizeMode == SizeFixed || s.SizeMode == SizeResponsive, "sizeMode"},
		{s.Opacity >= 0 && s.Opacity <= 1, "opacity"},
		{s.Outline.Width >= 0 && s.Outline.Width <= MaxOutlineWidth, "outline.width"},
		{inRange(s.Shadow.OffsetX, -MaxShadowOffset, MaxShadowOffset), "shadow.offsetX"},
		{inRange(s.Shadow.OffsetY, -MaxShadowOffset, MaxShadowOffset), "shadow.offsetY"},
		{s.Shadow.Blur >= 0 && s.Shadow.Blur <= MaxShadowBlur, "shadow.blur"},
		{s.Background.Padding >= 0 && s.Background.Padding <= MaxPadding, "background.padding"},
		{s.VerticalPosition > 0 && s.VerticalPosition <= 1, "verticalPosition"},
		{s.Alignment == AlignLeft || s.Alignment == AlignCenter || s.Alignment == AlignRight, "alignment"},
		{s.MaxLines >= MinLines && s.MaxLines <= MaxLines, "maxLines"},
		{inRange(s.LineSpacing, MinLineSpacing, MaxLineSpacing), "lineSpacing"},
	}
	return firstFailure(checks, ErrInvalidStyle)
}

// check is one row of a validation table: the field it names fails when ok
// is false. Every row is written as "inside the range", so NaN fails it.
type check struct {
	ok    bool
	field string
}

// firstFailure returns the first failing row as a FieldError of kind.
func firstFailure(checks []check, kind error) error {
	for _, c := range checks {
		if !c.ok {
			return &FieldError{Field: c.field, kind: kind}
		}
	}
	return nil
}

func inRange(value, low, high float64) bool {
	return value >= low && value <= high
}
