package study_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

func TestRevealedTogglesOnPressesAndHoldsWhilePressed(t *testing.T) {
	cases := []struct {
		behavior         study.RevealBehavior
		visible, pressed bool
		want             bool
	}{
		{study.RevealToggle, false, true, true},
		{study.RevealToggle, true, true, false},
		// The release changes nothing when toggling.
		{study.RevealToggle, true, false, true},
		{study.RevealToggle, false, false, false},
		{study.RevealHold, false, true, true},
		{study.RevealHold, true, true, true},
		{study.RevealHold, true, false, false},
		{study.RevealHold, false, false, false},
	}
	for _, c := range cases {
		if got := c.behavior.Revealed(c.visible, c.pressed); got != c.want {
			t.Errorf("%s.Revealed(visible %v, pressed %v) = %v, want %v", c.behavior, c.visible, c.pressed, got, c.want)
		}
	}
}

func TestTogglingTwiceComesBack(t *testing.T) {
	visible := false
	for press := 1; press <= 4; press++ {
		visible = study.RevealToggle.Revealed(visible, true)
		visible = study.RevealToggle.Revealed(visible, false)
		if visible != (press%2 == 1) {
			t.Fatalf("after %d presses visible = %v", press, visible)
		}
	}
}
