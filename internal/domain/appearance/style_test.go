package appearance

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestDefaultStylesAreValidAndOrdered(t *testing.T) {
	primary, translation := DefaultPrimaryStyle(), DefaultTranslationStyle()
	for name, style := range map[string]SubtitleStyle{"primary": primary, "translation": translation} {
		if err := style.Validate(); err != nil {
			t.Fatalf("default %s style invalid: %v", name, err)
		}
	}
	if primary.FontSize != 32 || translation.FontSize != 28 {
		t.Fatalf("font sizes = %v/%v, want 32/28", primary.FontSize, translation.FontSize)
	}
	if primary.VerticalPosition != 0.82 || translation.VerticalPosition != 0.91 {
		t.Fatalf("positions = %v/%v, want 0.82/0.91", primary.VerticalPosition, translation.VerticalPosition)
	}
}

func TestStyleValidateRanges(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		name   string
		mutate func(*SubtitleStyle)
		valid  bool
	}{
		{"font size at minimum", func(s *SubtitleStyle) { s.FontSize = MinFontSize }, true},
		{"font size below minimum", func(s *SubtitleStyle) { s.FontSize = MinFontSize - 0.01 }, false},
		{"font size at maximum", func(s *SubtitleStyle) { s.FontSize = MaxFontSize }, true},
		{"font size above maximum", func(s *SubtitleStyle) { s.FontSize = MaxFontSize + 0.01 }, false},
		{"font size zero", func(s *SubtitleStyle) { s.FontSize = 0 }, false},
		{"font size NaN", func(s *SubtitleStyle) { s.FontSize = nan }, false},
		{"font size infinite", func(s *SubtitleStyle) { s.FontSize = inf }, false},
		{"blank font family", func(s *SubtitleStyle) { s.FontFamily = "   " }, false},
		{"very long font family", func(s *SubtitleStyle) { s.FontFamily = strings.Repeat("é", maxFontNameRunes+1) }, false},
		{"font family at the limit", func(s *SubtitleStyle) { s.FontFamily = strings.Repeat("é", maxFontNameRunes) }, true},
		{"unknown size mode", func(s *SubtitleStyle) { s.SizeMode = "auto" }, false},
		{"opacity zero", func(s *SubtitleStyle) { s.Opacity = 0 }, true},
		{"opacity one", func(s *SubtitleStyle) { s.Opacity = 1 }, true},
		{"opacity negative", func(s *SubtitleStyle) { s.Opacity = -0.001 }, false},
		{"opacity above one", func(s *SubtitleStyle) { s.Opacity = 1.001 }, false},
		{"opacity NaN", func(s *SubtitleStyle) { s.Opacity = nan }, false},
		{"outline width zero", func(s *SubtitleStyle) { s.Outline.Width = 0 }, true},
		{"outline width negative", func(s *SubtitleStyle) { s.Outline.Width = -1 }, false},
		{"outline width above maximum", func(s *SubtitleStyle) { s.Outline.Width = MaxOutlineWidth + 0.1 }, false},
		{"shadow offset at negative limit", func(s *SubtitleStyle) { s.Shadow.OffsetX = -MaxShadowOffset }, true},
		{"shadow offset beyond negative limit", func(s *SubtitleStyle) { s.Shadow.OffsetY = -MaxShadowOffset - 0.1 }, false},
		{"shadow offset NaN", func(s *SubtitleStyle) { s.Shadow.OffsetX = nan }, false},
		{"shadow blur negative", func(s *SubtitleStyle) { s.Shadow.Blur = -0.5 }, false},
		{"background padding above maximum", func(s *SubtitleStyle) { s.Background.Padding = MaxPadding + 1 }, false},
		{"vertical position zero", func(s *SubtitleStyle) { s.VerticalPosition = 0 }, false},
		{"vertical position at the bottom", func(s *SubtitleStyle) { s.VerticalPosition = 1 }, true},
		{"vertical position below the video", func(s *SubtitleStyle) { s.VerticalPosition = 1.01 }, false},
		{"vertical position NaN", func(s *SubtitleStyle) { s.VerticalPosition = nan }, false},
		{"empty alignment", func(s *SubtitleStyle) { s.Alignment = "" }, false},
		{"right alignment", func(s *SubtitleStyle) { s.Alignment = AlignRight }, true},
		{"zero lines", func(s *SubtitleStyle) { s.MaxLines = 0 }, false},
		{"one line", func(s *SubtitleStyle) { s.MaxLines = MinLines }, true},
		{"maximum lines", func(s *SubtitleStyle) { s.MaxLines = MaxLines }, true},
		{"too many lines", func(s *SubtitleStyle) { s.MaxLines = MaxLines + 1 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := DefaultPrimaryStyle()
			tc.mutate(&style)
			err := style.Validate()
			if tc.valid && err != nil {
				t.Fatalf("Validate() = %v, want valid", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidStyle) {
				t.Fatalf("Validate() = %v, want ErrInvalidStyle", err)
			}
		})
	}
}
