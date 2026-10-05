package appearance_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
)

func TestMarksValidateRanges(t *testing.T) {
	t.Parallel()
	nan := math.NaN()
	cases := []struct {
		name  string
		check func() error
		field string // empty when valid
	}{
		{"default notice", func() error { return appearance.DefaultNoticeLook().Validate() }, ""},
		{"notice at its largest", notice(func(n *appearance.NoticeLook) { n.FontSize = appearance.MaxNoticeFontSize }), ""},
		{"notice too large", notice(func(n *appearance.NoticeLook) { n.FontSize = appearance.MaxNoticeFontSize + 1 }), "notice.fontSize"},
		{"notice size NaN", notice(func(n *appearance.NoticeLook) { n.FontSize = nan }), "notice.fontSize"},
		{"notice without a font", notice(func(n *appearance.NoticeLook) { n.FontFamily = " " }), "notice.fontFamily"},
		{"notice on the right", notice(func(n *appearance.NoticeLook) { n.Side = appearance.AlignRight }), ""},
		{"notice on no side", notice(func(n *appearance.NoticeLook) { n.Side = "top" }), "notice.side"},
		{"notice margin NaN", notice(func(n *appearance.NoticeLook) { n.Margin = nan }), "notice.margin"},
		{"notice margin too wide", notice(func(n *appearance.NoticeLook) { n.Margin = appearance.MaxNoticeMargin + 0.01 }), "notice.margin"},
		{"notice as wide as the video", notice(func(n *appearance.NoticeLook) { n.MaxWidth = 1 }), ""},
		{"notice too narrow", notice(func(n *appearance.NoticeLook) { n.MaxWidth = appearance.MinNoticeMaxWidth - 0.01 }), "notice.maxWidth"},
		{"default selection", func() error { return appearance.DefaultSelectionLook().Validate() }, ""},
		{"selection too thin", selection(func(s *appearance.SelectionLook) { s.Width = appearance.MinSelectionWidth - 0.1 }), "selection.width"},
		{"selection width NaN", selection(func(s *appearance.SelectionLook) { s.Width = nan }), "selection.width"},
		{"selection without a gap", selection(func(s *appearance.SelectionLook) { s.Gap = 0 }), ""},
		{"selection gap negative", selection(func(s *appearance.SelectionLook) { s.Gap = -1 }), "selection.gap"},
		{"selection corners too round", selection(func(s *appearance.SelectionLook) { s.Radius = appearance.MaxSelectionCurve + 1 }), "selection.radius"},
		{"default time left", func() error { return appearance.DefaultCueStyle(appearance.RGB(1, 2, 3)).Validate() }, ""},
		{"bar too thick", cue(func(c *appearance.CueStyle) { c.BarThickness = appearance.MaxCueThickness + 1 }), "cue.barThickness"},
		{"no bar (an older profile)", cue(func(c *appearance.CueStyle) { c.BarThickness = 0 }), "cue.barThickness"},
		{"countdown at its smallest", cue(func(c *appearance.CueStyle) { c.CountdownSize = appearance.MinCountdownSize }), ""},
		{"countdown size NaN", cue(func(c *appearance.CueStyle) { c.CountdownSize = nan }), "cue.countdownSize"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.check()
			if tc.field == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want valid", err)
				}
				return
			}
			var field *appearance.FieldError
			if !errors.Is(err, appearance.ErrInvalidStyle) || !errors.As(err, &field) || field.Field != tc.field {
				t.Fatalf("Validate() = %v, want %s refused", err, tc.field)
			}
		})
	}
}

func notice(change func(*appearance.NoticeLook)) func() error {
	return func() error {
		look := appearance.DefaultNoticeLook()
		change(&look)
		return look.Validate()
	}
}

func selection(change func(*appearance.SelectionLook)) func() error {
	return func() error {
		look := appearance.DefaultSelectionLook()
		change(&look)
		return look.Validate()
	}
}

func cue(change func(*appearance.CueStyle)) func() error {
	return func() error {
		style := appearance.DefaultCueStyle(appearance.RGB(1, 2, 3))
		change(&style)
		return style.Validate()
	}
}
