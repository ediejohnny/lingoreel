package playback_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

func TestFrameDurationUsesTheRealRate(t *testing.T) {
	cases := []struct {
		fps  float64
		want time.Duration
	}{
		{24, 41_666_667 * time.Nanosecond},
		{23.976, 41_708_375 * time.Nanosecond},
		{24000.0 / 1001, 41_708_333 * time.Nanosecond}, // NTSC film rate, as written by mkvmerge
		{25, 40 * time.Millisecond},
		{30, 33_333_333 * time.Nanosecond},
		{29.97, 33_366_700 * time.Nanosecond},
		{60, 16_666_667 * time.Nanosecond},
		{playback.MinFrameRate, time.Second},
		{playback.MaxFrameRate, time.Millisecond},
	}
	for _, tc := range cases {
		got, ok := playback.FrameDuration(tc.fps)
		if !ok || got != tc.want {
			t.Fatalf("FrameDuration(%v) = %v,%v; want %v", tc.fps, got, ok, tc.want)
		}
	}
}

func TestFrameDurationRefusesUnknownRates(t *testing.T) {
	for _, fps := range []float64{0, -24, 0.5, playback.MaxFrameRate + 1, math.NaN(), math.Inf(1)} {
		if got, ok := playback.FrameDuration(fps); ok {
			t.Fatalf("FrameDuration(%v) = %v; an unknown rate must not be assumed", fps, got)
		}
	}
}
