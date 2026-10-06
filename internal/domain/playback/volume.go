package playback

import (
	"math"
	"time"
)

// Volume is the player's loudness: 0 is silent, 1 the player's 100%, and
// above it the player amplifies, up to 2 (200%) on players that can.
type Volume float64

// Volume limits and the steps of the volume keys.
const (
	MinVolume  Volume = 0
	FullVolume Volume = 1
	// MaxVolume is the loudest any player is asked for; each player's own
	// ceiling is in its capabilities.
	MaxVolume Volume = 2
	// VolumeStep is how far a held volume key moves the level each time.
	VolumeStep Volume = 0.01
	// DefaultVolumeTouch is how far one touch of a volume key moves it, as
	// the usual players do; MinVolumeTouch and MaxVolumeTouch bound the
	// user's choice.
	DefaultVolumeTouch Volume = 0.05
	MinVolumeTouch     Volume = 0.01
	MaxVolumeTouch     Volume = 0.25
)

// ValidTouch reports a touch the user may choose; zero stands for the
// default.
func ValidTouch(touch Volume) bool {
	return touch == 0 || (touch >= MinVolumeTouch && touch <= MaxVolumeTouch)
}

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

// How the volume keys move the level: a touch moves it at once; held, the
// level then moves on its own pace, one step per interval (about 20% a
// second), measured by the clock and not by the keyboard's repeat rate, so a
// held key is gradual on every keyboard.
const (
	VolumeHoldDelay    = 400 * time.Millisecond
	VolumeHoldInterval = 50 * time.Millisecond
	// maxHeldSteps bounds the steps one late repeat can catch up on.
	maxHeldSteps = 10
)

// VolumeRamp turns presses of the volume keys into levels, held keys going
// all the way to the player's ceiling (200% on VLC). A ramp is not safe for
// concurrent use.
type VolumeRamp struct {
	// Touch is how far one touch moves the level; zero, or anything outside
	// the user's range, is DefaultVolumeTouch.
	Touch     Volume
	heldSince time.Time
	given     int
}

// Next is the level after a press (repeat false) or a repeat of a held key,
// from the current level, inside the player's ceiling.
func (r *VolumeRamp) Next(now time.Time, up, repeat bool, current, ceiling Volume) Volume {
	delta := r.touch()
	if !repeat || r.heldSince.IsZero() {
		r.heldSince, r.given = now, 0
	} else {
		due := int((now.Sub(r.heldSince)-VolumeHoldDelay)/VolumeHoldInterval) + 1
		if now.Sub(r.heldSince) < VolumeHoldDelay || due <= r.given {
			return current.Within(ceiling)
		}
		delta = Volume(min(due-r.given, maxHeldSteps)) * VolumeStep
		r.given = due
	}
	if !up {
		delta = -delta
	}
	return (current + delta).Within(ceiling)
}

func (r *VolumeRamp) touch() Volume {
	if r.Touch == 0 || !ValidTouch(r.Touch) {
		return DefaultVolumeTouch
	}
	return r.Touch
}
