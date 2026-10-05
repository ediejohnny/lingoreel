package playback_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// A volume goes from 0 to 2 (200%); outside it is invalid and clamps to the
// nearest end. Not a number is silent: never louder than asked.
func TestVolumeBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      playback.Volume
		valid   bool
		clamped playback.Volume
	}{
		{0, true, 0},
		{0.5, true, 0.5},
		{1, true, 1},
		{2, true, 2},
		{-0.01, false, 0},
		{2.01, false, 2},
		{playback.Volume(math.Inf(1)), false, 2},
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

// A player's ceiling keeps the level below it, rounded to a whole percent.
func TestVolumeWithinACeiling(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, ceiling, want playback.Volume }{
		{1.5, 1, 1}, {1.5, 2, 1.5}, {0.123, 2, 0.12}, {0.126, 1, 0.13}, {3, 5, 2}, {0.5, -1, 0},
		{playback.Volume(math.NaN()), 2, 0},
	} {
		if got := c.in.Within(c.ceiling); got != c.want {
			t.Errorf("Volume(%v).Within(%v) = %v, want %v", c.in, c.ceiling, got, c.want)
		}
	}
	if (playback.Capabilities{}).VolumeCeiling() != 1 || (playback.Capabilities{MaxVolume: 9}).VolumeCeiling() != 2 {
		t.Fatal("a player's ceiling is 100% unless declared, and never past 200%")
	}
}

// The volume keys: a touch is 1% at once; held, nothing more for the first
// 400 ms, then 1% every 110 ms by the clock (about 9% a second) whatever
// the keyboard's repeat rate; going up, a held key stops at 100%.
func TestTheVolumeRampIsGradual(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return start.Add(time.Duration(ms) * time.Millisecond) }
	percent := func(v playback.Volume) int { return v.Percent() }

	// Held from 50% with a fast keyboard (a repeat every 30 ms) for 1.5 s.
	var ramp playback.VolumeRamp
	level := ramp.Next(at(0), true, false, 0.5, 2)
	if percent(level) != 51 {
		t.Fatalf("a touch = %d%%, want 51%%", percent(level))
	}
	for ms := 30; ms < 400; ms += 30 {
		if level = ramp.Next(at(ms), true, true, level, 2); percent(level) != 51 {
			t.Fatalf("held %d ms = %d%%, want 51%% until 400 ms", ms, percent(level))
		}
	}
	for ms := 420; ms <= 1500; ms += 30 {
		level = ramp.Next(at(ms), true, true, level, 2)
	}
	// From 400 to 1500 ms: 1 + 1100/110 = 11 steps.
	if percent(level) != 62 {
		t.Fatalf("held 1.5 s from 50%% = %d%%, want 62%%", percent(level))
	}

	// A slow keyboard (a repeat every 400 ms) reaches about the same level.
	var slow playback.VolumeRamp
	level = slow.Next(at(0), true, false, 0.5, 2)
	for ms := 400; ms <= 1600; ms += 400 {
		level = slow.Next(at(ms), true, true, level, 2)
	}
	if got := percent(level); got < 58 || got > 62 {
		t.Fatalf("slow repeats for 1.6 s = %d%%, want close to the fast keyboard's", got)
	}

	// Held up from 99% it stops at 100%; a new touch passes it.
	var up playback.VolumeRamp
	level = up.Next(at(0), true, false, 0.99, 2)
	for ms := 400; ms <= 3000; ms += 30 {
		level = up.Next(at(ms), true, true, level, 2)
	}
	if percent(level) != 100 {
		t.Fatalf("held up from 99%% = %d%%, want 100%%", percent(level))
	}
	if level = up.Next(at(3100), true, false, level, 2); percent(level) != 101 {
		t.Fatalf("a new touch at 100%% = %d%%, want 101%%", percent(level))
	}
	// Held from there it goes on, up to the player's ceiling.
	for ms := 3500; ms <= 20000; ms += 30 {
		level = up.Next(at(ms), true, true, level, 2)
	}
	if percent(level) != 200 {
		t.Fatalf("held up from 101%% = %d%%, want the ceiling", percent(level))
	}

	// Down has no stop, and a player of 100% never goes above it.
	var down playback.VolumeRamp
	level = down.Next(at(0), false, false, 1.2, 2)
	for ms := 400; ms <= 4000; ms += 30 {
		level = down.Next(at(ms), false, true, level, 2)
	}
	if percent(level) >= 100 {
		t.Fatalf("held down from 120%% for 4 s = %d%%, want below 100%%", percent(level))
	}
	var low playback.VolumeRamp
	if level = low.Next(at(0), true, false, 1, 1); percent(level) != 100 {
		t.Fatalf("a touch up at a 100%% player's top = %d%%", percent(level))
	}
}
