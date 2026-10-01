package playback

import "math"

// Volume is the player's loudness, from 0 (silent) to 1 (the player's 100%).
type Volume float64

// Volume limits and the step of the volume keys.
const (
	MinVolume  Volume = 0
	MaxVolume  Volume = 1
	VolumeStep Volume = 0.05
)

// Valid reports a volume inside the supported range.
func (v Volume) Valid() bool {
	return v >= MinVolume && v <= MaxVolume
}

// Clamp keeps a volume inside the range; a value that is not a number is
// silent, never louder than asked.
func (v Volume) Clamp() Volume {
	if math.IsNaN(float64(v)) {
		return MinVolume
	}
	return min(max(v, MinVolume), MaxVolume)
}
