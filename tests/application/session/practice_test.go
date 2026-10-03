package session_test

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/study"
)

// practiceRig drives a practice by hand: the test sets the clock's state
// and calls Tick, as the runner does on every turn.
type practiceRig struct {
	player   *fakePlayer
	clock    *session.Clock
	controls *session.Controls
	time     *fakeTime
	practice *session.Practice

	mu     sync.Mutex
	events []session.PracticeEvent
	looks  []study.Lap
}

func newPracticeRig(t *testing.T, subtitles *session.Subtitles) *practiceRig {
	t.Helper()
	r := &practiceRig{player: &fakePlayer{capabilities: allCapabilities(playback.PrecisionExact)}, time: newFakeTime()}
	r.clock = session.NewClock(playback.PrecisionExact, r.time.Now)
	r.clock.Observe(playback.Sample{Position: 0, Duration: time.Minute, Speed: 1, Status: playback.StatusPaused, ObservedAt: r.time.Now()})
	r.controls = session.NewControls(r.player, r.clock, 25)
	if subtitles == nil {
		subtitles = karaokeSubtitles(t)
	}
	r.practice = session.NewPractice(r.controls, subtitles, session.PracticeHooks{
		Event: func(event session.PracticeEvent, _ session.PracticeStatus) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.events = append(r.events, event)
		},
		Look: func(lap study.Lap) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.looks = append(r.looks, lap)
		},
	}, r.time.Now)
	return r
}

// at puts the clock at a position, playing or not, and ticks. A second
// passes first: right after a seek the clock ignores the player for a
// moment (its grace), and these samples must count.
func (r *practiceRig) at(position time.Duration, playing bool) {
	r.time.Advance(time.Second)
	status := playback.StatusPaused
	if playing {
		status = playback.StatusPlaying
	}
	r.clock.Observe(playback.Sample{Position: position, Duration: time.Minute, Speed: 1, Status: status, ObservedAt: r.time.Now()})
	r.practice.Tick(r.clock.State())
}

func (r *practiceRig) calls() []string {
	return slices.Clone(r.player.calls)
}

func (r *practiceRig) reset() { r.player.calls = nil }

func (r *practiceRig) lastEvent() session.PracticeEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return 0
	}
	return r.events[len(r.events)-1]
}

var passage = study.Segment{Start: ms(19_500), End: ms(23_000)}

func TestStartGoesToThePassageAndPlays(t *testing.T) {
	r := newPracticeRig(t, nil)
	if err := r.practice.Start(passage, study.PracticePlan{Mode: study.EndPause}); err != nil {
		t.Fatal(err)
	}
	if r.player.seekTarget != passage.Start || !slices.Contains(r.calls(), "play") {
		t.Fatalf("start: calls %v seek %v", r.calls(), r.player.seekTarget)
	}
	status := r.practice.Status()
	if !status.Active || status.Lap != 1 || status.Suspended || r.lastEvent() != session.PracticeLap {
		t.Fatalf("status %+v, last event %v", status, r.lastEvent())
	}
	// Bad passages and plans are refused before anything moves.
	r.reset()
	if err := r.practice.Start(study.Segment{Start: ms(10_000), End: ms(10_500)}, study.PracticePlan{Mode: study.EndPause}); !errors.Is(err, study.ErrInvalidSegment) {
		t.Fatalf("a 0.5 s passage: %v", err)
	}
	if err := r.practice.Start(passage, study.PracticePlan{Mode: "x"}); !errors.Is(err, study.ErrInvalidSegment) {
		t.Fatalf("a bad plan: %v", err)
	}
	if len(r.calls()) != 0 {
		t.Fatalf("a refused start moved the player: %v", r.calls())
	}
}

func TestPauseAtTheEndThenPlayStartsOver(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndPause})
	r.at(ms(22_000), true)
	r.reset()
	r.at(ms(22_999), true)
	if len(r.calls()) != 0 {
		t.Fatalf("acted before the end: %v", r.calls())
	}
	r.at(ms(23_000), true)
	if !slices.Contains(r.calls(), "pause") || !r.practice.Status().AtEnd || r.lastEvent() != session.PracticeAtEnd {
		t.Fatalf("at the end: calls %v status %+v", r.calls(), r.practice.Status())
	}
	// Play (or Space) goes back to the start for lap 2.
	r.reset()
	if err := r.controls.Play(); err != nil {
		t.Fatal(err)
	}
	if r.player.seekTarget != passage.Start || r.practice.Status().Lap != 2 || r.practice.Status().AtEnd {
		t.Fatalf("Play at the end: seek %v status %+v", r.player.seekTarget, r.practice.Status())
	}
	if r.practice.Status().Suspended {
		t.Fatal("going back to the start suspended the passage")
	}
}

func TestLoopWithoutAndWithGap(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndLoop})
	r.reset()
	r.at(ms(23_100), true)
	if r.player.seekTarget != passage.Start || r.practice.Status().Lap != 2 || slices.Contains(r.calls(), "pause") {
		t.Fatalf("loop without gap: calls %v lap %d", r.calls(), r.practice.Status().Lap)
	}

	r = newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndLoop, Gap: 2 * time.Second})
	r.reset()
	r.at(ms(23_000), true)
	if !slices.Contains(r.calls(), "pause") || r.practice.Status().Lap != 1 {
		t.Fatalf("gap: calls %v", r.calls())
	}
	if wake := r.practice.WakeIn(r.clock.State()); wake != 2*time.Second {
		t.Fatalf("WakeIn during the gap = %v, want 2 s", wake)
	}
	r.time.Advance(1999 * time.Millisecond)
	r.practice.Tick(r.clock.State())
	if r.practice.Status().Lap != 1 {
		t.Fatal("the next lap began before the gap was over")
	}
	r.time.Advance(time.Millisecond)
	r.practice.Tick(r.clock.State())
	if r.practice.Status().Lap != 2 || r.player.seekTarget != passage.Start {
		t.Fatalf("after the gap: lap %d seek %v", r.practice.Status().Lap, r.player.seekTarget)
	}
}

func TestGoalEndsThePractice(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndLoop, Goal: 2, Ladder: true})
	r.at(ms(23_000), true) // lap 1 done, lap 2 begins
	r.at(ms(23_000), true) // lap 2 done: goal
	if r.practice.Status().Active || r.lastEvent() != session.PracticeFinished || !slices.Contains(r.calls(), "pause") {
		t.Fatalf("after the goal: status %+v, last event %v, calls %v", r.practice.Status(), r.lastEvent(), r.calls())
	}
	r.mu.Lock()
	last := r.looks[len(r.looks)-1]
	r.mu.Unlock()
	if !last.ShowPrimary || !last.ShowTranslation {
		t.Fatalf("the ladder kept lines hidden after the end: %+v", last)
	}
}

func TestLapsFollowThePlan(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndLoop, Progressive: true, Ladder: true})
	if r.player.speed != 0.75 {
		t.Fatalf("lap 1 speed %v, want 0.75", r.player.speed)
	}
	r.at(ms(23_000), true)
	r.mu.Lock()
	looks := slices.Clone(r.looks)
	r.mu.Unlock()
	if r.player.speed != 0.9 || len(looks) != 2 || looks[1].ShowTranslation || !looks[1].ShowPrimary {
		t.Fatalf("lap 2: speed %v looks %+v", r.player.speed, looks)
	}
}

func TestSeekingOutSuspendsThePassage(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndPause})
	if err := r.controls.SeekTo(ms(40_000)); err != nil {
		t.Fatal(err)
	}
	if !r.practice.Status().Suspended || r.lastEvent() != session.PracticeSuspended {
		t.Fatalf("seek out: %+v", r.practice.Status())
	}
	r.reset()
	r.at(ms(45_000), true)
	if len(r.calls()) != 0 {
		t.Fatalf("a suspended passage still acted: %v", r.calls())
	}
	if err := r.practice.Resume(); err != nil {
		t.Fatal(err)
	}
	if r.practice.Status().Suspended || r.player.seekTarget != passage.Start {
		t.Fatalf("Resume: %+v seek %v", r.practice.Status(), r.player.seekTarget)
	}
	// Seeking back inside also brings the passage back.
	_ = r.controls.SeekTo(ms(40_000))
	_ = r.controls.SeekTo(ms(21_000))
	if r.practice.Status().Suspended {
		t.Fatal("a seek back into the passage left it suspended")
	}
	r.practice.Stop()
	if err := r.practice.Resume(); !errors.Is(err, session.ErrNoPassage) {
		t.Fatalf("Resume without a passage: %v", err)
	}
}

func TestRepeatEachLine(t *testing.T) {
	r := newPracticeRig(t, nil)
	if err := r.practice.SetLineRepeats(2); err != nil {
		t.Fatal(err)
	}
	// B ends at 23 s; 300 ms later it plays again from its preview (15 s).
	r.at(ms(23_299), true)
	if len(r.calls()) != 0 {
		t.Fatalf("repeated before the delay: %v", r.calls())
	}
	for range 2 {
		r.reset()
		r.at(ms(23_300), true)
		if r.player.seekTarget != ms(15_000) {
			t.Fatalf("repeat: seek %v calls %v", r.player.seekTarget, r.calls())
		}
	}
	r.reset()
	r.at(ms(23_300), true)
	if len(r.calls()) != 0 {
		t.Fatal("repeated more than asked")
	}
	// The user's seek starts the count over.
	_ = r.controls.SeekTo(ms(21_000))
	r.reset()
	r.at(ms(23_300), true)
	if r.player.seekTarget != ms(15_000) {
		t.Fatal("a seek of the user did not start the repeats over")
	}
	// A jump far past the line is not a line ending.
	r.reset()
	r.at(ms(30_000), true)
	if len(r.calls()) != 0 {
		t.Fatalf("a position long after the line repeated it: %v", r.calls())
	}
	if err := r.practice.SetLineRepeats(study.MaxRepeats + 1); err == nil {
		t.Fatal("11 repeats accepted")
	}
}

func TestReplayLine(t *testing.T) {
	r := newPracticeRig(t, nil)
	r.at(ms(21_000), false)
	r.reset()
	if err := r.practice.ReplayLine(); err != nil {
		t.Fatal(err)
	}
	// Back to B's preview, and playing: the line is trained by speaking along.
	if r.player.seekTarget != ms(15_000) || !slices.Contains(r.calls(), "play") {
		t.Fatalf("Treinar de novo: seek %v, calls %v; want B's preview at 15 s, playing", r.player.seekTarget, r.calls())
	}
	// Already playing, it only goes back.
	r.at(ms(21_000), true)
	r.reset()
	if err := r.practice.ReplayLine(); err != nil {
		t.Fatal(err)
	}
	if r.player.seekTarget != ms(15_000) || slices.Contains(r.calls(), "play") {
		t.Fatalf("playing: seek %v, calls %v", r.player.seekTarget, r.calls())
	}
	empty := newPracticeRig(t, session.NewSubtitles(nil, nil))
	if err := empty.practice.ReplayLine(); !errors.Is(err, session.ErrNotSupported) {
		t.Fatalf("no line on top: %v", err)
	}
}

// The reported case: a passage paused at its end ("Pausa no fim").
// "Treinar de novo" trains B, the passage's last line, and plays on from it:
// not from the passage's start, and the lap goes on. A line whose preview
// starts before the passage is trained from the passage's start.
func TestReplayLineInAPassagePausedAtItsEnd(t *testing.T) {
	for _, c := range []struct {
		passage study.Segment
		want    time.Duration
	}{
		{study.Segment{Start: ms(10_000), End: ms(30_000)}, ms(15_000)},
		{study.Segment{Start: ms(18_000), End: ms(30_000)}, ms(18_000)},
	} {
		r := newPracticeRig(t, nil)
		if err := r.practice.Start(c.passage, study.PracticePlan{Mode: study.EndPause}); err != nil {
			t.Fatal(err)
		}
		r.at(ms(29_000), true)
		r.at(ms(30_000), true)
		if !r.practice.Status().AtEnd {
			t.Fatalf("%v: not paused at the end", c.passage)
		}
		r.at(ms(30_000), false)
		r.reset()
		if err := r.practice.ReplayLine(); err != nil {
			t.Fatal(err)
		}
		// The last seek is the line's: Play did not start the passage over.
		if r.player.seekTarget != c.want || !slices.Contains(r.calls(), "play") {
			t.Fatalf("%v: seek %v, calls %v; want %v, playing", c.passage, r.player.seekTarget, r.calls(), c.want)
		}
		if status := r.practice.Status(); !status.Active || status.AtEnd || status.Suspended || status.Lap != 1 {
			t.Fatalf("%v: after Treinar de novo %+v", c.passage, status)
		}
	}
}

func TestPracticeWakesForTheEnd(t *testing.T) {
	r := newPracticeRig(t, nil)
	_ = r.practice.Start(passage, study.PracticePlan{Mode: study.EndPause})
	r.at(ms(21_000), true)
	if wake := r.practice.WakeIn(r.clock.State()); wake != 2*time.Second {
		t.Fatalf("WakeIn = %v, want 2 s to the end", wake)
	}
	r.at(ms(21_000), false)
	if wake := r.practice.WakeIn(r.clock.State()); wake != 0 {
		t.Fatalf("paused WakeIn = %v, want none", wake)
	}
}
