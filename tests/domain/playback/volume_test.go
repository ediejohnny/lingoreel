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

// The volume keys (user report: 1% a touch and about 9% a second held were
// far too slow): a touch is 5% at once, as the usual players do; held,
// nothing more for the first 400 ms, then 1% every 50 ms by the clock (about
// 20% a second) whatever the keyboard's repeat rate, all the way to the
// player's ceiling.
func TestTheVolumeRampIsGradual(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return start.Add(time.Duration(ms) * time.Millisecond) }
	percent := func(v playback.Volume) int { return v.Percent() }

	// Held from 50% with a fast keyboard (a repeat every 30 ms) for 1.5 s.
	var ramp playback.VolumeRamp
	level := ramp.Next(at(0), true, false, 0.5, 2)
	if percent(level) != 55 {
		t.Fatalf("a touch = %d%%, want 55%%", percent(level))
	}
	for ms := 30; ms < 400; ms += 30 {
		if level = ramp.Next(at(ms), true, true, level, 2); percent(level) != 55 {
			t.Fatalf("held %d ms = %d%%, want 55%% until 400 ms", ms, percent(level))
		}
	}
	for ms := 420; ms <= 1500; ms += 30 {
		level = ramp.Next(at(ms), true, true, level, 2)
	}
	// The last repeat is at 1500 ms: 1 + 1100/50 = 23 steps.
	if percent(level) != 78 {
		t.Fatalf("held 1.5 s from 50%% = %d%%, want 78%%", percent(level))
	}

	// A slow keyboard (a repeat every 400 ms) reaches the same level.
	var slow playback.VolumeRamp
	level = slow.Next(at(0), true, false, 0.5, 2)
	for ms := 400; ms <= 1600; ms += 400 {
		level = slow.Next(at(ms), true, true, level, 2)
	}
	if got := percent(level); got < 76 || got > 80 {
		t.Fatalf("slow repeats for 1.6 s = %d%%, want close to the fast keyboard's", got)
	}

	// Held up from 99% it goes past 100% without stopping, to the player's
	// ceiling. The touch makes 104%; the last repeat before 3 s is at 2980 ms,
	// with 1 + (2980-400)/50 = 1 + 51 = 52 steps due: 156%.
	var up playback.VolumeRamp
	level = up.Next(at(0), true, false, 0.99, 2)
	for ms := 400; ms <= 3000; ms += 30 {
		level = up.Next(at(ms), true, true, level, 2)
	}
	if percent(level) != 156 {
		t.Fatalf("held up from 99%% for 3 s = %d%%, want 156%%", percent(level))
	}
	for ms := 3030; ms <= 20000; ms += 30 {
		level = up.Next(at(ms), true, true, level, 2)
	}
	if percent(level) != 200 {
		t.Fatalf("held up from 99%% = %d%%, want the ceiling", percent(level))
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

// The touch is the user's choice, from 1% to 25%; anything else is the
// default 5%. Holding the key keeps its own pace whatever the touch.
func TestTheVolumeTouchIsTheUsersChoice(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		touch playback.Volume
		want  int
	}{
		{0.02, 52}, {0.25, 75}, {0.01, 51},
		{0, 55}, {0.3, 55}, {-0.1, 55}, {playback.Volume(math.NaN()), 55},
	} {
		ramp := playback.VolumeRamp{Touch: c.touch}
		if got := ramp.Next(start, true, false, 0.5, 2).Percent(); got != c.want {
			t.Errorf("touch %v: %d%%, want %d%%", c.touch, got, c.want)
		}
	}
	if !playback.ValidTouch(0) || !playback.ValidTouch(0.01) || !playback.ValidTouch(0.25) ||
		playback.ValidTouch(0.26) || playback.ValidTouch(-0.01) || playback.ValidTouch(playback.Volume(math.NaN())) {
		t.Fatal("ValidTouch takes zero and 1% to 25% only")
	}
}
