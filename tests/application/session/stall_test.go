package session_test

import (
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
)

// repeat feeds the same playing position every 100 ms for a while.
func repeat(clock *session.Clock, now *fakeTime, position, span time.Duration) {
	for elapsed := time.Duration(0); elapsed < span; elapsed += ms(100) {
		now.Advance(ms(100))
		clock.Observe(sample(position, playback.StatusPlaying, 1, now.Now()))
	}
}

// A player that says it plays but reports the same position for 3 s is
// frozen: the clock waits at its position instead of running ahead, and goes
// on when the position moves again.
func TestAFrozenPlayerHoldsTheClock(t *testing.T) {
	for _, precision := range []playback.Precision{playback.PrecisionEstimated, playback.PrecisionExact} {
		now := newFakeTime()
		clock := session.NewClock(precision, now.Now)
		clock.Observe(sample(ms(10_000), playback.StatusPlaying, 1, now.Now()))

		repeat(clock, now, ms(10_000), ms(2_900))
		if clock.Stalled() {
			t.Fatalf("precision %v: stalled before 3 s", precision)
		}
		repeat(clock, now, ms(10_000), ms(200))
		if !clock.Stalled() || clock.Position() != ms(10_000) {
			t.Fatalf("precision %v: after 3 s stalled %v at %v, want held at 10 s", precision, clock.Stalled(), clock.Position())
		}
		now.Advance(time.Minute)
		if got := clock.Position(); got != ms(10_000) {
			t.Fatalf("precision %v: a frozen player's clock ran to %v", precision, got)
		}

		// It moves again: the clock follows and estimates once more.
		clock.Observe(sample(ms(10_100), playback.StatusPlaying, 1, now.Now()))
		now.Advance(ms(500))
		if clock.Stalled() || clock.Position() != ms(10_600) {
			t.Fatalf("precision %v: after moving, stalled %v at %v", precision, clock.Stalled(), clock.Position())
		}
	}
}

// Paused, the same position is no freeze; a session command starts over.
func TestPausesAndCommandsAreNoFreeze(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(10_000), playback.StatusPaused, 1, now.Now()))
	for range 100 {
		now.Advance(ms(100))
		clock.Observe(sample(ms(10_000), playback.StatusPaused, 1, now.Now()))
	}
	if clock.Stalled() {
		t.Fatal("a paused video counted as frozen")
	}

	clock.Observe(sample(ms(10_000), playback.StatusPlaying, 1, now.Now()))
	repeat(clock, now, ms(10_000), ms(3_100))
	if !clock.Stalled() {
		t.Fatal("not stalled")
	}
	clock.SetPosition(ms(20_000))
	if clock.Stalled() {
		t.Fatal("a seek by the session kept the freeze")
	}
	// The status changed to playing at the seek: 3 s of the same report again.
	repeat(clock, now, ms(20_000), ms(2_000))
	if clock.Stalled() {
		t.Fatal("stalled 2 s after the seek")
	}
}
