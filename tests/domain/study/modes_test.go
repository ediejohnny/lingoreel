package study_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestEachModeStartsAndHidesItsOwnWay(t *testing.T) {
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
}

// What the translation key does, written out: in dual mode it toggles
// whatever the behavior says, on demand the behavior decides (a held key
// hides on release), and when always shown nothing changes it.
func TestTheTranslationKeyInEachMode(t *testing.T) {
	const (
		toggle = study.RevealToggle
		hold   = study.RevealHold
	)
	cases := []struct {
		mode             study.Mode
		behavior         study.RevealBehavior
		visible, pressed bool
		want             bool
	}{
		{study.ModeDual, toggle, false, true, true},
		{study.ModeDual, toggle, true, true, false},
		{study.ModeDual, toggle, true, false, true},
		{study.ModeDual, hold, false, true, true},
		{study.ModeDual, hold, true, true, false},
		{study.ModeDual, hold, true, false, true},
		{study.ModeDual, hold, false, false, false},
		{study.ModeOnDemand, toggle, false, true, true},
		{study.ModeOnDemand, toggle, true, true, false},
		{study.ModeOnDemand, toggle, true, false, true},
		{study.ModeOnDemand, hold, false, true, true},
		{study.ModeOnDemand, hold, true, true, true},
		{study.ModeOnDemand, hold, true, false, false},
		{study.ModeAlwaysTranslated, toggle, true, true, true},
		{study.ModeAlwaysTranslated, hold, true, false, true},
		{study.ModeAlwaysTranslated, hold, false, false, true},
	}
	for _, c := range cases {
		if got := c.mode.Reveal(c.behavior, c.visible, c.pressed); got != c.want {
			t.Errorf("%s.Reveal(%s, visible %v, pressed %v) = %v, want %v", c.mode, c.behavior, c.visible, c.pressed, got, c.want)
		}
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
