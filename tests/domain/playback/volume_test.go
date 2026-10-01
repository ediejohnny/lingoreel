package playback_test

import (
	"math"
	"testing"

	"lingoreel/internal/domain/playback"
)

func TestVolumeRange(t *testing.T) {
	for v, want := range map[playback.Volume]bool{0: true, 1: true, 0.5: true, -0.01: false, 1.01: false} {
		if got := v.Valid(); got != want {
			t.Errorf("Volume(%v).Valid() = %v, want %v", v, got, want)
		}
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if playback.Volume(bad).Valid() {
			t.Errorf("Volume(%v) valid", bad)
		}
	}
}

func TestVolumeClamp(t *testing.T) {
	cases := []struct{ in, want playback.Volume }{
		{0.5, 0.5}, {-1, 0}, {2, 1}, {0, 0}, {1, 1},
		{playback.Volume(math.Inf(1)), 1}, {playback.Volume(math.Inf(-1)), 0},
		// Not a number is silent: never louder than asked.
		{playback.Volume(math.NaN()), 0},
	}
	for _, c := range cases {
		if got := c.in.Clamp(); got != c.want {
			t.Errorf("Clamp(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
