package session_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func drained(subtitles *session.Subtitles) bool {
	select {
	case <-subtitles.Changes():
		return true
	default:
		return false
	}
}

func TestSyncMovesEachLineOnItsOwn(t *testing.T) {
	subtitles := twoLines(t)
	clock := newFakeTime()
	sync := session.NewSync(subtitles, subtitle.DefaultNudgeSteps(), clock.Now)

	if offset, ok := sync.Nudge(session.LinePrimary, false, false); !ok || offset != subtitle.Offset(-50*time.Millisecond) {
		t.Fatalf("primary earlier = %v, %v; want -50ms", offset, ok)
	}
	if !drained(subtitles) {
		t.Fatal("a nudge did not signal the change, so the screen would wait")
	}
	clock.Advance(300 * time.Millisecond)
	sync.Nudge(session.LineTranslation, true, false)
	clock.Advance(300 * time.Millisecond)
	if offset, _ := sync.Nudge(session.LineTranslation, true, false); offset != subtitle.Offset(100*time.Millisecond) {
		t.Fatalf("translation later twice = %v, want +100ms", offset)
	}
	if got := subtitles.Offset(session.LinePrimary); got != subtitle.Offset(-50*time.Millisecond) {
		t.Fatalf("moving the translation changed the primary to %v", got)
	}
	// With the offset applied the primary's cue "a" (1.0 to 3.0 s) shows 50 ms earlier.
	if visible := subtitles.At(ms(960)); len(visible.Primary) != 1 || visible.Primary[0].ID != "a" {
		t.Fatalf("at 0.96 s the earlier primary shows %v, want [a]", ids(visible.Primary))
	}
}

func TestSyncSpeedsUpWhileHeld(t *testing.T) {
	subtitles := twoLines(t)
	clock := newFakeTime()
	sync := session.NewSync(subtitles, subtitle.NudgeSteps{Step: 100 * time.Millisecond, FastStep: time.Second}, clock.Now)

	sync.Nudge(session.LinePrimary, true, false) // +0.1, held from now
	for range 9 {
		// Keyboard repeats every 100 ms: 0.1 to 0.9 s held, still the small step.
		clock.Advance(100 * time.Millisecond)
		sync.Nudge(session.LinePrimary, true, true)
	}
	if got := subtitles.Offset(session.LinePrimary); got != subtitle.Offset(time.Second) {
		t.Fatalf("after 0.9 s held: %v, want 10 small steps = 1s", got)
	}
	clock.Advance(100 * time.Millisecond) // held for exactly 1 s: fast step
	sync.Nudge(session.LinePrimary, true, true)
	if got := subtitles.Offset(session.LinePrimary); got != subtitle.Offset(2*time.Second) {
		t.Fatalf("after 1 s held: %v, want 1s + fast 1s = 2s", got)
	}
	// A new press (not a repeat) starts over with the small step.
	clock.Advance(100 * time.Millisecond)
	sync.Nudge(session.LinePrimary, false, false)
	if got := subtitles.Offset(session.LinePrimary); got != subtitle.Offset(1900*time.Millisecond) {
		t.Fatalf("new press earlier: %v, want 2s - 0.1s", got)
	}
}

func TestSyncOnAMissingLineDoesNothing(t *testing.T) {
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, cue("a", 1_000, 2_000))}, nil)
	sync := session.NewSync(subtitles, subtitle.DefaultNudgeSteps(), newFakeTime().Now)
	if _, ok := sync.Nudge(session.LineTranslation, true, false); ok {
		t.Fatal("nudged a line without subtitles")
	}
	if _, ok := sync.Nudge(session.Line(7), true, false); ok {
		t.Fatal("nudged an unknown line")
	}
	if drained(subtitles) {
		t.Fatal("signalled a change that did not happen")
	}
}

func TestSyncResetAndLimits(t *testing.T) {
	subtitles := twoLines(t)
	sync := session.NewSync(subtitles, subtitle.NudgeSteps{Step: time.Second, FastStep: 5 * time.Second}, newFakeTime().Now)
	subtitles.SetOffset(session.LinePrimary, subtitle.NewOffset(subtitle.MaxOffset-500*time.Millisecond))
	if offset, _ := sync.Nudge(session.LinePrimary, true, false); offset != subtitle.Offset(subtitle.MaxOffset) {
		t.Fatalf("past the limit: %v, want it held at %v", offset, subtitle.MaxOffset)
	}
	sync.Reset(session.LinePrimary)
	if got := subtitles.Offset(session.LinePrimary); got != 0 {
		t.Fatalf("after reset: %v", got)
	}
}
