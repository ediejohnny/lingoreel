package layout_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

func TestVideoRectKeepsTheAspectRatio(t *testing.T) {
	t.Parallel()
	area := layout.Rect{X: 10, Y: 20, W: 1920, H: 1080}
	cases := []struct {
		name          string
		width, height int
		want          layout.Rect
	}{
		{"mesma proporção", 1280, 720, area},
		// 2.4:1 in 16:9: 1920 wide, 800 high, 140 above and below.
		{"cinemascope", 2400, 1000, layout.Rect{X: 10, Y: 160, W: 1920, H: 800}},
		// 4:3 in 16:9: 1440 wide, 240 on each side.
		{"4:3", 640, 480, layout.Rect{X: 250, Y: 20, W: 1440, H: 1080}},
		{"largura zero", 0, 720, area},
		{"altura zero", 1280, 0, area},
		{"negativo", -1280, 720, area},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := layout.VideoRect(area, c.width, c.height); !sameRect(got, c.want) {
				t.Fatalf("VideoRect = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestVideoRectOfAnInvalidAreaIsTheArea(t *testing.T) {
	t.Parallel()
	for _, area := range []layout.Rect{{}, {W: -5, H: 10}, {W: math.NaN(), H: 10}, {W: 10, H: math.Inf(1)}} {
		got := layout.VideoRect(area, 1920, 1080)
		nanKept := math.IsNaN(got.W) && math.IsNaN(area.W) && got.H == area.H
		if !sameRect(got, area) && !nanKept {
			t.Errorf("VideoRect(%+v) = %+v, want the area back", area, got)
		}
	}
}

func TestRectValidAndContains(t *testing.T) {
	t.Parallel()
	r := layout.Rect{X: 0, Y: 0, W: 10, H: 5}
	for _, p := range [][2]float64{{0, 0}, {9.99, 4.99}} {
		if !r.Contains(p[0], p[1]) {
			t.Errorf("Contains(%v) = false", p)
		}
	}
	for _, p := range [][2]float64{{10, 0}, {0, 5}, {-0.01, 1}, {math.NaN(), 1}} {
		if r.Contains(p[0], p[1]) {
			t.Errorf("Contains(%v) = true, right/bottom edges and NaN are outside", p)
		}
	}
	for _, bad := range []layout.Rect{{W: 0, H: 1}, {W: 1, H: 0}, {W: -1, H: 1}, {X: math.NaN(), W: 1, H: 1}, {W: math.Inf(1), H: 1}} {
		if bad.Valid() {
			t.Errorf("%+v is Valid", bad)
		}
	}
}

func sameRect(a, b layout.Rect) bool {
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.W, b.W) && near(a.H, b.H)
}

// near compares with a tolerance; equal infinities are near, NaN never is.
func near(a, b float64) bool {
	return a == b || math.Abs(a-b) < 1e-9
}
