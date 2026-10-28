// Package playerkit holds what the player plugins share: the checks of a
// launch's paths and the reading of a running player's state.
package playerkit

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// Timing is how often a player is read and how long it may stay silent:
// after a failed reading the next one waits FirstRetry, doubling up to
// MaxRetry; Limit without an answer and the player is lost.
type Timing struct {
	Interval   time.Duration
	FirstRetry time.Duration
	MaxRetry   time.Duration
	Limit      time.Duration
}

// DefaultTiming suits a player on this computer: read every 100 ms; while it
// runs but does not answer, asked again after 250 ms, doubling up to 2 s; lost
// after 10 s without an answer.
var DefaultTiming = Timing{
	Interval:   100 * time.Millisecond,
	FirstRetry: 250 * time.Millisecond,
	MaxRetry:   2 * time.Second,
	Limit:      10 * time.Second,
}

// Read asks the player where it is, once. The poller sets the sample's
// ObservedAt to the middle of the reading.
type Read func(ctx context.Context) (playback.Sample, error)

// Poller reads a player on a fixed interval and keeps only the latest
// sample for its consumer. A player that stops answering is asked again with
// growing waits, and reported as reconnecting meanwhile; one silent past the
// limit is lost.
type Poller struct {
	name   string
	timing Timing
	logger *slog.Logger

	samples      chan playback.Sample
	done         chan struct{}
	started      atomic.Bool
	lost         atomic.Bool
	reconnecting atomic.Bool
}

// NewPoller makes a poller of the player named in its log lines.
func NewPoller(name string, timing Timing, logger *slog.Logger) *Poller {
	return &Poller{
		name: name, timing: timing, logger: logger,
		samples: make(chan playback.Sample, 1), done: make(chan struct{}),
	}
}

// Start reads the player until ctx ends, exited closes or the player is
// lost; the samples close then. A poller starts once.
func (p *Poller) Start(ctx context.Context, exited <-chan struct{}, read Read) {
	if p.started.Swap(true) {
		return
	}
	go p.run(ctx, exited, read)
}

// Samples delivers the latest reading; it closes once the polling ends.
func (p *Poller) Samples() <-chan playback.Sample {
	return p.samples
}

// Lost reports a player that stopped answering past the limit while it
// still ran.
func (p *Poller) Lost() bool {
	return p.lost.Load()
}

// Reconnecting reports a player that runs but does not answer, while it is
// asked again.
func (p *Poller) Reconnecting() bool {
	return p.reconnecting.Load()
}

// Wait waits for a started poller to end, at most for limit; one never
// started has nothing to wait for.
func (p *Poller) Wait(limit time.Duration) bool {
	return !p.started.Load() || WaitClosed(p.done, limit)
}

func (p *Poller) run(ctx context.Context, exited <-chan struct{}, read Read) {
	defer close(p.done)
	defer close(p.samples)
	defer p.reconnecting.Store(false)
	timer := time.NewTimer(p.timing.Interval)
	defer timer.Stop()
	lastSuccess := time.Now()
	retry := time.Duration(0) // zero while the player answers

	for {
		select {
		case <-ctx.Done():
			return
		case <-exited:
			return
		case <-timer.C:
		}
		started := time.Now()
		sample, err := read(ctx)
		finished := time.Now()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if finished.Sub(lastSuccess) > p.timing.Limit {
				p.logger.Warn(p.name+" stopped answering", "error", err)
				p.lost.Store(true)
				return
			}
			if retry == 0 {
				retry = p.timing.FirstRetry
				p.reconnecting.Store(true)
				p.logger.Warn(p.name+" is not answering; reconnecting", "error", err)
			} else {
				retry = min(2*retry, p.timing.MaxRetry)
			}
			timer.Reset(retry)
			continue
		}
		if retry != 0 {
			p.logger.Info(p.name+" answered again", "after", finished.Sub(lastSuccess).Round(time.Millisecond))
			retry = 0
			p.reconnecting.Store(false)
		}
		lastSuccess = finished
		sample.ObservedAt = started.Add(finished.Sub(started) / 2)
		p.publish(sample)
		timer.Reset(p.timing.Interval)
	}
}

// publish replaces a sample the consumer has not read yet.
func (p *Poller) publish(sample playback.Sample) {
	select {
	case <-p.samples:
	default:
	}
	select {
	case p.samples <- sample:
	default:
	}
}

// WaitClosed waits for a channel to close, at most for limit.
func WaitClosed(done <-chan struct{}, limit time.Duration) bool {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
