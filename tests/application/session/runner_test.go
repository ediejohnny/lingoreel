package session_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
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
	// prepare's change signal predates the runner, which reads the state on
	// its first pass anyway. Left pending, it woke the runner once more and
	// created a timer the harness did not expect, so under load time was
	// advanced before the runner armed its real timer (a flaky failure).
	select {
	case <-h.clock.Changes():
	default:
	}
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
	t.Parallel()
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
	t.Parallel()
	h := startRunner(t, twoLines(t), sample(0, playback.StatusPlaying, 2, epoch))
	none := []string{}

	h.expect(0, none, none)
	h.advance(ms(500)) // 1 s of video at 2x
	h.expect(ms(1_000), []string{"a"}, none)
}

func TestRunnerRecomputesRightAfterASeek(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	h := startRunner(t, twoLines(t), sample(ms(1_200), playback.StatusPaused, 1, epoch))
	h.expect(ms(1_200), []string{"a"}, []string{})

	h.advance(10 * time.Second)
	h.expectQuiet()
}

func TestRunnerAppliesOffsetsAfterRefresh(t *testing.T) {
	t.Parallel()
	subtitles := twoLines(t)
	h := startRunner(t, subtitles, sample(ms(1_200), playback.StatusPaused, 1, epoch))
	h.expect(ms(1_200), []string{"a"}, []string{})

	h.time.waitForTimer(t)
	subtitles.SetOffset(session.LineTranslation, subtitle.NewOffset(-ms(500)))
	h.runner.Refresh()
	h.expect(ms(1_200), []string{"a"}, []string{"x"})
}

// An offset change must reach the screen by itself: a caller that forgets
// Refresh (the sync shortcuts, the settings panel) would otherwise wait for
// the next subtitle boundary, or forever while paused.
func TestRunnerAppliesOffsetsWithoutRefresh(t *testing.T) {
	t.Parallel()
	subtitles := twoLines(t)
	h := startRunner(t, subtitles, sample(ms(1_200), playback.StatusPaused, 1, epoch))
	h.expect(ms(1_200), []string{"a"}, []string{})

	h.time.waitForTimer(t)
	subtitles.SetOffset(session.LineTranslation, subtitle.NewOffset(-ms(500)))
	h.expect(ms(1_200), []string{"a"}, []string{"x"})
	h.time.waitForTimer(t)
	subtitles.SetOffset(session.LinePrimary, subtitle.NewOffset(ms(300)))
	h.expect(ms(1_200), []string{}, []string{"x"})
}

// Scenario E of the requirements: speed changes while playing, each one
// taking effect at once.
func TestRunnerFollowsSpeedChangesWhilePlaying(t *testing.T) {
	t.Parallel()
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t,
		cue("1", 1_000, 1_900), cue("2", 2_000, 2_900), cue("3", 3_000, 3_900), cue("4", 4_000, 4_900))}, nil)
	h := startRunner(t, subtitles, sample(0, playback.StatusPlaying, 1, epoch))
	none := []string{}
	h.expect(0, none, none)

	// 1.0x: 1 s of video in 1 s.
	h.advance(ms(1_000))
	h.expect(ms(1_000), []string{"1"}, none)
	h.time.waitForTimer(t)
	h.clock.SetSpeed(0.75)
	// 0.75x: the 900 ms left of "1" take 1200 ms.
	h.time.waitForTimer(t)
	h.time.Advance(ms(1_200))
	h.expect(ms(1_900), none, none)
	h.time.waitForTimer(t)
	h.clock.SetSpeed(1.25)
	// 1.25x: the 100 ms to "2" take 80 ms.
	h.time.waitForTimer(t)
	h.time.Advance(ms(80))
	h.expect(ms(2_000), []string{"2"}, none)
	h.time.waitForTimer(t)
	h.clock.SetSpeed(2)
	// 2x: the 900 ms left of "2" take 450 ms, the 100 ms gap to "3" 50 ms.
	h.time.waitForTimer(t)
	h.time.Advance(ms(450))
	h.expect(ms(2_900), none, none)
	h.advance(ms(50))
	h.expect(ms(3_000), []string{"3"}, none)
}

// A hostile file can stack thousands of overlapping events; only the most
// recent few of each line are shown.
func TestSubtitlesCapTheEventsShownPerLine(t *testing.T) {
	t.Parallel()
	var events []subtitle.Event
	for i := range 1_000 {
		events = append(events, cue(fmt.Sprint(i), i, 100_000))
	}
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, events...)}, nil)
	visible := subtitles.At(ms(50_000))
	if got := ids(visible.Primary); !slices.Equal(got, []string{"996", "997", "998", "999"}) {
		t.Fatalf("visible = %v, want the 4 latest", got)
	}
}

// An unknown line is ignored instead of panicking.
func TestSubtitlesIgnoreUnknownLines(t *testing.T) {
	t.Parallel()
	subtitles := twoLines(t)
	subtitles.SetOffset(session.Line(7), subtitle.NewOffset(ms(1)))
	subtitles.SetOffset(session.Line(-1), subtitle.NewOffset(ms(1)))
	if subtitles.Offset(session.Line(7)) != 0 || subtitles.Offset(session.Line(-1)) != 0 {
		t.Fatal("an unknown line reported an offset")
	}
}

func TestRunnerAdoptsSamplesAndStopsWhenThePlayerEnds(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, cue("opening", 0, 1_000))}, nil)
	h := launchRunner(t, subtitles, func(clock *session.Clock, _ time.Time) {
		clock.SetStatus(playback.StatusPaused)
	})
	h.expect(0, []string{"opening"}, []string{})
}

func TestRunnerStopsOnCancellation(t *testing.T) {
	t.Parallel()
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
