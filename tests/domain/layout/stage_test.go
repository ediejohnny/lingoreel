package layout_test

import (
	"math"
	"testing"

	"lingoreel/internal/domain/layout"
)

func TestVideoAreaLeavesTheControlsTheirRoom(t *testing.T) {
	client := layout.Rect{W: 1008, H: 681}
	// The app's header (56 px) on top and its panel (380 px) on the right.
	panel := layout.Insets{Top: 56, Right: 380}
	cases := []struct {
		name       string
		insets     layout.Insets
		dpi        int
		fullscreen bool
		want       layout.Rect
	}{
		{"painel aberto a 100%", panel, 96, false, layout.Rect{X: 0, Y: 56, W: 628, H: 625}},
		// At 150% the controls take 1.5 times as many pixels: 84 and 570.
		{"painel aberto a 150%", panel, 144, false, layout.Rect{X: 0, Y: 84, W: 438, H: 597}},
		{"painel fechado", layout.Insets{Top: 56}, 96, false, layout.Rect{X: 0, Y: 56, W: 1008, H: 625}},
		{"os quatro lados", layout.Insets{Top: 10, Right: 20, Bottom: 30, Left: 40}, 96, false, layout.Rect{X: 40, Y: 10, W: 948, H: 641}},
		// Fullscreen: the whole client area, whatever the insets say.
		{"tela cheia", panel, 144, true, layout.Rect{W: 1008, H: 681}},
		// Fractions of a pixel round to whole pixels, inside the client area.
		{"frações", layout.Insets{Top: 56.4, Right: 379.6}, 120, false, layout.Rect{X: 0, Y: 71, W: 533, H: 610}},
		// A scaling that is no scaling reads as 100%.
		{"sem escala", panel, 0, false, layout.Rect{X: 0, Y: 56, W: 628, H: 625}},
		{"escala negativa", panel, -96, false, layout.Rect{X: 0, Y: 56, W: 628, H: 625}},
	}
	for _, c := range cases {
		if got := layout.VideoArea(client, c.insets, c.dpi, c.fullscreen); got != c.want {
			t.Errorf("%s: VideoArea = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestVideoAreaIsNeverNegative(t *testing.T) {
	client := layout.Rect{W: 640, H: 360}
	for _, insets := range []layout.Insets{
		{Right: 640},                    // the panel takes the whole width
		{Top: 200, Bottom: 160},         // nothing left in height
		{Right: 639.6},                  // less than a pixel left, rounded away
		{Left: 1e9},                     // hostile
		{Top: 360, Right: 1, Left: 1e6}, // all at once
	} {
		if got := layout.VideoArea(client, insets, 96, false); got != (layout.Rect{}) {
			t.Errorf("VideoArea(%+v) = %+v, want nothing", insets, got)
		}
	}
	// No client area, or one that is no area, leaves nothing either.
	for _, broken := range []layout.Rect{{}, {W: -5, H: 10}, {W: math.NaN(), H: 10}, {W: math.Inf(1), H: 10}} {
		if got := layout.VideoArea(broken, layout.Insets{}, 96, true); got != (layout.Rect{}) {
			t.Errorf("VideoArea of the client area %+v = %+v, want nothing", broken, got)
		}
	}
}

func TestInsetsThatAreNoSizeCountAsNone(t *testing.T) {
	client := layout.Rect{W: 800, H: 450}
	whole := layout.Rect{W: 800, H: 450}
	for _, insets := range []layout.Insets{
		{Top: -1},
		{Right: math.NaN()},
		{Bottom: math.Inf(1)},
		{Left: math.Inf(-1)},
	} {
		if insets.Valid() {
			t.Errorf("%+v is valid", insets)
		}
		if got := layout.VideoArea(client, insets, 96, false); got != whole {
			t.Errorf("VideoArea(%+v) = %+v, want the whole client area", insets, got)
		}
	}
	if !(layout.Insets{}).Valid() || !(layout.Insets{Top: 56, Right: 380}).Valid() {
		t.Fatal("ordinary insets are not valid")
	}
}
