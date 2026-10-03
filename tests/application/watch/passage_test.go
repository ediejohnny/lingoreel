package watch_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
)

// markingRig is a session that tells its passage marks on the video as
// "start 20s", "no start" and "too short 40s-40.5s".
func markingRig(t *testing.T) (*rig, *watch.Session) {
	t.Helper()
	r := newRig(t)
	r.ports.MarkNotice = func(event watch.MarkEvent, segment study.Segment) string {
		switch event {
		case watch.MarkStarted:
			return fmt.Sprintf("start %v", segment.Start)
		case watch.MarkNoStart:
			return "no start"
		case watch.MarkTooShort:
			return fmt.Sprintf("too short %v-%v", segment.Start, segment.End)
		}
		return "?"
	}
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 20 * time.Second, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	eventually(t, "the first sample", func() bool { return s.State().Duration == time.Minute })
	return r, s
}

// exact turns off the fitting to the lines, for marks checked to the
// millisecond: fitting adds a lead-in before the start.
func exact(t *testing.T, s *watch.Session) {
	t.Helper()
	if err := s.SetPassageOptions(study.PracticePlan{Mode: study.EndPause}, false); err != nil {
		t.Fatal(err)
	}
}

func lastNotice(r *rig) string {
	_, _, notices, _ := r.display.last()
	if len(notices) == 0 {
		return ""
	}
	return notices[len(notices)-1]
}

// Ctrl+F9 marks the start, Ctrl+F10 the end: the passage starts at once and
// the listener hears it, to save it. An end without a start, or too soon
// after it, is told on the video and starts nothing.
func TestMarkingAPassageStudiesItAndTellsTheListener(t *testing.T) {
	r, s := markingRig(t)
	exact(t, s)

	if err := s.Run(shortcut.PassageMarkEnd, false); err != nil {
		t.Fatalf("an end without a start is told, not an error: %v", err)
	}
	if lastNotice(r) != "no start" || s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatalf("end without a start: notice %q, active %v, marked %v", lastNotice(r), s.Passage().Active, r.listener.passages())
	}

	at(t, r, s, 20*time.Second)
	if err := s.Run(shortcut.PassageMarkStart, false); err != nil {
		t.Fatal(err)
	}
	if start, set := s.MarkedStart(); !set || start != 20*time.Second || lastNotice(r) != "start 20s" {
		t.Fatalf("after the start mark: %v %v, notice %q", start, set, lastNotice(r))
	}
	if s.Passage().Active {
		t.Fatal("marking the start began a passage")
	}

	at(t, r, s, 35*time.Second)
	if err := s.Run(shortcut.PassageMarkEnd, false); err != nil {
		t.Fatal(err)
	}
	status := s.Passage()
	want := study.Segment{Start: 20 * time.Second, End: 35 * time.Second}
	if !status.Active || status.Segment != want {
		t.Fatalf("after the end mark: %+v, want studying %+v", status, want)
	}
	marked := r.listener.passages()
	if len(marked) != 1 || marked[0].segment != want || marked[0].plan.Mode != study.EndPause {
		t.Fatalf("the listener heard %+v", marked)
	}
	if _, set := s.MarkedStart(); set {
		t.Fatal("the start stayed marked after the passage was closed")
	}
	// The passage starts where it begins and plays its first lap.
	if calls := r.running.took(); !slices.Contains(calls, "seek 20s") || !slices.Contains(calls, "play") {
		t.Fatalf("the player was asked %q", calls)
	}
}

func TestATooShortPassageKeepsItsStart(t *testing.T) {
	r, s := markingRig(t)
	exact(t, s)
	at(t, r, s, 40*time.Second)
	s.MarkPassageStart()
	at(t, r, s, 40*time.Second+500*time.Millisecond)
	_, err := s.MarkPassageEnd()
	if !errors.Is(err, study.ErrInvalidSegment) {
		t.Fatalf("MarkPassageEnd half a second later = %v, want ErrInvalidSegment", err)
	}
	if lastNotice(r) != "too short 40s-40.5s" {
		t.Fatalf("notice %q", lastNotice(r))
	}
	if start, set := s.MarkedStart(); !set || start != 40*time.Second {
		t.Fatalf("the start went away: %v %v", start, set)
	}
	if s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatal("a too short passage started or was told")
	}
	// Further on, the same start closes a good passage.
	at(t, r, s, 45*time.Second)
	if segment, err := s.MarkPassageEnd(); err != nil || segment != (study.Segment{Start: 40 * time.Second, End: 45 * time.Second}) {
		t.Fatalf("second try = %+v, %v", segment, err)
	}
}

// Marks may come in either order on the clock: an end before the start
// swaps them.
func TestAnEndBeforeTheStartSwapsThem(t *testing.T) {
	r, s := markingRig(t)
	exact(t, s)
	at(t, r, s, 50*time.Second)
	s.MarkPassageStart()
	at(t, r, s, 44*time.Second)
	segment, err := s.MarkPassageEnd()
	if err != nil || segment != (study.Segment{Start: 44 * time.Second, End: 50 * time.Second}) {
		t.Fatalf("MarkPassageEnd before the start = %+v, %v", segment, err)
	}
}

// By default the marked passage is fitted to the lines, like the panel's
// "Ajustar às falas"; the options last picked (or set) apply to the next.
func TestMarkedPassagesFollowThePassageOptions(t *testing.T) {
	r, s := markingRig(t)
	if plan, snap := s.PassageOptions(); plan.Mode != study.EndPause || !snap {
		t.Fatalf("default options = %+v %v, want pausing at the end, fitted to the lines", plan, snap)
	}
	// The line on top is spoken from 1 s to 3 s: a start inside it moves back
	// to its start, half a second earlier.
	at(t, r, s, 1500*time.Millisecond)
	s.MarkPassageStart()
	at(t, r, s, 10*time.Second)
	if segment, err := s.MarkPassageEnd(); err != nil || segment != (study.Segment{Start: 500 * time.Millisecond, End: 10 * time.Second}) {
		t.Fatalf("fitted passage = %+v, %v", segment, err)
	}

	loop := study.PracticePlan{Mode: study.EndLoop, Gap: time.Second, Goal: 3}
	if err := s.SetPassageOptions(loop, false); err != nil {
		t.Fatal(err)
	}
	at(t, r, s, 1500*time.Millisecond)
	s.MarkPassageStart()
	at(t, r, s, 10*time.Second)
	segment, err := s.MarkPassageEnd()
	if err != nil || segment != (study.Segment{Start: 1500 * time.Millisecond, End: 10 * time.Second}) {
		t.Fatalf("passage without fitting = %+v, %v", segment, err)
	}
	marked := r.listener.passages()
	if last := marked[len(marked)-1]; last.plan != loop || s.Passage().Plan != loop {
		t.Fatalf("plan told %+v, studied %+v, want %+v", last.plan, s.Passage().Plan, loop)
	}
	if err := s.SetPassageOptions(study.PracticePlan{Mode: study.EndLoop, Gap: time.Hour}, true); err == nil {
		t.Fatal("an invalid plan was taken")
	}
	// Studying a passage from the panel makes its options the next ones.
	if err := s.StudyPassage(study.Segment{Start: 30 * time.Second, End: 40 * time.Second}, study.PracticePlan{Mode: study.EndPause, Ladder: true}, true); err != nil {
		t.Fatal(err)
	}
	if plan, snap := s.PassageOptions(); !plan.Ladder || plan.Mode != study.EndPause || !snap {
		t.Fatalf("options after StudyPassage = %+v %v", plan, snap)
	}
}

// The system-wide keys reach the marks; a held Ctrl+F10 closes one passage,
// not one per repeat of the key.
func TestTheGlobalKeysMarkOncePerPress(t *testing.T) {
	r, s := markingRig(t)
	exact(t, s)
	commands := watch.GlobalCommands()
	start, end := slices.Index(commands, shortcut.PassageMarkStart), slices.Index(commands, shortcut.PassageMarkEnd)
	if start < 0 || end < 0 {
		t.Fatalf("the passage marks are not global commands: %v", commands)
	}
	at(t, r, s, 20*time.Second)
	r.shortcuts.presses <- watch.Press{Index: start}
	eventually(t, "the start marked by its key", func() bool { _, set := s.MarkedStart(); return set })
	at(t, r, s, 30*time.Second)
	// Held: only repeats arrive after the first press; none of them counts.
	r.shortcuts.presses <- watch.Press{Index: end, Repeat: true}
	r.shortcuts.presses <- watch.Press{Index: end, Repeat: true}
	time.Sleep(100 * time.Millisecond)
	if s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatal("a held key's repeats closed a passage")
	}
	r.shortcuts.presses <- watch.Press{Index: end}
	eventually(t, "the passage closed by its key", func() bool { return len(r.listener.passages()) == 1 })
	r.shortcuts.presses <- watch.Press{Index: end, Repeat: true}
	time.Sleep(100 * time.Millisecond)
	if got := len(r.listener.passages()); got != 1 {
		t.Fatalf("%d passages told for one press", got)
	}
}

// A session may open studying a saved passage: the player opens where it
// starts, and the first lap begins with its plan.
func TestARequestedPassageIsStudiedFromTheStart(t *testing.T) {
	r := newRig(t)
	plan := study.PracticePlan{Mode: study.EndLoop, Gap: time.Second, Goal: 3}
	r.request.Passage = &watch.PassageStart{Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second}, Plan: plan}
	s := r.start(t)
	if got := r.plugin.requests[0].Start; got != 10*time.Second {
		t.Fatalf("the player opened at %v, want the passage's start", got)
	}
	status := s.Passage()
	if !status.Active || status.Segment != r.request.Passage.Segment || status.Plan != plan {
		t.Fatalf("passage = %+v, want studying %+v with %+v", status, r.request.Passage.Segment, plan)
	}

	// A plan the rules refuse starts nothing.
	bad := newRig(t)
	bad.request.Passage = &watch.PassageStart{Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second},
		Plan: study.PracticePlan{Mode: study.EndLoop, Goal: 1000}}
	if _, err := watch.Start(t.Context(), bad.ports, bad.request, bad.listener, nil); err == nil {
		t.Fatal("a session started with an invalid passage plan")
	}
	if len(bad.plugin.requests) != 0 {
		t.Fatal("a player was launched for an invalid passage plan")
	}
}
