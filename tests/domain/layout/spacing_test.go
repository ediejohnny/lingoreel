package layout_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

func TestLineSpacingSpreadsTheLinesAndKeepsTheBlocksBottom(t *testing.T) {
	t.Parallel()
	in := defaults(video(1920, 1080), "Where are you going?\nHome.", "")
	in.Primary.Style.Background.Enabled = false
	tight := layout.Compute(in, monoMeasurer{}).Primary
	in.Primary.Style.LineSpacing = 1.5
	spread := layout.Compute(in, monoMeasurer{}).Primary
	if tight == nil || spread == nil || len(tight.Lines) != 2 || len(spread.Lines) != 2 {
		t.Fatalf("blocks: %+v / %+v, want two lines each", tight, spread)
	}
	// monoMeasurer: lines 1.25 font sizes apart, so 1.5 times that once spread.
	natural := 1.25 * spread.Font.Size
	if got := spread.Lines[1].Y - spread.Lines[0].Y; math.Abs(got-1.5*natural) > eps {
		t.Fatalf("lines %v apart, want %v", got, 1.5*natural)
	}
	if !near(spread.Lines[0].Height, natural) {
		t.Fatalf("line height %v, want the font's own %v", spread.Lines[0].Height, natural)
	}
	// The room added is split above and below each line, so the text stays
	// in the middle of its slot.
	above := spread.Lines[0].Y - spread.Bounds.Y
	below := spread.Bounds.Bottom() - (spread.Lines[1].Y + spread.Lines[1].Height)
	if !near(above, 0.25*natural) || !near(below, 0.25*natural) {
		t.Fatalf("room above %v and below %v, want %v each", above, below, 0.25*natural)
	}
	// The style's height is where the block's bottom sits: it grows upwards.
	if !near(spread.Bounds.Bottom(), tight.Bounds.Bottom()) {
		t.Fatalf("bottom moved from %v to %v", tight.Bounds.Bottom(), spread.Bounds.Bottom())
	}
}

func TestTheGapKeepsTheSubtitlesApartOnlyWhenTheyMeet(t *testing.T) {
	t.Parallel()
	for _, gap := range []float64{0, 0.2, 1, 2} {
		// The translation asks to sit right below the primary.
		in := defaults(video(1920, 1080), "I'm not going there.", "Eu não vou para lá.")
		in.Translation.Style.VerticalPosition = in.Primary.Style.VerticalPosition + 0.01
		in.Layout.LineGap = gap
		frame := layout.Compute(in, monoMeasurer{})
		line := 1.25 * frame.Translation.Font.Size
		if got := frame.Translation.Bounds.Y - frame.Primary.Bounds.Bottom(); math.Abs(got-gap*line) > eps {
			t.Fatalf("gap %v: %v px between the subtitles, want %v (lines of %v)", gap, got, gap*line, line)
		}
	}
	// Far apart, each stays at its own height whatever the gap.
	far := defaults(video(1920, 1080), "I'm not going there.", "Eu não vou para lá.")
	before := layout.Compute(far, monoMeasurer{}).Translation.Bounds
	far.Layout.LineGap = 1
	if after := layout.Compute(far, monoMeasurer{}).Translation.Bounds; !sameRect(before, after) {
		t.Fatalf("translation moved from %+v to %+v though it was far from the primary", before, after)
	}
}
