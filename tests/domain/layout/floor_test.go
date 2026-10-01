package layout_test

import (
	"math"
	"testing"

	"lingoreel/internal/domain/layout"
)

// lowest is the bottom of the lowest block of a frame.
func lowest(frame layout.Frame) float64 {
	bottom := 0.0
	for _, block := range []*layout.Block{frame.Primary, frame.Translation} {
		if block != nil {
			bottom = math.Max(bottom, block.Bounds.Bottom())
		}
	}
	return bottom
}

func TestFloorKeepsTheSubtitlesAboveTheControls(t *testing.T) {
	in := defaults(video(1280, 720), "Hello there, how are you?", "Olá, tudo bem?")
	free := layout.Compute(in, monoMeasurer{})
	if lowest(free) <= 600 {
		t.Fatalf("the default layout already ends at %v; the test needs subtitles near the bottom", lowest(free))
	}
	in.Floor = 600
	raised := layout.Compute(in, monoMeasurer{})
	if got := lowest(raised); got > 600+eps {
		t.Fatalf("subtitles end at %v, below the floor 600", got)
	}
	// Same text, same size: the block only moves up.
	if raised.Primary.Font != free.Primary.Font || len(raised.Primary.Lines) != len(free.Primary.Lines) {
		t.Fatalf("the floor changed the primary's text: %+v vs %+v", raised.Primary, free.Primary)
	}
	// The translation still never overlaps the primary.
	if p, tr := raised.Primary.Bounds, raised.Translation.Bounds; p.Y < tr.Bottom()-eps && tr.Y < p.Bottom()-eps {
		t.Fatalf("blocks overlap: %+v and %+v", p, tr)
	}
}

func TestFloorThatChangesNothing(t *testing.T) {
	in := defaults(video(1280, 720), "Hello there.", "Olá.")
	free := layout.Compute(in, monoMeasurer{})
	for _, floor := range []float64{0, -10, 720, 5000, math.NaN(), math.Inf(1)} {
		in.Floor = floor
		got := layout.Compute(in, monoMeasurer{})
		if got.Primary.Bounds != free.Primary.Bounds || got.Translation.Bounds != free.Translation.Bounds {
			t.Errorf("Floor %v moved the subtitles: %+v", floor, got.Primary.Bounds)
		}
	}
}

func TestFloorAboveTheSafeAreaKeepsTheText(t *testing.T) {
	in := defaults(video(1280, 720), "Hello there.", "Olá.")
	in.Floor = 1 // a window too small for the controls and the subtitles
	frame := layout.Compute(in, monoMeasurer{})
	if frame.Primary == nil || frame.Primary.Lines[0].Text != "Hello there." {
		t.Fatalf("text lost under a floor at the top: %+v", frame.Primary)
	}
}
