package session_test

import (
	"math"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
)

func signalled(clock *session.Clock) bool {
	select {
	case <-clock.Changes():
		return true
	default:
		return false
	}
}

func TestClockBeforeAnySampleIsPausedAtZero(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	now.Advance(time.Hour)

	state := clock.State()
	if state.Position != 0 || state.Status != playback.StatusPaused || state.Speed != playback.NormalSpeed {
		t.Fatalf("initial state = %+v", state)
	}
}

func TestClockExtrapolatesAtTheReportedSpeed(t *testing.T) {
	cases := []struct {
		name   string
		status playback.Status
		speed  playback.Speed
		after  time.Duration
		want   time.Duration
	}{
		{"playing at 1x", playback.StatusPlaying, 1, ms(500), ms(10_500)},
		{"playing at 2x", playback.StatusPlaying, 2, ms(500), ms(11_000)},
		{"playing at 0.5x", playback.StatusPlaying, 0.5, ms(500), ms(10_250)},
		{"paused does not move", playback.StatusPaused, 1, ms(500), ms(10_000)},
		{"stopped does not move", playback.StatusStopped, 1, ms(500), ms(10_000)},
		{"NaN speed counts as 1x", playback.StatusPlaying, playback.Speed(math.NaN()), ms(500), ms(10_500)},
		{"zero speed counts as 1x", playback.StatusPlaying, 0, ms(500), ms(10_500)},
		{"infinite speed counts as 1x", playback.StatusPlaying, playback.Speed(math.Inf(1)), ms(500), ms(10_500)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := newFakeTime()
			clock := session.NewClock(playback.PrecisionExact, now.Now)
			clock.Observe(sample(ms(10_000), tc.status, tc.speed, now.Now()))
			now.Advance(tc.after)
			if got := clock.Position(); got != tc.want {
				t.Fatalf("Position() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClockStaysInsideTheVideo(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	clock.Observe(playback.Sample{Position: ms(9_900), Duration: ms(10_000), Speed: 1, Status: playback.StatusPlaying, ObservedAt: now.Now()})
	now.Advance(time.Minute)
	if got := clock.Position(); got != ms(10_000) {
		t.Fatalf("Position() past the end = %v, want the duration", got)
	}

	clock.Observe(playback.Sample{Position: -ms(300), Speed: 1, Status: playback.StatusPaused, ObservedAt: now.Now()})
	if got := clock.Position(); got != 0 {
		t.Fatalf("negative report = %v, want 0", got)
	}
}

func TestEstimatedClockIgnoresRepeatedStaleValues(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
	if !signalled(clock) {
		t.Fatal("first sample did not signal a change")
	}

	now.Advance(ms(200))
	clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now())) // stale repeat
	if signalled(clock) || clock.Position() != ms(5_200) {
		t.Fatalf("stale repeat moved the clock to %v", clock.Position())
	}

	now.Advance(ms(50))
	clock.Observe(sample(ms(5_260), playback.StatusPlaying, 1, now.Now())) // fresh value
	if !signalled(clock) || clock.Position() != ms(5_260) {
		t.Fatalf("fresh value not adopted, position %v", clock.Position())
	}

	clock.Observe(sample(ms(5_260), playback.StatusPaused, 1, now.Now())) // same value, new status
	now.Advance(ms(400))
	if !signalled(clock) || clock.Position() != ms(5_260) {
		t.Fatalf("pause with the same value was ignored, position %v", clock.Position())
	}
}

func TestExactClockAdoptsEverySample(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
	signalled(clock)
	now.Advance(ms(200))

	clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
	if !signalled(clock) || clock.Position() != ms(5_000) {
		t.Fatalf("exact clock ignored a sample, position %v", clock.Position())
	}
}

func TestClockIgnoresOldPositionsRightAfterASeek(t *testing.T) {
	for _, precision := range []playback.Precision{playback.PrecisionExact, playback.PrecisionEstimated} {
		t.Run(string(precision), func(t *testing.T) {
			now := newFakeTime()
			clock := session.NewClock(precision, now.Now)
			clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
			clock.SetPosition(ms(60_000))

			now.Advance(ms(100))
			clock.Observe(sample(ms(5_150), playback.StatusPlaying, 1, now.Now())) // player not there yet
			if got := clock.Position(); got != ms(60_100) {
				t.Fatalf("stale pre-seek sample moved the clock to %v", got)
			}

			now.Advance(ms(100))
			clock.Observe(sample(ms(60_150), playback.StatusPlaying, 1, now.Now())) // player caught up
			if got := clock.Position(); got != ms(60_150) {
				t.Fatalf("sample near the target not adopted: %v", got)
			}

			now.Advance(time.Second) // grace period over: a real jump is accepted
			clock.Observe(sample(ms(2_000), playback.StatusPlaying, 1, now.Now()))
			if got := clock.Position(); got != ms(2_000) {
				t.Fatalf("jump after the grace period ignored: %v", got)
			}
		})
	}
}

func TestClockSettersReanchorImmediately(t *testing.T) {
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(1_000), playback.StatusPlaying, 1, now.Now()))
	signalled(clock)
	now.Advance(ms(1_000))

	clock.SetSpeed(2)
	if !signalled(clock) {
		t.Fatal("SetSpeed did not signal")
	}
	now.Advance(ms(500))
	if got := clock.Position(); got != ms(3_000) {
		t.Fatalf("after 1s at 1x and 0.5s at 2x position = %v, want 3s", got)
	}

	clock.SetStatus(playback.StatusPaused)
	now.Advance(ms(500))
	if got := clock.Position(); got != ms(3_000) {
		t.Fatalf("paused clock moved to %v", got)
	}
}
