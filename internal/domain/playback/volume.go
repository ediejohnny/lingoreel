package playback

import (
	"math"
	"time"
)

// Volume is the player's loudness: 0 is silent, 1 the player's 100%, and
// above it the player amplifies, up to 2 (200%) on players that can.
type Volume float64

// Volume limits and the step of the volume keys.
const (
	MinVolume  Volume = 0
	FullVolume Volume = 1
	// MaxVolume is the loudest any player is asked for; each player's own
	// ceiling is in its capabilities.
	MaxVolume  Volume = 2
	VolumeStep Volume = 0.01
)

// Valid reports a volume inside the supported range.
func (v Volume) Valid() bool {
	return v >= MinVolume && v <= MaxVolume
}

// Clamp keeps a volume inside the range; a value that is not a number is
// silent, never louder than asked.
func (v Volume) Clamp() Volume {
	return v.Within(MaxVolume)
}

// Within keeps a volume between silent and a ceiling, rounded to a whole
// percent; a value that is not a number is silent.
func (v Volume) Within(ceiling Volume) Volume {
	if math.IsNaN(float64(v)) {
		return MinVolume
	}
	ceiling = min(max(ceiling, MinVolume), MaxVolume)
	return Volume(math.Round(float64(min(max(v, MinVolume), ceiling))*100) / 100)
}

// Percent is the volume in whole percent, as the screens show it.
func (v Volume) Percent() int {
	return int(math.Round(float64(v) * 100))
}

// How the volume keys move the level: a touch moves one step at once; held,
// the level then moves on its own pace, one step per interval, measured by
// the clock and not by the keyboard's repeat rate, so a held key never
// raises the sound faster than about 9% a second.
const (
	VolumeHoldDelay    = 400 * time.Millisecond
	VolumeHoldInterval = 110 * time.Millisecond
	// maxHeldSteps bounds the steps one late repeat can catch up on.
	maxHeldSteps = 3
)

// VolumeRamp turns presses of the volume keys into levels. Going up, a held
// key stops at 100%: louder than that, the player amplifies and the sound
// may clip, so passing it takes a new press. A ramp is not safe for
// concurrent use.
type VolumeRamp struct {
	heldSince time.Time
	from      Volume
	given     int
}

// Next is the level after a press (repeat false) or a repeat of a held key,
// from the current level, inside the player's ceiling.
func (r *VolumeRamp) Next(now time.Time, up, repeat bool, current, ceiling Volume) Volume {
	steps := 1
	if !repeat || r.heldSince.IsZero() {
		r.heldSince, r.from, r.given = now, current, 0
	} else {
		due := int((now.Sub(r.heldSince)-VolumeHoldDelay)/VolumeHoldInterval) + 1
		if now.Sub(r.heldSince) < VolumeHoldDelay || due <= r.given {
			return current.Within(ceiling)
		}
		steps = min(due-r.given, maxHeldSteps)
		r.given = due
	}
	delta := Volume(steps) * VolumeStep
	if !up {
		delta = -delta
	}
	next := (current + delta).Within(ceiling)
	if up && repeat && r.from < FullVolume && next > FullVolume {
		next = FullVolume.Within(ceiling)
	}
	return next
}
