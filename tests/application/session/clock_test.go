package session_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

func signalled(clock *session.Clock) bool {
	select {
	case <-clock.Changes():
		return true
	default:
		return false
	}
}

func TestClockBeforeAnySampleIsUnknownAtZero(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	now.Advance(time.Hour)

	state := clock.State()
	if state.Position != 0 || state.Status != playback.StatusUnknown || state.Speed != playback.NormalSpeed {
		t.Fatalf("initial state = %+v", state)
	}
}

// A seek issued by the session before any sample makes the state known: the
// clock sits paused where the session put it.
func TestClockSessionCommandBeforeAnySampleMakesTheStateKnown(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.SetPosition(ms(3_000))
	now.Advance(time.Second)

	state := clock.State()
	if state.Position != ms(3_000) || state.Status != playback.StatusPaused || state.Speed != playback.NormalSpeed {
		t.Fatalf("state after SetPosition = %+v", state)
	}
}

// A sample read before the session's own play arrives after it (a slow
// computer, a busy player): it tells the pause the play ended, and must not
// put the video back to paused until the next sample.
func TestClockASampleOlderThanACommandIsStale(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	clock.Observe(playback.Sample{Position: ms(1_000), Speed: 1, Status: playback.StatusPaused, ObservedAt: now.Now()})
	read := now.Now()
	now.Advance(100 * time.Millisecond)
	clock.SetStatus(playback.StatusPlaying)
	clock.Observe(playback.Sample{Position: ms(1_000), Speed: 1, Status: playback.StatusPaused, ObservedAt: read})
	if state := clock.State(); state.Status != playback.StatusPlaying {
		t.Fatalf("a stale sample undid the play: %+v", state)
	}
	now.Advance(100 * time.Millisecond)
	clock.Observe(playback.Sample{Position: ms(1_100), Speed: 1, Status: playback.StatusPaused, ObservedAt: now.Now()})
	if state := clock.State(); state.Status != playback.StatusPaused {
		t.Fatalf("a sample after the play was ignored: %+v", state)
	}
}

// The sound heard at an instant belongs to the position the video had then,
// even an instant before the player's last report: sound arrives late.
func TestClockPositionAtAnInstant(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionExact, now.Now)
	reported := now.Now()
	clock.Observe(sample(ms(10_000), playback.StatusPlaying, 2, reported))
	if got := clock.PositionAt(reported.Add(ms(500))); got != ms(11_000) {
		t.Fatalf("half a second after at 2x: %v, want 11s", got)
	}
	if got := clock.PositionAt(reported.Add(-ms(250))); got != ms(9_500) {
		t.Fatalf("a quarter second before at 2x: %v, want 9.5s", got)
	}
	clock.Observe(sample(ms(10_000), playback.StatusPaused, 2, reported))
	if got := clock.PositionAt(reported.Add(-ms(250))); got != ms(10_000) {
		t.Fatalf("paused: %v, want 10s", got)
	}
}
func TestClockExtrapolatesAtTheReportedSpeed(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// An estimated sample 40 ms behind while playing must not move the clock back
// (that flashed the previous subtitle in a real VLC session): the clock keeps
// going, slower, and meets the player's track after the 600 ms slew.
func TestEstimatedClockNeverStepsBackForASmallCorrection(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(7_000), playback.StatusPlaying, 1, now.Now()))
	now.Advance(ms(250))
	before := clock.Position() // 7250
	clock.Observe(sample(ms(7_210), playback.StatusPlaying, 1, now.Now()))
	if !signalled(clock) {
		t.Fatal("the correction did not signal a change")
	}
	last := before
	for step := 0; step <= 12; step++ {
		position := clock.Position()
		if position < last {
			t.Fatalf("clock stepped back from %v to %v at step %d", last, position, step)
		}
		last = position
		now.Advance(ms(50))
	}
	// 650 ms after the sample, the player's track is at 7210 + 650 = 7860.
	if got, want := clock.Position(), ms(7_860); absolute(got-want) > time.Millisecond {
		t.Fatalf("after the slew position = %v, want %v", got, want)
	}
}

// The slew budget is half the speed over 600 ms: 300 ms at 1x. A larger
// correction is a real jump (the player skipped) and applies at once.
func TestEstimatedClockAppliesLargeOrForwardCorrectionsAtOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		speed    playback.Speed
		reported time.Duration
	}{
		{"behind, past the limit", 1, ms(7_250 - 301)},
		{"behind, a smaller limit at 0.25x", 0.25, ms(7_250 - 76)},
		{"ahead", 1, ms(7_300)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := newFakeTime()
			clock := session.NewClock(playback.PrecisionEstimated, now.Now)
			clock.Observe(sample(ms(7_000), playback.StatusPlaying, c.speed, now.Now()))
			now.Advance(time.Duration(float64(ms(250)) / float64(c.speed)))
			clock.Observe(sample(c.reported, playback.StatusPlaying, c.speed, now.Now()))
			if got := clock.Position(); got != c.reported {
				t.Fatalf("position = %v, want the reported %v at once", got, c.reported)
			}
		})
	}
}

// Within the budget, a correction just under it is still smoothed.
func TestEstimatedClockSmoothsACorrectionAtTheBudget(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(7_000), playback.StatusPlaying, 1, now.Now()))
	now.Advance(ms(250))
	clock.Observe(sample(ms(7_250-300), playback.StatusPlaying, 1, now.Now()))
	if got := clock.Position(); got != ms(7_250) {
		t.Fatalf("position = %v, want 7.25s kept (300 ms is within the budget)", got)
	}
}

// Right after a session seek the clock holds a guess: the player's first
// answer slightly behind it is adopted at once, not smoothed.
func TestFirstAnswerAfterASeekIsAdoptedEvenIfBehind(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
	clock.SetPosition(ms(60_000))
	now.Advance(ms(200))
	clock.Observe(sample(ms(60_120), playback.StatusPlaying, 1, now.Now()))
	if got := clock.Position(); got != ms(60_120) {
		t.Fatalf("position = %v, want the player's 60.12s", got)
	}
}

// A pause during a slew stops exactly where the player says.
func TestPauseEndsASlewAtThePlayersPosition(t *testing.T) {
	t.Parallel()
	now := newFakeTime()
	clock := session.NewClock(playback.PrecisionEstimated, now.Now)
	clock.Observe(sample(ms(7_000), playback.StatusPlaying, 1, now.Now()))
	now.Advance(ms(250))
	clock.Observe(sample(ms(7_210), playback.StatusPlaying, 1, now.Now()))
	now.Advance(ms(100))
	clock.Observe(sample(ms(7_300), playback.StatusPaused, 1, now.Now()))
	now.Advance(time.Second)
	if got := clock.Position(); got != ms(7_300) {
		t.Fatalf("paused position = %v, want 7.3s", got)
	}
}

func absolute(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func TestExactClockAdoptsEverySample(t *testing.T) {
	t.Parallel()
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

// Right after a seek the player's stale answers are ignored. Every jump is
// counted, the session's and the player's own, and nothing else: the voice
// heard before a jump says nothing of what comes.
func TestClockIgnoresOldPositionsRightAfterASeek(t *testing.T) {
	t.Parallel()
	for _, precision := range []playback.Precision{playback.PrecisionExact, playback.PrecisionEstimated} {
		t.Run(string(precision), func(t *testing.T) {
			now := newFakeTime()
			clock := session.NewClock(precision, now.Now)
			clock.Observe(sample(ms(5_000), playback.StatusPlaying, 1, now.Now()))
			clock.SetPosition(ms(60_000))
			seeks := func(want uint64) {
				t.Helper()
				if got := clock.Seeks(); got != want {
					t.Fatalf("seeks = %d, want %d", got, want)
				}
			}
			seeks(1)

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
			seeks(1)

			now.Advance(time.Second) // grace period over: a real jump is accepted
			clock.Observe(sample(ms(2_000), playback.StatusPlaying, 1, now.Now()))
			if got := clock.Position(); got != ms(2_000) {
				t.Fatalf("jump after the grace period ignored: %v", got)
			}
			seeks(2)
		})
	}
}

func TestClockSettersReanchorImmediately(t *testing.T) {
	t.Parallel()
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
