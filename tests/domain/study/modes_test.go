package study_test

import (
	"slices"
	"testing"
	"time"

	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

func TestTheModesStartAndHideAsThePRDSays(t *testing.T) {
	cases := []struct {
		mode               study.Mode
		revealed, hideable bool
	}{
		{study.ModeDual, true, true},
		{study.ModeOnDemand, false, true},
		{study.ModeAlwaysTranslated, true, false},
	}
	for _, c := range cases {
		if c.mode.StartsRevealed() != c.revealed || c.mode.Hideable() != c.hideable {
			t.Errorf("%s: starts revealed %v, hideable %v", c.mode, c.mode.StartsRevealed(), c.mode.Hideable())
		}
	}
	if !slices.Equal(study.Modes(), []study.Mode{study.ModeDual, study.ModeOnDemand, study.ModeAlwaysTranslated}) {
		t.Fatalf("Modes() = %v", study.Modes())
	}
	for _, mode := range study.Modes() {
		if !mode.Valid() {
			t.Errorf("%s is not valid", mode)
		}
	}
}

func TestTheTranslationKeyInEachMode(t *testing.T) {
	behaviors := []study.RevealBehavior{study.RevealToggle, study.RevealHold}
	for _, behavior := range behaviors {
		for _, visible := range []bool{false, true} {
			for _, pressed := range []bool{false, true} {
				// Always shown: nothing changes it.
				if !study.ModeAlwaysTranslated.Reveal(behavior, visible, pressed) {
					t.Errorf("always, %s, visible %v, pressed %v: hidden", behavior, visible, pressed)
				}
				// Dual: the key toggles, whatever the behavior says.
				if got, want := study.ModeDual.Reveal(behavior, visible, pressed), study.RevealToggle.Revealed(visible, pressed); got != want {
					t.Errorf("dual, %s, visible %v, pressed %v = %v, want %v", behavior, visible, pressed, got, want)
				}
				// On demand: the behavior decides.
				if got, want := study.ModeOnDemand.Reveal(behavior, visible, pressed), behavior.Revealed(visible, pressed); got != want {
					t.Errorf("on demand, %s, visible %v, pressed %v = %v, want %v", behavior, visible, pressed, got, want)
				}
			}
		}
	}
	// The hold and the toggle differ on demand: a release hides only when held.
	if study.ModeOnDemand.Reveal(study.RevealHold, true, false) || !study.ModeOnDemand.Reveal(study.RevealToggle, true, false) {
		t.Fatal("on demand ignored the behavior")
	}
	// In dual, holding still toggles: the release keeps the hidden line hidden.
	if study.ModeDual.Reveal(study.RevealHold, false, false) {
		t.Fatal("dual showed the translation on a release")
	}
}

func TestPhraseStart(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	timeline, err := subtitle.NewTimeline([]subtitle.Event{
		{ID: "a", Start: ms(10000), End: ms(14000), OriginalText: "A long line."},
		{ID: "b", Start: ms(12000), End: ms(13000), OriginalText: "Inside it."},
		{ID: "c", Start: ms(14200), End: ms(16000), OriginalText: "Right after."},
		{ID: "d", Start: ms(30000), End: ms(32000), OriginalText: "Much later."},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		position time.Duration
		offset   subtitle.Offset
		want     time.Duration
		found    bool
	}{
		{"before the first line", ms(9999), 0, 0, false},
		// At its start exactly, after the 0.5 s lead-in back to the silence.
		{"first line starting", ms(10000), 0, ms(9500), true},
		// Inside b, which overlaps a: the phrase is a, from its start.
		{"overlapping lines", ms(12500), 0, ms(9500), true},
		// c starts 0.2 s after a ends: the lead-in stops at that end.
		{"close to the line before", ms(15000), 0, ms(14000), true},
		// Between lines: the last one spoken, c, with the same lead-in.
		{"in a silence", ms(20000), 0, ms(14000), true},
		{"long after", ms(40000), 0, ms(29500), true},
		// A line 1 s late on the playback clock: everything shifts.
		{"with an offset", ms(31000), subtitle.Offset(time.Second), ms(30500), true},
		{"offset before the first line", ms(10500), subtitle.Offset(time.Second), 0, false},
	}
	for _, c := range cases {
		got, found := study.PhraseStart(timeline, c.offset, c.position)
		if found != c.found || got != c.want {
			t.Errorf("%s: PhraseStart(%v) = %v %v, want %v %v", c.name, c.position, got, found, c.want, c.found)
		}
	}
	if _, found := study.PhraseStart(nil, 0, ms(5000)); found {
		t.Fatal("a missing timeline has a phrase")
	}
}
