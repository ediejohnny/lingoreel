package session_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/subtitle"
)

type change struct {
	position    time.Duration
	primary     []string
	translation []string
}

// runnerHarness runs a Runner against fake time and records its reports.
type runnerHarness struct {
	t       *testing.T
	time    *fakeTime
	clock   *session.Clock
	runner  *session.Runner
	samples chan playback.Sample
	changes chan change
	done    chan error
	cancel  context.CancelFunc
}

func startRunner(t *testing.T, subtitles *session.Subtitles, first playback.Sample) *runnerHarness {
	t.Helper()
	return launchRunner(t, subtitles, func(clock *session.Clock, now time.Time) {
		first.ObservedAt = now
		clock.Observe(first)
	})
}

// launchRunner starts a runner after prepare has set up its clock.
func launchRunner(t *testing.T, subtitles *session.Subtitles, prepare func(*session.Clock, time.Time)) *runnerHarness {
	t.Helper()
	h := &runnerHarness{t: t, time: newFakeTime(), samples: make(chan playback.Sample), changes: make(chan change, 64), done: make(chan error, 1)}
	h.clock = session.NewClock(playback.PrecisionExact, h.time.Now)
	prepare(h.clock, h.time.Now())
	h.runner = session.NewRunner(h.clock, subtitles, func(position time.Duration, visible session.Visible) {
		h.changes <- change{position, ids(visible.Primary), ids(visible.Translation)}
	}, h.time.NewTimer)

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.done <- h.runner.Run(ctx, h.samples) }()
	t.Cleanup(cancel)
	return h
}

func (h *runnerHarness) expect(position time.Duration, primary, translation []string) {
	h.t.Helper()
	select {
	case got := <-h.changes:
		if got.position != position || !slices.Equal(got.primary, primary) || !slices.Equal(got.translation, translation) {
			h.t.Fatalf("change = %+v, want at %v %v / %v", got, position, primary, translation)
		}
	case <-time.After(2 * time.Second):
		h.t.Fatalf("no change reported, want at %v %v / %v", position, primary, translation)
	}
}

func (h *runnerHarness) expectQuiet() {
	h.t.Helper()
	select {
	case got := <-h.changes:
		h.t.Fatalf("unexpected change %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// advance waits until the runner sleeps, then moves time forward.
func (h *runnerHarness) advance(d time.Duration) {
	h.t.Helper()
	h.time.waitForTimer(h.t)
	h.time.Advance(d)
}

func twoLines(t *testing.T) *session.Subtitles {
	return session.NewSubtitles(
		&session.Track{Timeline: timeline(t, cue("a", 1_000, 3_000), cue("b", 2_000, 5_000))},
		&session.Track{Timeline: timeline(t, cue("x", 1_500, 4_000))},
	)
}

func TestRunnerReportsEachBoundaryWhilePlaying(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPlaying, 1, epoch))
	none := []string{}

	h.expect(0, none, none)
	h.advance(ms(1_000))
	h.expect(ms(1_000), []string{"a"}, none)
	h.advance(ms(500))
	h.expect(ms(1_500), []string{"a"}, []string{"x"})
	h.advance(ms(500))
	h.expect(ms(2_000), []string{"a", "b"}, []string{"x"}) // two simultaneous events
	h.advance(ms(1_000))
	h.expect(ms(3_000), []string{"b"}, []string{"x"})
	h.advance(ms(1_000))
	h.expect(ms(4_000), []string{"b"}, none)
	h.advance(ms(1_000))
	h.expect(ms(5_000), none, none)
}

func TestRunnerFollowsSpeed(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPlaying, 2, epoch))
	none := []string{}

	h.expect(0, none, none)
	h.advance(ms(500)) // 1 s of video at 2x
	h.expect(ms(1_000), []string{"a"}, none)
}

func TestRunnerRecomputesRightAfterASeek(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPaused, 1, epoch))
	none := []string{}
	h.expect(0, none, none)

	h.time.waitForTimer(t)
	h.clock.SetPosition(ms(3_500)) // no time passes
	h.expect(ms(3_500), []string{"b"}, []string{"x"})

	h.time.waitForTimer(t)
	h.clock.SetPosition(ms(6_000)) // into a region without subtitles
	h.expect(ms(6_000), none, none)
}

func TestRunnerStaysQuietWhilePaused(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(ms(1_200), playback.StatusPaused, 1, epoch))
	h.expect(ms(1_200), []string{"a"}, []string{})

	h.advance(10 * time.Second)
	h.expectQuiet()
}

func TestRunnerAppliesOffsetsAfterRefresh(t *testing.T) {
	subtitles := twoLines(t)
	h := startRunner(t, subtitles, sample(ms(1_200), playback.StatusPaused, 1, epoch))
	h.expect(ms(1_200), []string{"a"}, []string{})

	h.time.waitForTimer(t)
	subtitles.SetOffset(session.LineTranslation, subtitle.NewOffset(-ms(500)))
	h.runner.Refresh()
	h.expect(ms(1_200), []string{"a"}, []string{"x"})
}

func TestRunnerAdoptsSamplesAndStopsWhenThePlayerEnds(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPaused, 1, epoch))
	h.expect(0, []string{}, []string{})

	h.samples <- sample(ms(2_500), playback.StatusPaused, 1, h.time.Now())
	h.expect(ms(2_500), []string{"a", "b"}, []string{"x"})

	close(h.samples)
	select {
	case err := <-h.done:
		if err != nil {
			t.Fatalf("Run returned %v after the player ended, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop when the samples channel closed")
	}
}

// A session opened at 3.5 s must never flash the line at 0 s while the player
// has not reported its position yet.
func TestRunnerWaitsForThePlayerBeforeTheFirstReport(t *testing.T) {
	subtitles := session.NewSubtitles(
		&session.Track{Timeline: timeline(t, cue("opening", 0, 1_000), cue("late", 3_000, 4_000))},
		&session.Track{Timeline: timeline(t, cue("abertura", 0, 1_000))},
	)
	h := launchRunner(t, subtitles, func(*session.Clock, time.Time) {})
	h.expectQuiet()

	h.samples <- sample(ms(3_500), playback.StatusPaused, 1, h.time.Now())
	h.expect(ms(3_500), []string{"late"}, []string{})
	h.expectQuiet()
}

// Play or pause issued by the session is enough: the clock then knows the
// state without waiting for the player.
func TestRunnerReportsOnceTheSessionSetsTheStatus(t *testing.T) {
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, cue("opening", 0, 1_000))}, nil)
	h := launchRunner(t, subtitles, func(clock *session.Clock, _ time.Time) {
		clock.SetStatus(playback.StatusPaused)
	})
	h.expect(0, []string{"opening"}, []string{})
}

func TestRunnerStopsOnCancellation(t *testing.T) {
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPlaying, 1, epoch))
	h.expect(0, []string{}, []string{})

	h.cancel()
	select {
	case err := <-h.done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run ignored the cancellation")
	}
}
