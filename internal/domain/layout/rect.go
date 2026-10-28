package layout

import "math"

// Rect is an axis-aligned rectangle in pixels.
type Rect struct {
	X, Y, W, H float64
}

// Right returns the x of the right edge.
func (r Rect) Right() float64 { return r.X + r.W }

// Bottom returns the y of the bottom edge.
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Valid reports whether the rectangle has a finite, positive size.
func (r Rect) Valid() bool {
	for _, v := range []float64{r.X, r.Y, r.W, r.H} {
		if !Finite(v) {
			return false
		}
	}
	return r.W > 0 && r.H > 0
}

// Finite tells a number is neither NaN nor infinite.
func Finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Contains reports whether a point lies inside the rectangle; the right and
// bottom edges are outside.
func (r Rect) Contains(x, y float64) bool {
	return x >= r.X && x < r.Right() && y >= r.Y && y < r.Bottom()
}

// VideoRect fits a video of the given size inside an area, keeping its aspect
// ratio and centering it (black bars above and below, or on the sides). An
// unknown video size fills the area.
func VideoRect(area Rect, videoWidth, videoHeight int) Rect {
	if !area.Valid() || videoWidth <= 0 || videoHeight <= 0 {
		return area
	}
	videoAspect := float64(videoWidth) / float64(videoHeight)
	if area.W/area.H > videoAspect {
		width := area.H * videoAspect
		return Rect{X: area.X + (area.W-width)/2, Y: area.Y, W: width, H: area.H}
	}
	height := area.W / videoAspect
	return Rect{X: area.X, Y: area.Y + (area.H-height)/2, W: area.W, H: height}
}
