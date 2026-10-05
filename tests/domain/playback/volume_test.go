package playback_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// A volume goes from 0 to 1; outside it is invalid and clamps to the
// nearest end. Not a number is silent: never louder than asked.
func TestVolumeBounds(t *testing.T) {
	cases := []struct {
		in      playback.Volume
		valid   bool
		clamped playback.Volume
	}{
		{0, true, 0},
		{0.5, true, 0.5},
		{1, true, 1},
		{-0.01, false, 0},
		{1.01, false, 1},
		{playback.Volume(math.Inf(1)), false, 1},
		{playback.Volume(math.Inf(-1)), false, 0},
		{playback.Volume(math.NaN()), false, 0},
	}
	for _, c := range cases {
		if got := c.in.Valid(); got != c.valid {
			t.Errorf("Volume(%v).Valid() = %v, want %v", c.in, got, c.valid)
		}
		if got := c.in.Clamp(); got != c.clamped {
			t.Errorf("Volume(%v).Clamp() = %v, want %v", c.in, got, c.clamped)
		}
	}
}
