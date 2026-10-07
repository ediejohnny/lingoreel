package controlbar_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

// The seek preview: the still over the time, both above the pointer, inside
// the video. Like the bar's tests, these check how the parts relate.

// within reports whether r lies inside area, at least margin off its edges.
func within(r, area layout.Rect, margin float64) bool {
	return r.X >= area.X+margin && r.Y >= area.Y && r.Right() <= area.Right()-margin && r.Bottom() <= area.Bottom()
}

func centre(r layout.Rect) float64 { return r.X + r.W/2 }

func TestThePreviewSitsAboveThePointer(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	x := bar.Track.X + bar.Track.W*0.4
	p, ok := bar.PreviewAt(wide, x, 16.0/9, 60, 20)
	switch {
	case !ok:
		t.Fatal("no preview over a wide video")
	case math.Abs(centre(p.Picture)-x) > 0.5 || math.Abs(centre(p.Label)-x) > 0.5:
		t.Fatalf("still centred on %v, time on %v, pointer at %v", centre(p.Picture), centre(p.Label), x)
	case p.Label.Bottom() >= bar.Track.Y || p.Picture.Bottom() > p.Label.Y:
		t.Fatalf("not still over time over track: still %+v, time %+v, track %+v", p.Picture, p.Label, bar.Track)
	case p.Box.Y != p.Picture.Y || p.Box.Bottom() != p.Label.Bottom() || !within(p.Picture, p.Box, 0) || !within(p.Label, p.Box, 0):
		t.Fatalf("box %+v does not hold still %+v and time %+v", p.Box, p.Picture, p.Label)
	case !within(p.Box, wide, 1):
		t.Fatalf("box %+v outside the video", p.Box)
	}
}

// At the ends of the track and past them the preview stays inside the video,
// resting against its edge.
func TestThePreviewStaysInsideTheVideoAtTheEdges(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	end, _ := bar.PreviewAt(wide, bar.Track.Right(), 16.0/9, 60, 20)
	for _, x := range []float64{-500, 0, bar.Track.X, bar.Track.Right(), wide.W, 5000} {
		p, ok := bar.PreviewAt(wide, x, 16.0/9, 60, 20)
		if !ok || !within(p.Box, wide, 1) {
			t.Errorf("pointer at %v: box %+v (%v), want inside the video", x, p.Box, ok)
		}
		if x > bar.Track.Right() && p != end {
			t.Errorf("pointer at %v past the track: %+v, want where the track ends %+v", x, p, end)
		}
	}
}

// The still keeps its shape in the same box, grows with the scale and
// shrinks to fit a narrow window.
func TestThePreviewKeepsTheStillsShape(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	x := bar.Track.X + bar.Track.W/2
	shape := func(bar controlbar.Bar, area layout.Rect, aspect float64) layout.Rect {
		t.Helper()
		p, ok := bar.PreviewAt(area, x, aspect, 60, 20)
		if !ok || p.Picture.W <= 0 || math.Abs(p.Picture.W/p.Picture.H-aspect) > 1e-9 {
			t.Fatalf("aspect %v: still %+v (%v)", aspect, p.Picture, ok)
		}
		return p.Picture
	}
	film, square, phone := shape(bar, wide, 16.0/9), shape(bar, wide, 4.0/3), shape(bar, wide, 9.0/16)
	if square.H != film.H || phone.H != film.H || square.W >= film.W || phone.W >= square.W {
		t.Fatalf("16:9 %+v, 4:3 %+v, 9:16 %+v: want the same height, narrower", film, square, phone)
	}
	if double := shape(controlbar.Layout(wide, 2, fullState()), wide, 16.0/9); math.Abs(double.W-2*film.W) > 1e-9 {
		t.Fatalf("at twice the scale the still is %v wide, want %v", double.W, 2*film.W)
	}
	narrow := layout.Rect{W: 100, H: 720}
	if small := shape(controlbar.Layout(narrow, 1, fullState()), narrow, 16.0/9); small.W >= film.W || small.W > narrow.W {
		t.Fatalf("in a 100 px window the still is %v wide", small.W)
	}
}

// With no room above the track for the still the time shows alone; with no
// room for the time, or nothing to measure, there is no preview.
func TestNoRoomDropsTheStillThenThePreview(t *testing.T) {
	t.Parallel()
	low := layout.Rect{W: 1280, H: 110}
	bar := controlbar.Layout(low, 1, fullState())
	if p, ok := bar.PreviewAt(low, 300, 16.0/9, 60, 20); !ok || p.Picture != (layout.Rect{}) || p.Box != p.Label {
		t.Fatalf("a low window gave %+v (%v), want the time alone", p, ok)
	}
	if p, ok := controlbar.Layout(wide, 1, fullState()).PreviewAt(wide, 300, 0, 60, 20); !ok || p.Picture != (layout.Rect{}) {
		t.Fatalf("no still asked for gave %+v (%v), want the time alone", p, ok)
	}
	lower := layout.Rect{W: 1280, H: 70}
	if p, ok := controlbar.Layout(lower, 1, fullState()).PreviewAt(lower, 300, 16.0/9, 60, 20); ok {
		t.Fatalf("no room for the time gave %+v", p)
	}
	full := controlbar.Layout(wide, 1, fullState())
	for _, c := range []struct{ x, w, h float64 }{{math.NaN(), 60, 20}, {300, 0, 20}, {300, 60, math.Inf(1)}, {300, 2000, 20}} {
		if p, ok := full.PreviewAt(wide, c.x, 16.0/9, c.w, c.h); ok {
			t.Errorf("pointer %v, time %vx%v gave %+v", c.x, c.w, c.h, p)
		}
	}
}
