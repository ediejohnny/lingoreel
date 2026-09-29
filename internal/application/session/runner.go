package session

import (
	"context"
	"time"

	"lingoreel/internal/domain/playback"
)

// maxIdleWait bounds how long the runner sleeps without any subtitle change
// ahead, so it still notices a player that stopped reporting.
const maxIdleWait = time.Second

// Timer abstracts time.Timer so tests can control time.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

type realTimer struct{ timer *time.Timer }

func (t realTimer) C() <-chan time.Time { return t.timer.C }
func (t realTimer) Stop() bool          { return t.timer.Stop() }

// NewRealTimer returns a timer backed by the system clock.
func NewRealTimer(d time.Duration) Timer {
	return realTimer{timer: time.NewTimer(d)}
}

// Runner keeps the visible subtitles in step with the clock. It wakes up at
// the next subtitle boundary instead of polling, and reports only changes.
type Runner struct {
	clock     *Clock
	subtitles *Subtitles
	onChange  func(position time.Duration, visible Visible)
	newTimer  func(time.Duration) Timer
	refresh   chan struct{}
}

// NewRunner creates a runner; once the clock knows the playback status (a
// player sample, or a command issued by the session), onChange receives every new set of visible
// subtitles, including the first one.
func NewRunner(clock *Clock, subtitles *Subtitles, onChange func(time.Duration, Visible), newTimer func(time.Duration) Timer) *Runner {
	return &Runner{clock: clock, subtitles: subtitles, onChange: onChange, newTimer: newTimer, refresh: make(chan struct{}, 1)}
}

// Refresh asks for an immediate recomputation, for instance after an offset change.
func (r *Runner) Refresh() {
	select {
	case r.refresh <- struct{}{}:
	default:
	}
}

// Run follows the clock until the context ends or the samples channel is
// closed, which means the player went away.
func (r *Runner) Run(ctx context.Context, samples <-chan playback.Sample) error {
	var last Visible
	first := true
	for {
		state := r.clock.State()
		visible := r.subtitles.At(state.Position)
		// Until the player reports, the position is a guess (zero), and showing
		// its subtitles would flash the wrong line on a session that starts later.
		known := state.Status != playback.StatusUnknown
		if known && (first || !visible.Equal(last)) {
			r.onChange(state.Position, visible)
			last, first = visible, false
		}

		timer := r.newTimer(r.waitFor(state))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case sample, open := <-samples:
			timer.Stop()
			if !open {
				return nil
			}
			r.clock.Observe(sample)
		case <-r.clock.Changes():
			timer.Stop()
		case <-r.subtitles.Changes():
			timer.Stop()
		case <-r.refresh:
			timer.Stop()
		case <-timer.C():
		}
	}
}

// waitFor converts the distance to the next subtitle boundary into wall time
// at the current speed.
func (r *Runner) waitFor(state playback.Sample) time.Duration {
	if state.Status != playback.StatusPlaying {
		return maxIdleWait
	}
	next, found := r.subtitles.NextChangeAfter(state.Position)
	if !found {
		return maxIdleWait
	}
	wait := time.Duration(float64(next-state.Position) / float64(usableSpeed(state.Speed)))
	return min(max(wait, time.Millisecond), maxIdleWait)
}
