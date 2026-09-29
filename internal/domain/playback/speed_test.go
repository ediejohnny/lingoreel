package playback

import (
	"math"
	"testing"
)

func TestSpeedValid(t *testing.T) {
	cases := map[Speed]bool{
		MinSpeed:           true,
		MaxSpeed:           true,
		NormalSpeed:        true,
		0.2499:             false,
		2.0001:             false,
		0:                  false,
		-1:                 false,
		Speed(math.NaN()):  false,
		Speed(math.Inf(1)): false,
		MaxSpeed + 1e-12:   true, // floating point noise
	}
	for speed, want := range cases {
		if got := speed.Valid(); got != want {
			t.Fatalf("Speed(%v).Valid() = %v, want %v", speed, got, want)
		}
	}
}

func TestSpeedFaster(t *testing.T) {
	cases := []struct{ from, want Speed }{
		{NormalSpeed, 1.1},
		{0.75, 0.8},
		{1.05, 1.1},          // between presets
		{1.0999999999, 1.25}, // noise around a preset counts as that preset
		{MaxSpeed, MaxSpeed},
		{3, MaxSpeed},
		{0.1, MinSpeed},
		{Speed(math.NaN()), NormalSpeed},
	}
	for _, tc := range cases {
		if got := tc.from.Faster(); got != tc.want {
			t.Fatalf("Speed(%v).Faster() = %v, want %v", tc.from, got, tc.want)
		}
	}
}

func TestSpeedSlower(t *testing.T) {
	cases := []struct{ from, want Speed }{
		{NormalSpeed, 0.9},
		{0.3, MinSpeed},
		{MinSpeed, MinSpeed},
		{1.1000000001, NormalSpeed},
		{3, MaxSpeed},
		{0, MinSpeed},
		{Speed(math.NaN()), NormalSpeed},
	}
	for _, tc := range cases {
		if got := tc.from.Slower(); got != tc.want {
			t.Fatalf("Speed(%v).Slower() = %v, want %v", tc.from, got, tc.want)
		}
	}
}

func TestSpeedStepsWalkEveryPresetBothWays(t *testing.T) {
	steps := SpeedSteps()
	current := MinSpeed
	for _, want := range steps[1:] {
		current = current.Faster()
		if current != want {
			t.Fatalf("walking up reached %v, want %v", current, want)
		}
	}
	for i := len(steps) - 2; i >= 0; i-- {
		current = current.Slower()
		if current != steps[i] {
			t.Fatalf("walking down reached %v, want %v", current, steps[i])
		}
	}
}

func TestSpeedStepsReturnsACopy(t *testing.T) {
	SpeedSteps()[0] = 99
	if SpeedSteps()[0] != MinSpeed {
		t.Fatal("SpeedSteps exposed its internal slice")
	}
}
