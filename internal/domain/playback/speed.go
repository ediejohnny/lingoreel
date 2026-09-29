// Package playback models playback state and the rules for speed, seeking and
// frame stepping, independently of any concrete player.
package playback

import "math"

// Speed is a playback rate where 1 is normal speed.
type Speed float64

// Speed limits.
const (
	MinSpeed    Speed = 0.25
	MaxSpeed    Speed = 2
	NormalSpeed Speed = 1
)

// speedTolerance absorbs floating point noise when comparing rates.
const speedTolerance = 1e-9

// speedSteps are the preset rates offered to the user, in increasing order.
var speedSteps = []Speed{0.25, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}

// SpeedSteps returns the preset rates in increasing order.
func SpeedSteps() []Speed {
	steps := make([]Speed, len(speedSteps))
	copy(steps, speedSteps)
	return steps
}

// Valid reports whether the rate is within the supported range.
func (s Speed) Valid() bool {
	return s >= MinSpeed-speedTolerance && s <= MaxSpeed+speedTolerance
}

// Faster returns the next preset rate above s, or MaxSpeed. A rate that is not
// a number resets to NormalSpeed.
func (s Speed) Faster() Speed {
	if math.IsNaN(float64(s)) {
		return NormalSpeed
	}
	for _, step := range speedSteps {
		if step > s+speedTolerance {
			return step
		}
	}
	return MaxSpeed
}

// Slower returns the next preset rate below s, or MinSpeed. A rate that is not
// a number resets to NormalSpeed.
func (s Speed) Slower() Speed {
	if math.IsNaN(float64(s)) {
		return NormalSpeed
	}
	for i := len(speedSteps) - 1; i >= 0; i-- {
		if speedSteps[i] < s-speedTolerance {
			return speedSteps[i]
		}
	}
	return MinSpeed
}
