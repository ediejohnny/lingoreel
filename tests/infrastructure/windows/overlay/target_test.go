package overlay_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
)

func TestChooseTargetPicksTheLargestVisibleWindow(t *testing.T) {
	// The real VLC process: a hidden hotkeys window, two hidden IME windows
	// and the visible video window.
	vlc := []overlay.Candidate{
		{Handle: 1, Visible: false, Width: 130, Height: 10},
		{Handle: 2, Visible: false},
		{Handle: 3, Visible: true, Width: 320, Height: 180},
		{Handle: 4, Visible: false},
	}
	if got, found := overlay.ChooseTarget(vlc); !found || got.Handle != 3 {
		t.Fatalf("ChooseTarget = %+v, %v; want the video window", got, found)
	}

	sizes := []overlay.Candidate{
		{Handle: 1, Visible: true, Width: 100, Height: 100},
		{Handle: 2, Visible: true, Width: 1920, Height: 1080},
		{Handle: 3, Visible: true, Width: 1000, Height: 1000},
		{Handle: 4, Visible: false, Width: 3840, Height: 2160},
	}
	if got, _ := overlay.ChooseTarget(sizes); got.Handle != 2 {
		t.Fatalf("ChooseTarget = %d, want 2 (largest visible)", got.Handle)
	}
}

func TestChooseTargetTiesKeepTheFirst(t *testing.T) {
	tie := []overlay.Candidate{
		{Handle: 7, Visible: true, Width: 200, Height: 100},
		{Handle: 8, Visible: true, Width: 100, Height: 200},
	}
	if got, _ := overlay.ChooseTarget(tie); got.Handle != 7 {
		t.Fatalf("ChooseTarget = %d, want 7", got.Handle)
	}
}

func TestChooseTargetFindsNothingUsable(t *testing.T) {
	for name, candidates := range map[string][]overlay.Candidate{
		"vazio":            nil,
		"todas ocultas":    {{Handle: 1, Width: 100, Height: 100}},
		"tamanho zero":     {{Handle: 1, Visible: true}, {Handle: 2, Visible: true, Width: 100}},
		"tamanho negativo": {{Handle: 1, Visible: true, Width: -100, Height: -100}},
	} {
		if got, found := overlay.ChooseTarget(candidates); found {
			t.Errorf("%s: ChooseTarget = %+v, want nothing", name, got)
		}
	}
}
