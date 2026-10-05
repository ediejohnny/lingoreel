package potplayer_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/potplayer"
)

// The windows PotPlayer really has inside its main window (measured): a
// container that is not a view, the picture on the left and the playlist on
// the right, plus the small controls of its own bar.
func potPlayerChildren() []potplayer.AreaCandidate {
	return []potplayer.AreaCandidate{
		{Handle: 1, Class: "AfxWnd140su", Visible: true, Width: 1256, Height: 700},
		{Handle: 2, Class: "Afx:00007FF6:8:00010003:00000000:00000000", Visible: true, Width: 960, Height: 540},
		{Handle: 3, Class: "Afx:00007FF6:8:00010003:00000000:00000001", Visible: true, Width: 296, Height: 540},
		{Handle: 4, Class: "Button", Visible: true, Width: 24, Height: 24},
	}
}

func TestChooseAreaPicksThePictureNotTheContainerOrThePlaylist(t *testing.T) {
	got, found := potplayer.ChooseArea(potPlayerChildren())
	if !found || got != 2 {
		t.Fatalf("ChooseArea = %d, %v; want the picture (2)", got, found)
	}
}

// The playlist alone must never be covered: with the picture hidden there
// is no area, and the subtitles go over the whole window.
func TestChooseAreaSkipsHiddenAndEmptyWindows(t *testing.T) {
	hidden := []potplayer.AreaCandidate{
		{Handle: 1, Class: "AfxWnd140su", Visible: true, Width: 1256, Height: 700},
		{Handle: 2, Class: "Afx:1", Visible: false, Width: 1920, Height: 1080},
		{Handle: 3, Class: "Afx:2", Visible: true, Width: 0, Height: 540},
		{Handle: 4, Class: "Afx:3", Visible: true, Width: 300, Height: 0},
	}
	if got, found := potplayer.ChooseArea(hidden); found || got != 0 {
		t.Fatalf("ChooseArea = %d, %v; want none", got, found)
	}
	if got, found := potplayer.ChooseArea(nil); found || got != 0 {
		t.Fatalf("ChooseArea(nil) = %d, %v; want none", got, found)
	}
}

// A window of the toolkit's plain class is never the picture, however big.
func TestChooseAreaIgnoresWindowsThatAreNotViews(t *testing.T) {
	candidates := []potplayer.AreaCandidate{
		{Handle: 1, Class: "AfxWnd140su", Visible: true, Width: 3840, Height: 2160},
		{Handle: 2, Class: "", Visible: true, Width: 3840, Height: 2160},
		{Handle: 3, Class: "afx:lower", Visible: true, Width: 3840, Height: 2160},
		{Handle: 4, Class: "Afx:small", Visible: true, Width: 320, Height: 180},
	}
	if got, found := potplayer.ChooseArea(candidates); !found || got != 4 {
		t.Fatalf("ChooseArea = %d, %v; want the only view (4)", got, found)
	}
}

func TestChooseAreaTiesKeepTheFirst(t *testing.T) {
	tie := []potplayer.AreaCandidate{
		{Handle: 7, Class: "Afx:a", Visible: true, Width: 200, Height: 100},
		{Handle: 8, Class: "Afx:b", Visible: true, Width: 100, Height: 200},
	}
	if got, _ := potplayer.ChooseArea(tie); got != 7 {
		t.Fatalf("ChooseArea = %d, want 7", got)
	}
}
