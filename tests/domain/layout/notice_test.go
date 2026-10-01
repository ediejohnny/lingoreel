package layout_test

import (
	"math"
	"strings"
	"testing"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/layout"
)

func withNotice(in layout.Input, notice string) layout.Input {
	in.Notice = layout.Text{Style: appearance.NoticeStyle(), Content: notice}
	return in
}

func TestNoticeSitsInTheTopLeftCorner(t *testing.T) {
	const notice = "Legenda de cima (inglês): −0,150 s" // 34 runes
	frame := layout.Compute(withNotice(defaults(video(1920, 1080), "Hi.", ""), notice), monoMeasurer{})
	block := frame.Notice
	if block == nil || len(block.Lines) != 1 {
		t.Fatalf("notice = %+v, want one line", block)
	}
	// Responsive 22 px at 1080p: font 22, lines 27.5 apart, 11 px per rune,
	// padding 8. Margin 3% of 1080 = 32.4 from the top and the left.
	line := block.Lines[0]
	if !near(block.Font.Size, 22) || !near(line.X, 40.4) || !near(line.Y, 40.4) || !near(line.Width, 374) {
		t.Fatalf("line = %+v, font %v; want x 40.4, y 40.4, width 374, font 22", line, block.Font.Size)
	}
	if want := (layout.Rect{X: 32.4, Y: 32.4, W: 390, H: 43.5}); !sameRect(block.Bounds, want) {
		t.Fatalf("bounds = %+v, want %+v", block.Bounds, want)
	}
}

func TestNoticeFollowsTheLetterboxedPicture(t *testing.T) {
	picture := layout.Rect{X: 0, Y: 140, W: 1920, H: 800}
	block := layout.Compute(withNotice(defaults(picture, "", ""), "Aviso"), monoMeasurer{}).Notice
	// 3% of 800 = 24 below the picture's top edge, not the window's.
	if block == nil || !near(block.Bounds.Y, 164) || !near(block.Bounds.X, 24) {
		t.Fatalf("notice = %+v, want its box at (24, 164)", block)
	}
}

func TestNoticeNeverMovesTheSubtitles(t *testing.T) {
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
	notice := strings.Repeat("Trecho pausado — fora do intervalo ", 6)
	v := video(1280, 720)
	block := layout.Compute(withNotice(defaults(v, "", ""), notice), monoMeasurer{}).Notice
	if block == nil {
		t.Fatal("no notice")
	}
	var words []string
	for _, line := range block.Lines {
		// The box may take 60% of the width from the 3% margin on.
		if line.X+line.Width > 0.03*720+0.6*1280+eps {
			t.Errorf("line %q ends at %v, past the notice's 60%% of the width", line.Text, line.X+line.Width)
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
	block := layout.Compute(withNotice(defaults(video(320, 180), "", ""), "Aviso"), monoMeasurer{}).Notice
	if block == nil || block.Font.Size < minFontAt96-eps || math.IsNaN(block.Font.Size) {
		t.Fatalf("notice = %+v, want a font of at least %d px", block, minFontAt96)
	}
}
