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
		{"default bar", func() error { return appearance.DefaultBarStyle().Validate() }, ""},
		{"bar at its largest", bar(func(b *appearance.BarStyle) { b.Size = appearance.MaxBarSize }), ""},
		{"bar too small", bar(func(b *appearance.BarStyle) { b.Size = appearance.MinBarSize - 0.01 }), "bar.size"},
		{"bar size NaN", bar(func(b *appearance.BarStyle) { b.Size = nan }), "bar.size"},
		{"bar without a font", bar(func(b *appearance.BarStyle) { b.FontFamily = "" }), "bar.fontFamily"},
		{"default box", func() error { return appearance.DefaultEditStyle().Validate() }, ""},
		{"hint too large", edit(func(e *appearance.EditStyle) { e.HintSize = appearance.MaxEditHintSize + 1 }), "edit.hintSize"},
		{"hint size NaN", edit(func(e *appearance.EditStyle) { e.HintSize = nan }), "edit.hintSize"},
		{"default app", func() error { return appearance.DefaultInterfaceStyle().Validate() }, ""},
		{"app accent chosen", app(func(i *appearance.InterfaceStyle) { i.Accent = appearance.RGB(0xFF, 0x8C, 0x00) }), ""},
		{"app accent see-through", app(func(i *appearance.InterfaceStyle) { i.Accent = appearance.RGB(0xFF, 0x8C, 0x00).WithAlpha(0x80) }), "interface.accent"},
		{"app font chosen", app(func(i *appearance.InterfaceStyle) { i.FontFamily = "Verdana" }), ""},
		{"app font with spaces around", app(func(i *appearance.InterfaceStyle) { i.FontFamily = " Verdana" }), "interface.fontFamily"},
		{"app text at its largest", app(func(i *appearance.InterfaceStyle) { i.TextSize = appearance.MaxTextSize }), ""},
		{"app text too small", app(func(i *appearance.InterfaceStyle) { i.TextSize = appearance.MinTextSize - 0.01 }), "interface.textSize"},
		{"app text size NaN", app(func(i *appearance.InterfaceStyle) { i.TextSize = nan }), "interface.textSize"},
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

func bar(change func(*appearance.BarStyle)) func() error {
	return func() error {
		style := appearance.DefaultBarStyle()
		change(&style)
		return style.Validate()
	}
}

func edit(change func(*appearance.EditStyle)) func() error {
	return func() error {
		style := appearance.DefaultEditStyle()
		change(&style)
		return style.Validate()
	}
}

func app(change func(*appearance.InterfaceStyle)) func() error {
	return func() error {
		look := appearance.DefaultInterfaceStyle()
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
