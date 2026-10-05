package playback_test

import (
	"math"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

func TestSpeedValid(t *testing.T) {
	cases := []struct {
		speed playback.Speed
		want  bool
	}{
		{playback.MinSpeed, true},
		{playback.MaxSpeed, true},
		{playback.NormalSpeed, true},
		{0.2499, false},
		{2.0001, false},
		{0, false},
		{-1, false},
		{playback.Speed(math.NaN()), false},
		{playback.Speed(math.Inf(1)), false},
		{playback.Speed(math.Inf(-1)), false},
		{playback.MaxSpeed + 1e-12, true}, // floating point noise
	}
	for _, tc := range cases {
		if got := tc.speed.Valid(); got != tc.want {
			t.Fatalf("Speed(%v).Valid() = %v, want %v", tc.speed, got, tc.want)
		}
	}
}

func TestSpeedFaster(t *testing.T) {
	cases := []struct{ from, want playback.Speed }{
		{playback.NormalSpeed, 1.1},
		{0.75, 0.8},
		{1.05, 1.1},          // between presets
		{1.0999999999, 1.25}, // noise around a preset counts as that preset
		{playback.MaxSpeed, playback.MaxSpeed},
		{3, playback.MaxSpeed},
		{0.1, playback.MinSpeed},
		{-4, playback.MinSpeed},
		{playback.Speed(math.NaN()), playback.NormalSpeed},
	}
	for _, tc := range cases {
		if got := tc.from.Faster(); got != tc.want {
			t.Fatalf("Speed(%v).Faster() = %v, want %v", tc.from, got, tc.want)
		}
	}
}

func TestSpeedSlower(t *testing.T) {
	cases := []struct{ from, want playback.Speed }{
		{playback.NormalSpeed, 0.9},
		{0.3, playback.MinSpeed},
		{playback.MinSpeed, playback.MinSpeed},
		{1.1000000001, playback.NormalSpeed},
		{3, playback.MaxSpeed},
		{0, playback.MinSpeed},
		{playback.Speed(math.NaN()), playback.NormalSpeed},
	}
	for _, tc := range cases {
		if got := tc.from.Slower(); got != tc.want {
			t.Fatalf("Speed(%v).Slower() = %v, want %v", tc.from, got, tc.want)
		}
	}
}

// The preset list must be reachable step by step in both directions.
func TestSpeedStepsWalkEveryPresetBothWays(t *testing.T) {
	want := []playback.Speed{0.25, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}
	steps := playback.SpeedSteps()
	if len(steps) != len(want) {
		t.Fatalf("SpeedSteps() = %v, want %v", steps, want)
	}
	current := playback.MinSpeed
	for _, next := range want[1:] {
		if current = current.Faster(); current != next {
			t.Fatalf("walking up reached %v, want %v", current, next)
		}
	}
	for i := len(want) - 2; i >= 0; i-- {
		if current = current.Slower(); current != want[i] {
			t.Fatalf("walking down reached %v, want %v", current, want[i])
		}
	}
}
