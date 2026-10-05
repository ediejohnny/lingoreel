package appearance_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
)

func TestDefaultStylesAreValidAndOrdered(t *testing.T) {
	t.Parallel()
	primary, translation := appearance.DefaultPrimaryStyle(), appearance.DefaultTranslationStyle()
	for name, style := range map[string]appearance.SubtitleStyle{"primary": primary, "translation": translation} {
		if err := style.Validate(); err != nil {
			t.Fatalf("default %s style invalid: %v", name, err)
		}
	}
	// The translation sits below the primary line and must not compete with the primary line.
	if translation.VerticalPosition <= primary.VerticalPosition {
		t.Fatalf("positions = %v/%v, want the translation below", primary.VerticalPosition, translation.VerticalPosition)
	}
	if primary.Opacity != 1 || !(translation.Opacity < primary.Opacity && translation.Opacity >= 0.8) {
		t.Fatalf("opacities = %v/%v, want the translation slightly behind (0.8 to 1)", primary.Opacity, translation.Opacity)
	}
}

func TestStyleValidateRanges(t *testing.T) {
	t.Parallel()
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		name   string
		mutate func(*appearance.SubtitleStyle)
		valid  bool
	}{
		{"font size at minimum", func(s *appearance.SubtitleStyle) { s.FontSize = appearance.MinFontSize }, true},
		{"font size below minimum", func(s *appearance.SubtitleStyle) { s.FontSize = appearance.MinFontSize - 0.01 }, false},
		{"font size at maximum", func(s *appearance.SubtitleStyle) { s.FontSize = appearance.MaxFontSize }, true},
		{"font size above maximum", func(s *appearance.SubtitleStyle) { s.FontSize = appearance.MaxFontSize + 0.01 }, false},
		{"font size zero", func(s *appearance.SubtitleStyle) { s.FontSize = 0 }, false},
		{"font size NaN", func(s *appearance.SubtitleStyle) { s.FontSize = nan }, false},
		{"font size infinite", func(s *appearance.SubtitleStyle) { s.FontSize = inf }, false},
		{"blank font family", func(s *appearance.SubtitleStyle) { s.FontFamily = "   " }, false},
		{"very long font family", func(s *appearance.SubtitleStyle) { s.FontFamily = strings.Repeat("é", appearance.MaxFontNameRunes+1) }, false},
		{"font family at the limit", func(s *appearance.SubtitleStyle) { s.FontFamily = strings.Repeat("é", appearance.MaxFontNameRunes) }, true},
		{"unknown size mode", func(s *appearance.SubtitleStyle) { s.SizeMode = "auto" }, false},
		{"fixed size mode", func(s *appearance.SubtitleStyle) { s.SizeMode = appearance.SizeFixed }, true},
		{"opacity zero", func(s *appearance.SubtitleStyle) { s.Opacity = 0 }, true},
		{"opacity one", func(s *appearance.SubtitleStyle) { s.Opacity = 1 }, true},
		{"opacity negative", func(s *appearance.SubtitleStyle) { s.Opacity = -0.001 }, false},
		{"opacity above one", func(s *appearance.SubtitleStyle) { s.Opacity = 1.001 }, false},
		{"opacity NaN", func(s *appearance.SubtitleStyle) { s.Opacity = nan }, false},
		{"outline width zero", func(s *appearance.SubtitleStyle) { s.Outline.Width = 0 }, true},
		{"outline width negative", func(s *appearance.SubtitleStyle) { s.Outline.Width = -1 }, false},
		{"outline width above maximum", func(s *appearance.SubtitleStyle) { s.Outline.Width = appearance.MaxOutlineWidth + 0.1 }, false},
		{"shadow offset at negative limit", func(s *appearance.SubtitleStyle) { s.Shadow.OffsetX = -appearance.MaxShadowOffset }, true},
		{"shadow offset beyond negative limit", func(s *appearance.SubtitleStyle) { s.Shadow.OffsetY = -appearance.MaxShadowOffset - 0.1 }, false},
		{"shadow offset NaN", func(s *appearance.SubtitleStyle) { s.Shadow.OffsetX = nan }, false},
		{"shadow blur negative", func(s *appearance.SubtitleStyle) { s.Shadow.Blur = -0.5 }, false},
		{"shadow blur above maximum", func(s *appearance.SubtitleStyle) { s.Shadow.Blur = appearance.MaxShadowBlur + 0.5 }, false},
		{"background padding above maximum", func(s *appearance.SubtitleStyle) { s.Background.Padding = appearance.MaxPadding + 1 }, false},
		{"vertical position zero", func(s *appearance.SubtitleStyle) { s.VerticalPosition = 0 }, false},
		{"vertical position at the bottom", func(s *appearance.SubtitleStyle) { s.VerticalPosition = 1 }, true},
		{"vertical position below the video", func(s *appearance.SubtitleStyle) { s.VerticalPosition = 1.01 }, false},
		{"vertical position NaN", func(s *appearance.SubtitleStyle) { s.VerticalPosition = nan }, false},
		{"empty alignment", func(s *appearance.SubtitleStyle) { s.Alignment = "" }, false},
		{"right alignment", func(s *appearance.SubtitleStyle) { s.Alignment = appearance.AlignRight }, true},
		{"zero lines", func(s *appearance.SubtitleStyle) { s.MaxLines = 0 }, false},
		{"one line", func(s *appearance.SubtitleStyle) { s.MaxLines = appearance.MinLines }, true},
		{"maximum lines", func(s *appearance.SubtitleStyle) { s.MaxLines = appearance.MaxLines }, true},
		{"too many lines", func(s *appearance.SubtitleStyle) { s.MaxLines = appearance.MaxLines + 1 }, false},
		{"tightest line spacing", func(s *appearance.SubtitleStyle) { s.LineSpacing = appearance.MinLineSpacing }, true},
		{"widest line spacing", func(s *appearance.SubtitleStyle) { s.LineSpacing = appearance.MaxLineSpacing }, true},
		{"no line spacing (an older profile)", func(s *appearance.SubtitleStyle) { s.LineSpacing = 0 }, false},
		{"line spacing NaN", func(s *appearance.SubtitleStyle) { s.LineSpacing = nan }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			style := appearance.DefaultPrimaryStyle()
			tc.mutate(&style)
			err := style.Validate()
			if tc.valid && err != nil {
				t.Fatalf("Validate() = %v, want valid", err)
			}
			if !tc.valid && !errors.Is(err, appearance.ErrInvalidStyle) {
				t.Fatalf("Validate() = %v, want ErrInvalidStyle", err)
			}
		})
	}
}
