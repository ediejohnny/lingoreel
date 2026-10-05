package layout_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// The user's monitor: 1440x900 with a 40 px taskbar at the bottom.
var workArea = layout.Rect{W: 1440, H: 860}

func TestFitWindowShrinksBigVideosIntoTheScreen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		width, height int
		want          layout.Rect
	}{
		// 1920x1040 (the real movie) into 1152x688 (80%): 1920/1040 = 1.85
		// is wider than 1152/688 = 1.67, so width limits: scale 0.6 →
		// 1152x624, centered at ((1440-1152)/2, (860-624)/2).
		{"full HD largo", 1920, 1040, layout.Rect{X: 144, Y: 118, W: 1152, H: 624}},
		// 1920x800 (2.4:1): width limits → 1152x480.
		{"cinemascope", 1920, 800, layout.Rect{X: 144, Y: 190, W: 1152, H: 480}},
		// 3840x2160 (4K) → 1152x648.
		{"4K", 3840, 2160, layout.Rect{X: 144, Y: 106, W: 1152, H: 648}},
		// A small video keeps its size, centered.
		{"pequeno", 320, 180, layout.Rect{X: 560, Y: 340, W: 320, H: 180}},
		// Exactly at the limit is not scaled.
		{"no limite", 1152, 688, layout.Rect{X: 144, Y: 86, W: 1152, H: 688}},
	}
	for _, c := range cases {
		got := layout.FitWindow(c.width, c.height, workArea, layout.InitialWindowFraction)
		if got != c.want {
			t.Errorf("%s: FitWindow = %+v, want %+v", c.name, got, c.want)
		}
		insideScreen(t, c.name, got, workArea)
		if c.width > 0 && c.height > 0 {
			if aspect, want := got.W/got.H, float64(c.width)/float64(c.height); math.Abs(aspect-want) > 0.01 {
				t.Errorf("%s: aspect %v, want %v", c.name, aspect, want)
			}
		}
	}
}

func insideScreen(t *testing.T, name string, got, area layout.Rect) {
	t.Helper()
	if got.X < area.X || got.Y < area.Y || got.Right() > area.Right() || got.Bottom() > area.Bottom() {
		t.Errorf("%s: %+v leaves the work area %+v", name, got, area)
	}
	if got.X != math.Floor(got.X) || got.W != math.Floor(got.W) {
		t.Errorf("%s: %+v is not in whole pixels", name, got)
	}
}

func TestFitWindowOnAMonitorThatDoesNotStartAtZero(t *testing.T) {
	t.Parallel()
	second := layout.Rect{X: 1440, Y: -200, W: 1920, H: 1040}
	got := layout.FitWindow(1920, 1080, second, layout.InitialWindowFraction)
	insideScreen(t, "segundo monitor", got, second)
}

func TestFitWindowWithUnknownOrHostileInput(t *testing.T) {
	t.Parallel()
	// Unknown video size: a 16:9 window at the limit.
	got := layout.FitWindow(0, 0, workArea, layout.InitialWindowFraction)
	if got.W != 1152 || got.H != 648 {
		t.Errorf("unknown size = %+v, want 1152x648", got)
	}
	insideScreen(t, "desconhecido", got, workArea)
	for _, size := range [][2]int{{-1920, 1080}, {1920, -1080}} {
		insideScreen(t, "negativo", layout.FitWindow(size[0], size[1], workArea, layout.InitialWindowFraction), workArea)
	}
	// An absurd aspect still yields at least one pixel each way.
	thin := layout.FitWindow(100000, 1, workArea, layout.InitialWindowFraction)
	if thin.W < 1 || thin.H < 1 {
		t.Errorf("thin video = %+v", thin)
	}
	insideScreen(t, "fino", thin, workArea)

	for _, fraction := range []float64{0, -1, 1.5, math.NaN()} {
		if got := layout.FitWindow(1920, 1080, workArea, fraction); got != (layout.Rect{}) {
			t.Errorf("fraction %v: %+v, want nothing", fraction, got)
		}
	}
	for _, area := range []layout.Rect{{}, {W: -1, H: 100}, {W: math.NaN(), H: 100}} {
		if got := layout.FitWindow(1920, 1080, area, layout.InitialWindowFraction); got != (layout.Rect{}) {
			t.Errorf("area %+v: %+v, want nothing", area, got)
		}
	}
}
