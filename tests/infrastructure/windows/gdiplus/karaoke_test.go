//go:build windows && amd64

package gdiplus_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

var (
	previewGrey = appearance.Color{R: 0xC8, G: 0xCD, B: 0xD4, A: 0xFF}
	fillYellow  = appearance.Color{R: 0xFF, G: 0xD2, B: 0x3F, A: 0xFF}
)

// karaokeFrame lays out one primary line on a 1920x1080 picture.
func karaokeFrame(t testing.TB, r *gdiplus.Renderer) (layout.Frame, gdiplus.Look) {
	t.Helper()
	style := appearance.DefaultPrimaryStyle()
	frame := layout.Compute(layout.Input{
		Video: layout.Rect{W: 1920, H: 1080}, DPI: 96, Layout: appearance.DefaultLayout(),
		Primary: layout.Text{Style: style, Content: "Training the karaoke line"},
	}, r)
	if frame.Primary == nil || len(frame.Primary.Lines) != 1 {
		t.Fatalf("layout: %+v", frame.Primary)
	}
	return frame, gdiplus.Look{Primary: style, Notice: appearance.NoticeStyle(), Preview: previewGrey, FillColor: fillYellow}
}

// colored counts the opaque pixels of a rectangle close to a color.
func colored(dib *win32.DIB, area layout.Rect, c appearance.Color) int {
	count := 0
	for y := int(area.Y); y < int(area.Bottom()); y++ {
		for x := int(area.X); x < int(area.Right()); x++ {
			if x < 0 || y < 0 || x >= dib.Width || y >= dib.Height {
				continue
			}
			p := dib.Pixels[(y*dib.Width+x)*4:]
			if p[3] == 0xFF && near(p[2], c.R) && near(p[1], c.G) && near(p[0], c.B) {
				count++
			}
		}
	}
	return count
}

func near(a, b byte) bool { return int(a)-int(b) < 24 && int(b)-int(a) < 24 }

func TestPreviewBarShrinksWithTheTimeLeft(t *testing.T) {
	r := newRenderer(t)
	frame, look := karaokeFrame(t, r)
	bounds := frame.Primary.Bounds
	// The bar sits above the text: 8 style units of gap, 5 thick.
	strip := layout.Rect{X: bounds.X, Y: bounds.Y - 30, W: bounds.W, H: 30}
	left := layout.Rect{X: bounds.X, Y: strip.Y, W: bounds.W / 4, H: strip.H}
	right := layout.Rect{X: bounds.X + bounds.W*3/4, Y: strip.Y, W: bounds.W / 4, H: strip.H}

	draw := func(cue *gdiplus.CueLook) *win32.DIB {
		dib := newDIB(t, 1920, 1080)
		look.Cue = cue
		if err := r.Draw(dib, frame, look); err != nil {
			t.Fatal(err)
		}
		return dib
	}
	half := draw(&gdiplus.CueLook{Indicator: study.IndicatorBar, Progress: 0.5})
	if colored(half, left, previewGrey) == 0 || colored(half, right, previewGrey) != 0 {
		t.Fatalf("half way: left %d, right %d preview pixels; want the bar on the left half only",
			colored(half, left, previewGrey), colored(half, right, previewGrey))
	}
	if done := draw(&gdiplus.CueLook{Indicator: study.IndicatorBar, Progress: 0}); colored(done, strip, previewGrey) != 0 {
		t.Fatal("a bar at the line's start")
	}
	// The text itself is in the preview color, not the style's.
	if colored(half, bounds, previewGrey) == 0 || colored(half, bounds, look.Primary.Color) > colored(half, bounds, previewGrey) {
		t.Fatal("the preview text is not in the preview color")
	}
	count := draw(&gdiplus.CueLook{Indicator: study.IndicatorCountdown, Seconds: 3})
	centre := layout.Rect{X: bounds.X + bounds.W/2 - 40, Y: bounds.Y - 80, W: 80, H: 80}
	if colored(count, centre, previewGrey) == 0 || colored(count, left, previewGrey) != 0 {
		t.Fatal("the countdown is not a number centered above the line")
	}
}

func TestFillPaintsTheSpokenPart(t *testing.T) {
	r := newRenderer(t)
	frame, look := karaokeFrame(t, r)
	line := frame.Primary.Lines[0]
	look.FillWidths = layout.FillWidths(frame.Primary, 0.5, r)
	dib := newDIB(t, 1920, 1080)
	if err := r.Draw(dib, frame, look); err != nil {
		t.Fatal(err)
	}
	spoken := layout.Rect{X: line.X, Y: line.Y, W: look.FillWidths[0], H: line.Height}
	ahead := layout.Rect{X: line.X + look.FillWidths[0] + 4, Y: line.Y, W: line.Width - look.FillWidths[0] - 4, H: line.Height}
	if colored(dib, spoken, fillYellow) == 0 || colored(dib, ahead, fillYellow) != 0 {
		t.Fatalf("fill: %d yellow pixels spoken, %d ahead", colored(dib, spoken, fillYellow), colored(dib, ahead, fillYellow))
	}
	if colored(dib, ahead, look.Primary.Color) == 0 {
		t.Fatal("the part ahead lost its own color")
	}
}

// maxAlpha is the most opaque pixel of a rectangle.
func maxAlpha(dib *win32.DIB, area layout.Rect) int {
	most := 0
	for y := max(int(area.Y), 0); y < min(int(area.Bottom()), dib.Height); y++ {
		for x := max(int(area.X), 0); x < min(int(area.Right()), dib.Width); x++ {
			most = max(most, int(dib.Pixels[(y*dib.Width+x)*4+3]))
		}
	}
	return most
}

func TestTheFillAndTheBarFollowTheLinesOpacity(t *testing.T) {
	r := newRenderer(t)
	frame, look := karaokeFrame(t, r)
	line := frame.Primary.Lines[0]
	look.FillWidths = layout.FillWidths(frame.Primary, 0.5, r)
	spoken := layout.Rect{X: line.X, Y: line.Y, W: look.FillWidths[0], H: line.Height}
	ahead := layout.Rect{X: line.X + look.FillWidths[0] + 4, Y: line.Y, W: line.Width - look.FillWidths[0] - 4, H: line.Height}
	draw := func(opacity float64, cue *gdiplus.CueLook) *win32.DIB {
		t.Helper()
		look.Primary.Opacity, look.Cue = opacity, cue
		dib := newDIB(t, 1920, 1080)
		if err := r.Draw(dib, frame, look); err != nil {
			t.Fatal(err)
		}
		return dib
	}
	// At 50% the spoken part is as transparent as the part still to come.
	half := draw(0.5, nil)
	if a, b := maxAlpha(half, spoken), maxAlpha(half, ahead); a < 118 || a > 138 || b < 118 || b > 138 {
		t.Fatalf("at 50%%: spoken alpha %d, ahead %d; want both about 128", a, b)
	}
	if hidden := draw(0, nil); maxAlpha(hidden, frame.Primary.Bounds) != 0 {
		t.Fatal("at 0% the spoken part still shows")
	}
	// The bar above a preview fades with it.
	bounds := frame.Primary.Bounds
	strip := layout.Rect{X: bounds.X, Y: bounds.Y - 30, W: bounds.W, H: 22}
	look.FillWidths = nil
	bar := draw(0.5, &gdiplus.CueLook{Indicator: study.IndicatorBar, Progress: 1})
	if a := maxAlpha(bar, strip); a < 118 || a > 138 {
		t.Fatalf("at 50%% the bar has alpha %d; want about 128", a)
	}
}

func TestHiddenPrimaryDrawsNothing(t *testing.T) {
	r := newRenderer(t)
	frame, look := karaokeFrame(t, r)
	look.PrimaryHidden = true
	dib := newDIB(t, 1920, 1080)
	if err := r.Draw(dib, frame, look); err != nil {
		t.Fatal(err)
	}
	for _, b := range dib.Pixels {
		if b != 0 {
			t.Fatal("a hidden primary was drawn")
		}
	}
}

// A karaoke frame at 1080p (clear, text, fill) must fit well inside the 40 ms
// between frames; the target is 8 ms (measured: 4.6 ms).
func BenchmarkKaraokeFrame(b *testing.B) {
	r := newRenderer(b)
	frame, look := karaokeFrame(b, r)
	dib := newDIB(b, 1920, 1080)
	i := 0
	for b.Loop() {
		dib.Clear()
		look.FillWidths = layout.FillWidths(frame.Primary, float64(i%50)/50, r)
		if err := r.Draw(dib, frame, look); err != nil {
			b.Fatal(err)
		}
		i++
	}
}
