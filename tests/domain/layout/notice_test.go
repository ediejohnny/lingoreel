package layout_test

import (
	"math"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
)

func withNotice(in layout.Input, notice string) layout.Input {
	return withNoticeLook(in, notice, appearance.DefaultNoticeLook())
}

func withNoticeLook(in layout.Input, notice string, look appearance.NoticeLook) layout.Input {
	in.Notice = layout.Notice{
		Text:   layout.Text{Style: look.Style(), Content: notice},
		Margin: look.Margin, MaxWidth: look.MaxWidth,
	}
	return in
}

func TestNoticeSitsOnItsSideWithinItsMargin(t *testing.T) {
	t.Parallel()
	v := video(1920, 1080)
	look := appearance.DefaultNoticeLook()
	look.Margin, look.MaxWidth = 0.05, 0.3
	margin := 0.05 * 1080
	for _, side := range []appearance.Alignment{appearance.AlignLeft, appearance.AlignCenter, appearance.AlignRight} {
		look.Side = side
		block := layout.Compute(withNoticeLook(defaults(v, "", ""), "Aviso", look), monoMeasurer{}).Notice
		if block == nil {
			t.Fatalf("%s: no notice", side)
		}
		box := block.Bounds
		if !near(box.Y, margin) || box.X < margin-eps || box.Right() > v.W-margin+eps {
			t.Fatalf("%s: box %+v, want it %v from the top and inside the side margins", side, box, margin)
		}
		var off float64
		switch side {
		case appearance.AlignLeft:
			off = box.X - margin
		case appearance.AlignCenter:
			off = (box.X + box.W/2) - v.W/2
		case appearance.AlignRight:
			off = (v.W - margin) - box.Right()
		}
		if math.Abs(off) > eps {
			t.Fatalf("%s: box %+v is %v off its side", side, box, off)
		}
	}
}

func TestNoticeSitsInTheTopRightCorner(t *testing.T) {
	t.Parallel()
	const notice = "Legenda de estudo (inglês): −0,150 s" // 36 runes
	frame := layout.Compute(withNotice(defaults(video(1920, 1080), "Hi.", ""), notice), monoMeasurer{})
	block := frame.Notice
	if block == nil || len(block.Lines) != 1 {
		t.Fatalf("notice = %+v, want one line", block)
	}
	// Responsive 22 px at 1080p: font 22, lines 27.5 apart, 11 px per rune,
	// padding 8. Margin 3% of 1080 = 32.4 from the top and the right: the
	// box ends at 1887.6 and is 396 + 16 wide.
	line := block.Lines[0]
	if !near(block.Font.Size, 22) || !near(line.X, 1483.6) || !near(line.Y, 40.4) || !near(line.Width, 396) {
		t.Fatalf("line = %+v, font %v; want x 1483.6, y 40.4, width 396, font 22", line, block.Font.Size)
	}
	if want := (layout.Rect{X: 1475.6, Y: 32.4, W: 412, H: 43.5}); !sameRect(block.Bounds, want) {
		t.Fatalf("bounds = %+v, want %+v", block.Bounds, want)
	}
	if frame.Meter != nil {
		t.Fatalf("a notice of text alone got a meter: %+v", frame.Meter)
	}
}

func TestNoticeFollowsTheLetterboxedPicture(t *testing.T) {
	t.Parallel()
	picture := layout.Rect{X: 0, Y: 140, W: 1920, H: 800}
	block := layout.Compute(withNotice(defaults(picture, "", ""), "Aviso"), monoMeasurer{}).Notice
	// 3% of 800 = 24 below the picture's top edge, not the window's.
	if block == nil || !near(block.Bounds.Y, 164) || !near(block.Bounds.Right(), 1920-24) {
		t.Fatalf("notice = %+v, want its box 24 from the picture's top and right", block)
	}
}

// The volume's notice is larger and keeps its box while the level changes
// (a held key would make a box that follows the text jump), with the bar
// under the words, inside the box, across its whole width.
func TestALevelNoticeKeepsItsBoxWhileTheLevelChanges(t *testing.T) {
	t.Parallel()
	v := video(1920, 1080)
	look := appearance.DefaultNoticeLook().WithLevel()
	in := func(text string, level float64) layout.Input {
		in := withNoticeLook(defaults(v, "", ""), text, look)
		in.Notice.Meter = &layout.Meter{Level: level, Mark: 0.5}
		return in
	}
	low := layout.Compute(in("Volume 5%", 0.025), monoMeasurer{})
	high := layout.Compute(in("Volume 135%", 0.675), monoMeasurer{})
	if low.Notice == nil || low.Meter == nil || high.Notice == nil || high.Meter == nil {
		t.Fatalf("notices %+v / %+v, want both with a meter", low, high)
	}
	if !near(low.Notice.Font.Size, 33) {
		t.Fatalf("font %v, want 1.5 × 22 at 1080p", low.Notice.Font.Size)
	}
	if !sameRect(low.Notice.Bounds, high.Notice.Bounds) || !sameRect(low.Meter.Bar, high.Meter.Bar) {
		t.Fatalf("box %+v / bar %+v at 5%%, %+v / %+v at 135%%: want the same", low.Notice.Bounds, low.Meter.Bar,
			high.Notice.Bounds, high.Meter.Bar)
	}
	box, bar, line := high.Notice.Bounds, high.Meter.Bar, high.Notice.Lines[0]
	// Padding 8 and gap 6 × 1.0 scale; the bar is 6 thick.
	if !near(bar.X, box.X+8) || !near(bar.Right(), box.Right()-8) || !near(bar.Bottom(), box.Bottom()-8) || !near(bar.H, 6) {
		t.Fatalf("bar %+v, want it 8 inside the box %+v and 6 thick", bar, box)
	}
	if bar.Y < line.Y+line.Height+6-eps {
		t.Fatalf("bar at %v, want it 6 below the text ending at %v", bar.Y, line.Y+line.Height)
	}
	if !near(box.Right(), v.W-0.03*v.H) || !near(box.Y, 0.03*v.H) {
		t.Fatalf("box %+v, want it in the top right corner", box)
	}
	if high.Meter.Level != 0.675 || high.Meter.Mark != 0.5 {
		t.Fatalf("meter %+v, want the level and mark asked for", high.Meter.Meter)
	}
}

func TestNoticeNeverMovesTheSubtitles(t *testing.T) {
	t.Parallel()
	in := defaults(video(1280, 720), "I'm not going there.", "Eu não vou para lá.")
	without := layout.Compute(in, monoMeasurer{})
	with := layout.Compute(withNotice(in, strings.Repeat("aviso comprido ", 20)), monoMeasurer{})
	if !sameRect(with.Primary.Bounds, without.Primary.Bounds) || !sameRect(with.Translation.Bounds, without.Translation.Bounds) {
		t.Fatalf("subtitles moved: %+v / %+v, want %+v / %+v", with.Primary.Bounds, with.Translation.Bounds, without.Primary.Bounds, without.Translation.Bounds)
	}
	if without.Notice != nil {
		t.Fatal("a notice appeared without text")
	}
}

func TestLongNoticeWrapsInsideItsCornerAndKeepsEveryWord(t *testing.T) {
	t.Parallel()
	notice := strings.Repeat("Trecho pausado — fora do intervalo ", 6)
	v := video(1280, 720)
	block := layout.Compute(withNotice(defaults(v, "", ""), notice), monoMeasurer{}).Notice
	if block == nil {
		t.Fatal("no notice")
	}
	var words []string
	right := 1280 - 0.03*720
	for _, line := range block.Lines {
		// The box may take 60% of the width up to the 3% margin.
		if line.X < right-0.6*1280-eps || line.X+line.Width > right+eps {
			t.Errorf("line %q spans %v to %v, past the notice's 60%% of the width up to %v", line.Text, line.X, line.X+line.Width, right)
		}
		words = append(words, strings.Fields(line.Text)...)
	}
	if got, want := strings.Join(words, " "), strings.Join(strings.Fields(notice), " "); got != want {
		t.Fatalf("notice text changed:\n got  %q\n want %q", got, want)
	}
	if block.Bounds.Bottom() > v.H-0.03*720+eps {
		t.Fatalf("notice reaches %v, past the bottom margin", block.Bounds.Bottom())
	}
}

func TestNoticeOnATinyVideoKeepsAReadableFont(t *testing.T) {
	t.Parallel()
	block := layout.Compute(withNotice(defaults(video(320, 180), "", ""), "Aviso"), monoMeasurer{}).Notice
	if block == nil || block.Font.Size < minFontAt96-eps || math.IsNaN(block.Font.Size) {
		t.Fatalf("notice = %+v, want a font of at least %d px", block, minFontAt96)
	}
}
