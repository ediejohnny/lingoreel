// Package session coordinates a running study session: the playback clock,
// the player controls and the subtitles visible at each instant.
package session

import (
	"math"
	"sync"
	"time"

	"lingoreel/internal/domain/playback"
)

// After the session moves the position itself (seek, frame step), players
// may still report the old position for a moment. Samples far from the
// expected position are ignored during this grace period.
const (
	seekGracePeriod = 700 * time.Millisecond
	seekTolerance   = time.Second
)

// Clock turns player samples into a continuous playback position.
// It is safe for concurrent use.
type Clock struct {
	mu        sync.Mutex
	now       func() time.Time
	precision playback.Precision
	anchor    playback.Sample
	anchored  bool
	// lastReported is the last position the player reported; estimated
	// players repeat stale values, which must not move the anchor.
	lastReported time.Duration
	graceUntil   time.Time
	changes      chan struct{}
}

// NewClock creates a clock for a player of the given precision.
func NewClock(precision playback.Precision, now func() time.Time) *Clock {
	return &Clock{now: now, precision: precision, changes: make(chan struct{}, 1)}
}

// Changes signals every time the anchor moves, so subtitles are recomputed.
func (c *Clock) Changes() <-chan struct{} {
	return c.changes
}

// Observe feeds a player sample into the clock.
func (c *Clock) Observe(sample playback.Sample) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sample.Speed = usableSpeed(sample.Speed)
	stateChanged := !c.anchored || sample.Status != c.anchor.Status || sample.Speed != c.anchor.Speed
	c.anchor.Duration = sample.Duration

	if !stateChanged && sample.ObservedAt.Before(c.graceUntil) {
		expected := c.extrapolate(sample.ObservedAt)
		if absolute(sample.Position-expected) > seekTolerance {
			return
		}
	}
	valueChanged := sample.Position != c.lastReported
	c.lastReported = sample.Position
	if stateChanged || valueChanged || c.precision == playback.PrecisionExact {
		c.anchor = sample
		c.anchored = true
		c.signal()
	}
}

// Position returns the current playback position.
func (c *Clock) Position() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.extrapolate(c.now())
}

// State returns the current sample: position, speed, status and duration.
func (c *Clock) State() playback.Sample {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	state := c.anchor
	state.Position = c.extrapolate(now)
	state.ObservedAt = now
	if !c.anchored {
		state.Speed = playback.NormalSpeed
		state.Status = playback.StatusPaused
	}
	return state
}

// SetPosition moves the clock after the session itself moved the player.
func (c *Clock) SetPosition(position time.Duration) {
	c.reanchor(func(sample *playback.Sample) { sample.Position = position })
}

// SetStatus records a play or pause requested by the session.
func (c *Clock) SetStatus(status playback.Status) {
	c.reanchor(func(sample *playback.Sample) { sample.Status = status })
}

// SetSpeed records a speed change requested by the session.
func (c *Clock) SetSpeed(speed playback.Speed) {
	c.reanchor(func(sample *playback.Sample) { sample.Speed = usableSpeed(speed) })
}

func (c *Clock) reanchor(change func(*playback.Sample)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	sample := c.anchor
	if !c.anchored {
		sample = playback.Sample{Speed: playback.NormalSpeed, Status: playback.StatusPaused}
	}
	sample.Position = c.extrapolate(now)
	sample.ObservedAt = now
	change(&sample)
	c.anchor = sample
	c.anchored = true
	c.graceUntil = now.Add(seekGracePeriod)
	c.signal()
}

// extrapolate projects the anchor to an instant; callers hold the lock.
func (c *Clock) extrapolate(at time.Time) time.Duration {
	if !c.anchored {
		return 0
	}
	position := c.anchor.Position
	if c.anchor.Status == playback.StatusPlaying {
		if elapsed := at.Sub(c.anchor.ObservedAt); elapsed > 0 {
			position += time.Duration(float64(elapsed) * float64(c.anchor.Speed))
		}
	}
	position = max(position, 0)
	if c.anchor.Duration > 0 {
		position = min(position, c.anchor.Duration)
	}
	return position
}

func (c *Clock) signal() {
	select {
	case c.changes <- struct{}{}:
	default:
	}
}

// usableSpeed keeps extrapolation sane when a player reports a rate that is
// not a positive finite number.
func usableSpeed(speed playback.Speed) playback.Speed {
	if value := float64(speed); value > 0 && !math.IsInf(value, 0) {
		return speed
	}
	return playback.NormalSpeed
}

func absolute(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
