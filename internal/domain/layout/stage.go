package layout

import "math"

// Insets are the room a window's own controls take on each side of the
// video, in pixels at 100% scaling (the app's panel measures them so).
type Insets struct {
	Top, Right, Bottom, Left float64
}

// Valid reports whether every inset is finite and not negative.
func (i Insets) Valid() bool {
	for _, v := range []float64{i.Top, i.Right, i.Bottom, i.Left} {
		if !(v >= 0) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// VideoArea is the part of a window's client area left to the video: all
// of it in fullscreen, otherwise what the insets leave, scaled to the
// window's dpi (96 is 100%). In whole pixels, inside the client area; empty
// when nothing is left. Insets that are not valid count as none.
func VideoArea(client Rect, insets Insets, dpi int, fullscreen bool) Rect {
	if !client.Valid() {
		return Rect{}
	}
	width, height := math.Floor(client.W), math.Floor(client.H)
	if fullscreen || !insets.Valid() {
		return Rect{X: client.X, Y: client.Y, W: width, H: height}
	}
	scale := float64(dpi) / BaseDPI
	if !(scale > 0) {
		scale = 1
	}
	left, right := math.Round(insets.Left*scale), math.Round(insets.Right*scale)
	top, bottom := math.Round(insets.Top*scale), math.Round(insets.Bottom*scale)
	w, h := width-left-right, height-top-bottom
	if !(w >= 1 && h >= 1) {
		return Rect{}
	}
	return Rect{X: client.X + left, Y: client.Y + top, W: w, H: h}
}
