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

// An estimated sample slightly behind the clock while playing is not applied
// by jumping back, which would flash the previous subtitle for a moment: the
// clock runs slower for slewDuration until it meets the player. Corrections
// that would need more than maxSlowdown of the speed are applied at once.
const (
	slewDuration = 600 * time.Millisecond
	maxSlowdown  = 0.5
)

// stallLimit is how long a playing player may report the same position
// before the clock takes it as frozen: it then stops estimating, so the
// subtitles wait for the player instead of running ahead.
const stallLimit = 3 * time.Second

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
	// reportedSince is when the reported position or state last changed;
	// stalled holds the clock at the reported position.
	reportedSince time.Time
	stalled       bool
	graceUntil    time.Time
	// slewFactor scales the speed from the anchor until slewUntil.
	slewFactor float64
	slewUntil  time.Time
	changes    chan struct{}
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
	switch {
	case stateChanged || valueChanged:
		c.reportedSince = sample.ObservedAt
		c.stalled = false
	case !c.stalled && sample.Status == playback.StatusPlaying && sample.ObservedAt.Sub(c.reportedSince) >= stallLimit:
		c.stalled = true
		c.anchor = sample
		c.slewUntil = time.Time{}
		c.signal()
		return
	}
	if !stateChanged && !valueChanged && c.precision != playback.PrecisionExact {
		return
	}
	// Only estimated positions are smoothed: an exact player behind the clock
	// really is behind (it stalled), and must be followed at once. Right after
	// a session command the clock holds a guess, so the player's first answer
	// is adopted as is.
	smooth := c.precision == playback.PrecisionEstimated && !sample.ObservedAt.Before(c.graceUntil)
	if smooth && !stateChanged && sample.Status == playback.StatusPlaying && c.startSlew(sample) {
		c.signal()
		return
	}
	c.anchor = sample
	c.anchored = true
	c.slewUntil = time.Time{}
	c.signal()
}

// startSlew absorbs a small backward correction by slowing down instead of
// jumping back; it reports false when the correction must be applied at once.
func (c *Clock) startSlew(sample playback.Sample) bool {
	current := c.extrapolate(sample.ObservedAt)
	ahead := current - sample.Position
	budget := time.Duration(float64(slewDuration) * float64(sample.Speed) * maxSlowdown)
	if ahead <= 0 || ahead > budget {
		return false
	}
	c.anchor = sample
	c.anchor.Position = current
	c.slewFactor = 1 - float64(ahead)/(float64(slewDuration)*float64(sample.Speed))
	c.slewUntil = sample.ObservedAt.Add(slewDuration)
	return true
}

// Position returns the current playback position.
func (c *Clock) Position() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.extrapolate(c.now())
}

// State returns the current sample: position, speed, status and duration.
// The status stays unknown until a sample arrives or the session moves the
// clock.
func (c *Clock) State() playback.Sample {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	state := c.anchor
	state.Position = c.extrapolate(now)
	state.ObservedAt = now
	if !c.anchored {
		state.Speed = playback.NormalSpeed
		state.Status = playback.StatusUnknown
	}
	return state
}

// Stalled reports a playing player whose position stopped moving.
func (c *Clock) Stalled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stalled
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
	c.slewUntil = time.Time{}
	c.graceUntil = now.Add(seekGracePeriod)
	c.reportedSince, c.stalled = now, false
	c.signal()
}

// extrapolate projects the anchor to an instant; callers hold the lock.
func (c *Clock) extrapolate(at time.Time) time.Duration {
	if !c.anchored {
		return 0
	}
	position := c.anchor.Position
	if c.anchor.Status == playback.StatusPlaying && !c.stalled {
		if elapsed := at.Sub(c.anchor.ObservedAt); elapsed > 0 {
			speed := float64(c.anchor.Speed)
			slowed := min(elapsed, max(c.slewUntil.Sub(c.anchor.ObservedAt), 0))
			position += time.Duration(float64(slowed)*speed*c.slewFactor + float64(elapsed-slowed)*speed)
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
