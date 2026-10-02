package study_test

import (
	"math"
	"testing"
	"time"

	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

func s(seconds float64) time.Duration { return time.Duration(seconds * float64(time.Second)) }

func line(id string, start, end float64, text string) subtitle.Event {
	return subtitle.Event{ID: id, Start: s(start), End: s(end), OriginalText: text}
}

// The scene (seconds):
//
//	e1  2–4    silence before it: 2 (from the start of the video)
//	e2  20–23  silence 16
//	e3  24–26  silence 1
//	e4  25–40  overlaps e3: silence -1
//	e5  45–47  silence 5 (after e4 ends at 40)
func scene(t *testing.T) *subtitle.Timeline {
	t.Helper()
	timeline, err := subtitle.NewTimeline([]subtitle.Event{
		line("e1", 2, 4, "Hello there."), line("e2", 20, 23, "Second line."),
		line("e3", 24, 26, "Quick reply."), line("e4", 25, 40, "Overlap."), line("e5", 45, 47, "Late line."),
	})
	if err != nil {
		t.Fatal(err)
	}
	return timeline
}

var defaults = study.DefaultCueRules() // 10 s gap, 5 s lead

func TestUpcomingCueAfterALongSilence(t *testing.T) {
	timeline := scene(t)
	// e2: 16 s of silence, so its preview starts 5 s before it, at 15 s.
	cue, ok := study.UpcomingCue(timeline, 0, s(15), defaults)
	if !ok || cue.Event.ID != "e2" || cue.Start != s(20) || cue.Lead != s(5) || cue.Remaining != s(5) || cue.Progress() != 1 {
		t.Fatalf("at 15 s: %+v %v", cue, ok)
	}
	if _, ok := study.UpcomingCue(timeline, 0, s(15)-1, defaults); ok {
		t.Fatal("preview 1 ns before its lead")
	}
	cue, _ = study.UpcomingCue(timeline, 0, s(19), defaults)
	if cue.Remaining != s(1) || math.Abs(cue.Progress()-0.2) > 1e-9 {
		t.Fatalf("at 19 s: remaining %v progress %v", cue.Remaining, cue.Progress())
	}
	// At 20 s e2 is spoken: the next line (e3) comes after only 1 s.
	if _, ok := study.UpcomingCue(timeline, 0, s(20), defaults); ok {
		t.Fatal("a preview while nothing qualifies")
	}
	// The first line counts its silence from the start of the video: 2 s.
	if _, ok := study.UpcomingCue(timeline, 0, s(1), defaults); ok {
		t.Fatal("the first line had a preview after 2 s of silence")
	}
}

func TestMinimumGapBoundary(t *testing.T) {
	timeline := scene(t)
	exact := study.CueRules{MinGap: s(16), Lead: s(5)}
	if _, ok := study.UpcomingCue(timeline, 0, s(16), exact); !ok {
		t.Fatal("a silence exactly as long as the minimum gave no preview")
	}
	above := study.CueRules{MinGap: s(16) + 1, Lead: s(5)}
	if _, ok := study.UpcomingCue(timeline, 0, s(16), above); ok {
		t.Fatal("a silence 1 ns shorter than the minimum gave a preview")
	}
}

func TestLeadNeverCoversTheLineBefore(t *testing.T) {
	timeline := scene(t)
	// e5 after 5 s of silence with a 10 s lead: the preview starts when e4
	// ends (40 s), not before.
	rules := study.CueRules{MinGap: s(3), Lead: s(10)}
	cue, ok := study.UpcomingCue(timeline, 0, s(40), rules)
	if !ok || cue.Event.ID != "e5" || cue.Lead != s(5) {
		t.Fatalf("at 40 s: %+v %v", cue, ok)
	}
	if _, ok := study.UpcomingCue(timeline, 0, s(39.9), rules); ok {
		t.Fatal("the preview started while e4 was on screen")
	}
	// Overlapping lines (e4 starts inside e3) never get a preview.
	if cue, ok := study.UpcomingCue(timeline, 0, s(24.5), study.CueRules{MinGap: s(3), Lead: s(2)}); ok {
		t.Fatalf("an overlapping line got a preview: %+v", cue)
	}
}

func TestUpcomingCueFollowsTheOffset(t *testing.T) {
	timeline := scene(t)
	// +2 s shows the track later: e2 is spoken at 22 s, its preview at 17 s.
	offset := subtitle.Offset(s(2))
	cue, ok := study.UpcomingCue(timeline, offset, s(17), defaults)
	if !ok || cue.Start != s(22) || cue.Remaining != s(5) {
		t.Fatalf("with +2 s at 17 s: %+v %v", cue, ok)
	}
	if _, ok := study.UpcomingCue(timeline, offset, s(16.9), defaults); ok {
		t.Fatal("the offset was ignored")
	}
	if next, ok := study.NextCueStartAfter(timeline, offset, 0, defaults); !ok || next != s(17) {
		t.Fatalf("next preview with +2 s = %v %v, want 17 s", next, ok)
	}
}

func TestNextCueStartAfter(t *testing.T) {
	timeline := scene(t)
	if next, ok := study.NextCueStartAfter(timeline, 0, 0, defaults); !ok || next != s(15) {
		t.Fatalf("from 0: %v %v", next, ok)
	}
	// From the moment e2's preview began, no later line qualifies.
	if next, ok := study.NextCueStartAfter(timeline, 0, s(15), defaults); ok {
		t.Fatalf("from 15 s: %v, want none", next)
	}
	if _, ok := study.NextCueStartAfter(nil, 0, 0, defaults); ok {
		t.Fatal("a missing timeline has previews")
	}
}

func TestCountdownIsInRealTime(t *testing.T) {
	cue := study.Cue{Remaining: s(1), Lead: s(5)}
	for speed, want := range map[float64]int{1: 1, 0.75: 2, 2: 1, 0: 0, -1: 0} {
		if got := cue.Countdown(speed); got != want {
			t.Errorf("Countdown(%v) = %d, want %d", speed, got, want)
		}
	}
	if got := (study.Cue{Remaining: s(2.5), Lead: s(5)}).Countdown(1); got != 3 {
		t.Errorf("2.5 s left counts %d, want 3", got)
	}
	if got := (study.Cue{Remaining: 0, Lead: s(5)}).Countdown(1); got != 0 {
		t.Errorf("at the line the countdown shows %d", got)
	}
	if got := (study.Cue{Remaining: s(1)}).Progress(); got != 0 {
		t.Errorf("progress with no lead = %v", got)
	}
}

func TestReplayTarget(t *testing.T) {
	timeline := scene(t)
	cases := []struct {
		name     string
		position float64
		offset   subtitle.Offset
		want     float64
		ok       bool
	}{
		{"inside a line with a preview: its preview start", 21, 0, 15, true},
		{"during the preview: the same", 17, 0, 15, true},
		{"after a short silence: 1 s before (silence 1 s)", 24.5, 0, 23, true},
		{"overlapping line: its own start", 25.5, 0, 25, true},
		{"before any line", 1, 0, 0, false},
		{"never before the video", 2.5, subtitle.Offset(-s(1.5)), 0, true},
	}
	for _, c := range cases {
		got, ok := study.ReplayTarget(timeline, c.offset, s(c.position), defaults)
		if ok != c.ok || (ok && got != s(c.want)) {
			t.Errorf("%s: %v %v, want %v %v", c.name, got, ok, s(c.want), c.ok)
		}
	}
}

func TestSpokenFraction(t *testing.T) {
	// "ab cd" over 10 s is spoken in the first 8.5 s. Weights: letters 1,
	// the space 0.5: 4.5 in all. Half the speaking time (4.25 s) reaches
	// weight 2.25: past "ab" (2), inside the space, so 2 of 4 letters.
	event := line("x", 0, 10, "ab cd")
	cases := map[float64]float64{0: 0, -1: 0, 4.25: 0.5, 8.5: 1, 10: 1, 20: 1}
	for at, want := range cases {
		if got := study.SpokenFraction(event, s(at)); math.Abs(got-want) > 1e-9 {
			t.Errorf("SpokenFraction at %v s = %v, want %v", at, got, want)
		}
	}
	// A comma holds the fill: "a, b" weighs 1 + 4 + 0.5 + 1 = 6.5, and the
	// comma alone takes 4 of it.
	pause := line("p", 0, 10, "a, b")
	if before, after := study.SpokenFraction(pause, s(8.5*1.2/6.5)), study.SpokenFraction(pause, s(8.5*4.8/6.5)); after-before >= 1.0/3 {
		t.Errorf("the comma did not slow the fill: %v -> %v", before, after)
	}
	if got := study.SpokenFraction(line("z", 5, 5, "x"), s(5)); got != 0 {
		t.Errorf("an empty span gave %v", got)
	}
}

func FuzzSpokenFraction(f *testing.F) {
	f.Add("Hello, world!", int64(1000), int64(5000), int64(2000), int64(10))
	f.Add("…", int64(0), int64(1), int64(0), int64(1))
	f.Fuzz(func(t *testing.T, text string, start, end, at, step int64) {
		event := subtitle.Event{Start: time.Duration(start), End: time.Duration(end), OriginalText: text}
		got := study.SpokenFraction(event, time.Duration(at))
		if math.IsNaN(got) || got < 0 || got > 1 {
			t.Fatalf("SpokenFraction = %v", got)
		}
		if step > 0 && at < math.MaxInt64-step {
			if later := study.SpokenFraction(event, time.Duration(at+step)); later < got-1e-12 {
				t.Fatalf("the fill went back: %v then %v", got, later)
			}
		}
	})
}

func TestCueRulesValidate(t *testing.T) {
	cases := []struct {
		rules study.CueRules
		ok    bool
	}{
		{study.DefaultCueRules(), true},
		{study.CueRules{MinGap: study.MinCueGap, Lead: study.MinCueLead}, true},
		{study.CueRules{MinGap: study.MaxCueGap, Lead: study.MaxCueLead}, true},
		{study.CueRules{MinGap: study.MinCueGap - 1, Lead: study.DefaultLead}, false},
		{study.CueRules{MinGap: study.MaxCueGap + 1, Lead: study.DefaultLead}, false},
		{study.CueRules{MinGap: study.DefaultCueGap, Lead: study.MinCueLead - 1}, false},
		{study.CueRules{MinGap: study.DefaultCueGap, Lead: study.MaxCueLead + 1}, false},
	}
	for _, c := range cases {
		if err := c.rules.Validate(); (err == nil) != c.ok {
			t.Errorf("Validate(%+v) = %v, want ok %v", c.rules, err, c.ok)
		}
	}
	if !study.IndicatorBar.Valid() || !study.IndicatorCountdown.Valid() || study.Indicator("").Valid() {
		t.Error("indicator validity")
	}
}
