package playback

import (
	"math"
	"time"
)

// Frame rates outside this range are treated as unknown.
const (
	MinFrameRate = 1.0
	MaxFrameRate = 1000.0
)

// FrameDuration returns the length of one frame at the given rate, rounded to
// the nearest nanosecond. It reports false for unknown or absurd rates: frame
// stepping must follow the real rate of the video, never an assumed one.
func FrameDuration(framesPerSecond float64) (time.Duration, bool) {
	if !(framesPerSecond >= MinFrameRate && framesPerSecond <= MaxFrameRate) {
		return 0, false
	}
	return time.Duration(math.Round(float64(time.Second) / framesPerSecond)), true
}
