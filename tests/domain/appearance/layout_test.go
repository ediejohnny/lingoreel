package appearance_test

import (
	"errors"
	"math"
	"testing"

	"lingoreel/internal/domain/appearance"
)

func TestLayoutValidateRanges(t *testing.T) {
	nan := math.NaN()
	cases := []struct {
		name   string
		mutate func(*appearance.Layout)
		valid  bool
	}{
		{"default", func(*appearance.Layout) {}, true},
		{"no margins", func(l *appearance.Layout) { l.MarginLeft, l.MarginRight, l.MarginBottom = 0, 0, 0 }, true},
		{"margin at the limit", func(l *appearance.Layout) { l.MarginLeft = appearance.MaxMarginFraction }, true},
		{"margin beyond the limit", func(l *appearance.Layout) { l.MarginRight = appearance.MaxMarginFraction + 0.0001 }, false},
		{"negative margin", func(l *appearance.Layout) { l.MarginBottom = -0.01 }, false},
		{"margin NaN", func(l *appearance.Layout) { l.MarginLeft = nan }, false},
		{"full width", func(l *appearance.Layout) { l.MaxWidth = 1 }, true},
		{"zero width", func(l *appearance.Layout) { l.MaxWidth = 0 }, false},
		{"width above full", func(l *appearance.Layout) { l.MaxWidth = 1.01 }, false},
		{"width NaN", func(l *appearance.Layout) { l.MaxWidth = nan }, false},
		{"width infinite", func(l *appearance.Layout) { l.MaxWidth = math.Inf(1) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layout := appearance.DefaultLayout()
			tc.mutate(&layout)
			err := layout.Validate()
			if tc.valid && err != nil {
				t.Fatalf("Validate() = %v, want valid", err)
			}
			if !tc.valid && !errors.Is(err, appearance.ErrInvalidLayout) {
				t.Fatalf("Validate() = %v, want ErrInvalidLayout", err)
			}
		})
	}
}
