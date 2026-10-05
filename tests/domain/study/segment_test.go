package study_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestParseClock(t *testing.T) {
	t.Parallel()
	good := map[string]time.Duration{
		"1:10:23": s(4223), "10:23": s(623), "23": s(23), "10:23,5": s(623.5), "10:23.25": s(623.25),
		" 0:00 ": 0, "0": 0, "100:00:00": 100 * time.Hour, "59:59": s(3599),
	}
	for text, want := range good {
		if got, err := study.ParseClock(text); err != nil || got != want {
			t.Errorf("ParseClock(%q) = %v, %v; want %v", text, got, err, want)
		}
	}
	for _, text := range []string{"", "  ", "1:99:00", "1:10:60", "a", "-5", "+5", "1::2", "1:2:3:4", "23,5,1", "1:2,5:3", "101:00:00", "1e3", "NaN", "Inf"} {
		if got, err := study.ParseClock(text); !errors.Is(err, study.ErrInvalidSegment) {
			t.Errorf("ParseClock(%q) = %v, %v; want ErrInvalidSegment", text, got, err)
		}
	}
}

func TestSegmentValidate(t *testing.T) {
	t.Parallel()
	minute := time.Minute
	cases := []struct {
		segment  study.Segment
		duration time.Duration
		ok       bool
	}{
		{study.Segment{Start: s(10), End: s(11)}, minute, true}, // exactly 1 s
		{study.Segment{Start: s(10), End: s(11) - time.Millisecond}, minute, false},
		{study.Segment{Start: s(10), End: s(10)}, minute, false},
		{study.Segment{Start: s(10), End: s(5)}, minute, false},
		{study.Segment{Start: -1, End: s(5)}, minute, false},
		{study.Segment{Start: s(50), End: minute}, minute, true},
		{study.Segment{Start: s(50), End: minute + 1}, minute, false},
		{study.Segment{Start: s(50), End: 2 * minute}, 0, true}, // unknown duration
	}
	for _, c := range cases {
		if err := c.segment.Validate(c.duration); (err == nil) != c.ok {
			t.Errorf("Validate(%+v, %v) = %v, want ok %v", c.segment, c.duration, err, c.ok)
		}
	}
	seg := study.Segment{Start: s(10), End: s(20)}
	if !seg.Contains(s(10)) || !seg.Contains(s(20)) || seg.Contains(s(20)+1) || seg.Contains(s(10)-1) {
		t.Fatal("Contains must include both ends only")
	}
}

func TestSnapToLines(t *testing.T) {
	t.Parallel()
	timeline := scene(t)
	cases := []struct {
		name       string
		in         study.Segment
		offset     subtitle.Offset
		start, end float64
	}{
		// Inside e2 (20–23): back to 20, then 0.5 s earlier; the end falls in
		// a silence and stays.
		{"start inside a line", study.Segment{Start: s(21), End: s(23.5)}, 0, 19.5, 23.5},
		// Inside e3 (24–26): back to 24; the lead-in stops at e2's end (23)?
		// No: 24-0.5 = 23.5 is after 23. The end in e3 and e4 goes to 40.
		{"end inside overlapping lines", study.Segment{Start: s(24.2), End: s(25.5)}, 0, 23.5, 40},
		// Just before a line (19.8, e2 at 20): the line's lead-in.
		{"start just before a line", study.Segment{Start: s(19.8), End: s(23)}, 0, 19.5, 23},
		// In a long silence: 0.5 s earlier, still after e1's end (4).
		{"start in a silence", study.Segment{Start: s(10), End: s(12)}, 0, 9.5, 12},
		// +1 s offset: e2 shows 21–24.
		{"offset", study.Segment{Start: s(22), End: s(23)}, subtitle.Offset(s(1)), 20.5, 24},
	}
	for _, c := range cases {
		got := study.SnapToLines(c.in, timeline, c.offset)
		if got.Start != s(c.start) || got.End != s(c.end) {
			t.Errorf("%s: %+v, want %v–%v", c.name, got, s(c.start), s(c.end))
		}
	}
	// The lead-in never reaches into the line before: x starts 0.2 s after
	// e2 ends.
	close, err := subtitle.NewTimeline([]subtitle.Event{line("a", 20, 23, "A."), line("x", 23.2, 24, "X.")})
	if err != nil {
		t.Fatal(err)
	}
	if got := study.SnapToLines(study.Segment{Start: s(23.5), End: s(24)}, close, 0); got.Start != s(23) {
		t.Fatalf("lead-in into the line before: start %v, want 23 s", got.Start)
	}
	if got := study.SnapToLines(study.Segment{Start: s(1), End: s(3)}, nil, 0); got != (study.Segment{Start: s(1), End: s(3)}) {
		t.Fatalf("no timeline changed the passage: %+v", got)
	}
}

func TestSegmentForLine(t *testing.T) {
	t.Parallel()
	got := study.SegmentForLine(line("e2", 20, 23, "Second line."), scene(t), 0)
	if got != (study.Segment{Start: s(19.5), End: s(23)}) {
		t.Fatalf("SegmentForLine = %+v", got)
	}
}

func TestPracticePlanLaps(t *testing.T) {
	t.Parallel()
	full := study.PracticePlan{Mode: study.EndPause, Progressive: true, Ladder: true}
	want := []study.Lap{
		{Speed: 0.75, ShowPrimary: true, ShowTranslation: true},
		{Speed: 0.9, ShowPrimary: true, ShowTranslation: false},
		{Speed: 1, ShowPrimary: false, ShowTranslation: false},
		{Speed: 1, ShowPrimary: false, ShowTranslation: false},
	}
	for i, lap := range want {
		if got := full.Lap(i + 1); got != lap {
			t.Errorf("lap %d = %+v, want %+v", i+1, got, lap)
		}
	}
	if got := full.Lap(0); got != want[0] {
		t.Errorf("lap 0 = %+v, want the first lap", got)
	}
	plain := study.PracticePlan{Mode: study.EndLoop}
	if got := plain.Lap(5); got != (study.Lap{ShowPrimary: true, ShowTranslation: true}) || got.Speed != playback.Speed(0) {
		t.Errorf("plain lap = %+v", got)
	}
	goal := study.PracticePlan{Mode: study.EndPause, Goal: 3}
	if goal.Done(2) || !goal.Done(3) || (study.PracticePlan{}).Done(1000) {
		t.Error("Done must follow the goal, and no goal never ends")
	}
}

func TestPracticePlanValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		plan study.PracticePlan
		ok   bool
	}{
		{study.PracticePlan{Mode: study.EndPause}, true},
		{study.PracticePlan{Mode: study.EndLoop, Gap: study.MaxLoopGap, Goal: study.MaxGoal}, true},
		{study.PracticePlan{}, false},
		{study.PracticePlan{Mode: "stop"}, false},
		{study.PracticePlan{Mode: study.EndLoop, Gap: -1}, false},
		{study.PracticePlan{Mode: study.EndLoop, Gap: study.MaxLoopGap + 1}, false},
		{study.PracticePlan{Mode: study.EndLoop, Goal: -1}, false},
		{study.PracticePlan{Mode: study.EndLoop, Goal: study.MaxGoal + 1}, false},
	}
	for _, c := range cases {
		if err := c.plan.Validate(); (err == nil) != c.ok {
			t.Errorf("Validate(%+v) = %v, want ok %v", c.plan, err, c.ok)
		}
	}
}

// Fitting to the lines may stretch a passage past the video, to a line of
// the credits; FitTo brings it back inside, and an unknown duration (zero)
// leaves the end alone.
func TestSegmentFitTo(t *testing.T) {
	t.Parallel()
	s := func(start, end time.Duration) study.Segment { return study.Segment{Start: start, End: end} }
	cases := []struct {
		name     string
		segment  study.Segment
		duration time.Duration
		want     study.Segment
	}{
		{"inside", s(10*time.Second, 20*time.Second), time.Minute, s(10*time.Second, 20*time.Second)},
		{"past the end", s(50*time.Second, 70*time.Second), time.Minute, s(50*time.Second, time.Minute)},
		{"ending at the end", s(50*time.Second, time.Minute), time.Minute, s(50*time.Second, time.Minute)},
		{"before zero", s(-time.Second, 5*time.Second), time.Minute, s(0, 5*time.Second)},
		{"unknown duration", s(50*time.Second, 70*time.Second), 0, s(50*time.Second, 70*time.Second)},
	}
	for _, c := range cases {
		if got := c.segment.FitTo(c.duration); got != c.want {
			t.Errorf("%s: FitTo = %+v, want %+v", c.name, got, c.want)
		}
	}
	// Fitted, a passage that ran past the end validates against the video.
	if err := s(55*time.Second, 61*time.Second).FitTo(time.Minute).Validate(time.Minute); err != nil {
		t.Fatalf("a fitted passage is refused: %v", err)
	}
}
