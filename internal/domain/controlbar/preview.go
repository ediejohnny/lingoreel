package controlbar

import (
	"math"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// Sizes of the seek preview at 100% scaling, in pixels.
const (
	// previewWidth and previewHeight are the box the still fits in, keeping
	// its shape: a video site's.
	previewWidth  = 240.0
	previewHeight = 135.0
	// previewGap is between the track and the time; labelGap between the
	// time and the still; previewMargin keeps the preview off the edges.
	previewGap    = 6.0
	labelGap      = 4.0
	previewMargin = 6.0
)

// Preview is where the seek preview goes, above the track: the still and,
// under it, the time. A zero Picture shows the time alone.
type Preview struct {
	Box, Picture, Label layout.Rect
}

// PreviewAt places the preview for the pointer at x, centred on it above the
// track and kept inside the area (at an end of the track it rests against
// the area's edge). aspect is the still's width over its
// height (0 for the time alone); labelW and labelH are the time's size.
// With no room for the still the time shows alone, and with no room for the
// time there is no preview.
func (b Bar) PreviewAt(area layout.Rect, x, aspect, labelW, labelH float64) (Preview, bool) {
	s := b.Scale
	if !area.Valid() || !b.Track.Valid() || !(s > 0) || !layout.Finite(x) || !layout.Finite(labelW) || !layout.Finite(labelH) || labelW <= 0 || labelH <= 0 {
		return Preview{}, false
	}
	label := layout.Rect{W: labelW, H: labelH, Y: b.Track.Y - previewGap*s - labelH}
	if label.Y < area.Y || label.W > area.W-2*previewMargin*s {
		return Preview{}, false
	}
	var picture layout.Rect
	if layout.Finite(aspect) && aspect > 0 {
		width := math.Min(math.Min(previewWidth, previewHeight*aspect)*s, area.W-2*previewMargin*s)
		picture = layout.Rect{W: width, H: width / aspect}
		picture.Y = label.Y - labelGap*s - picture.H
		if picture.Y < area.Y || !picture.Valid() {
			picture = layout.Rect{}
		}
	}
	// The column of the still and the time is centred on the pointer, then
	// kept inside the area as one.
	column := math.Max(picture.W, label.W)
	left := math.Min(math.Max(x-column/2, area.X+previewMargin*s), area.Right()-previewMargin*s-column)
	label.X = left + (column-label.W)/2
	box := label
	if picture.W > 0 {
		picture.X = left + (column-picture.W)/2
		box = layout.Rect{X: left, Y: picture.Y, W: column, H: label.Bottom() - picture.Y}
	}
	return Preview{Box: box, Picture: picture, Label: label}, true
}
