package layout

import "math"

// InitialWindowFraction is the largest share of the monitor's work area a
// player window opens with, leaving room for its borders and title bar.
const InitialWindowFraction = 0.8

// defaultAspect is used when the video size is unknown.
const defaultAspect = 16.0 / 9.0

// FitWindow returns where a player window should open: the video's own size
// when it fits within fraction of the work area, otherwise scaled down to
// fit, keeping the aspect ratio; centered, in whole pixels. Small videos are
// never enlarged. After it opens, the user resizes it at will.
func FitWindow(videoWidth, videoHeight int, workArea Rect, fraction float64) Rect {
	if !workArea.Valid() || !(fraction > 0 && fraction <= 1) {
		return Rect{}
	}
	maxW, maxH := workArea.W*fraction, workArea.H*fraction
	width, height := float64(videoWidth), float64(videoHeight)
	if videoWidth <= 0 || videoHeight <= 0 {
		width, height = maxW, maxW/defaultAspect
	}
	if scale := math.Min(maxW/width, maxH/height); scale < 1 {
		width, height = width*scale, height*scale
	}
	width, height = math.Max(math.Floor(width), 1), math.Max(math.Floor(height), 1)
	return Rect{
		X: workArea.X + math.Floor((workArea.W-width)/2),
		Y: workArea.Y + math.Floor((workArea.H-height)/2),
		W: width, H: height,
	}
}
