package controlbar_test

import (
	"math"
	"testing"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/playback"
)

// At scale 1 in a 1280x720 window (sizes in controlbar.go):
//
//	panel: y 720-58 = 662, height 58; inner: x 12, width 1280-2*12 = 1256
//	track: y 662+(16-4)/2 = 668, height 4; its hit area: y 662, height 16
//	buttons row: y 662+16 = 678, height 40
//	left: play 12, back 12+44 = 56, forward 100, mute 144, volume 188 (72
//	      wide, line 192..256), time 188+76 = 264 (124 wide)
//	right: fullscreen 1268-40 = 1228, speed 1224-60 = 1164, translation
//	       1160-40 = 1120
var wide = layout.Rect{W: 1280, H: 720}

func fullState() controlbar.State {
	return controlbar.State{
		Position: 25 * time.Second, Duration: 100 * time.Second, Speed: 1, Volume: 0.5,
		HasTranslation: true, TranslationVisible: true,
		Capabilities: playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true},
	}
}

func item(t *testing.T, bar controlbar.Bar, action controlbar.Action) controlbar.Item {
	t.Helper()
	for _, it := range bar.Items {
		if it.Action == action {
			return it
		}
	}
	t.Fatalf("no item %d in the bar", action)
	return controlbar.Item{}
}

func has(bar controlbar.Bar, action controlbar.Action) bool {
	for _, it := range bar.Items {
		if it.Action == action {
			return true
		}
	}
	return false
}

func TestLayoutPlacesEveryPart(t *testing.T) {
	bar := controlbar.Layout(wide, 1, fullState())
	if bar.Panel != (layout.Rect{X: 0, Y: 662, W: 1280, H: 58}) {
		t.Fatalf("panel = %+v", bar.Panel)
	}
	if bar.Track != (layout.Rect{X: 12, Y: 668, W: 1256, H: 4}) || bar.Played != 0.25 {
		t.Fatalf("track = %+v played %v", bar.Track, bar.Played)
	}
	want := map[controlbar.Action]float64{
		controlbar.ActionPlayPause: 12, controlbar.ActionBack: 56, controlbar.ActionForward: 100,
		controlbar.ActionMute: 144, controlbar.ActionVolume: 188,
		controlbar.ActionFullscreen: 1228, controlbar.ActionSpeed: 1164, controlbar.ActionTranslation: 1120,
	}
	for action, x := range want {
		if got := item(t, bar, action).Bounds; got.X != x || got.Y != 678 || got.H != 40 {
			t.Errorf("item %d at %+v, want x %v, y 678, height 40", action, got, x)
		}
	}
	if bar.Time != (layout.Rect{X: 264, Y: 678, W: 124, H: 40}) {
		t.Fatalf("time = %+v", bar.Time)
	}
	if bar.VolumeTrack != (layout.Rect{X: 192, Y: 678 + 18, W: 64, H: 4}) || bar.VolumeLevel != 0.5 {
		t.Fatalf("volume line = %+v level %v", bar.VolumeTrack, bar.VolumeLevel)
	}
	// The band starts 34 pixels above the panel, for the tips.
	if bar.Band != (layout.Rect{X: 0, Y: 628, W: 1280, H: 92}) {
		t.Fatalf("band = %+v", bar.Band)
	}
}

func TestHitTestEdges(t *testing.T) {
	bar := controlbar.Layout(wide, 1, fullState())
	cases := []struct {
		x, y float64
		want controlbar.Action
	}{
		{12, 678, controlbar.ActionPlayPause},       // top left corner is inside
		{51.99, 717.99, controlbar.ActionPlayPause}, // just before the far edges
		{52, 700, controlbar.ActionNone},            // the gap after play
		{56, 700, controlbar.ActionBack},
		{1228, 700, controlbar.ActionFullscreen},
		{1268, 700, controlbar.ActionNone}, // right padding
		{600, 662, controlbar.ActionTrack},
		{600, 677.99, controlbar.ActionTrack},
		{600, 678, controlbar.ActionNone}, // between the time and the translation
		{600, 661.99, controlbar.ActionNone},
		{math.NaN(), 700, controlbar.ActionNone},
	}
	for _, c := range cases {
		if got := bar.HitTest(c.x, c.y).Action; got != c.want {
			t.Errorf("HitTest(%v, %v) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if !bar.Covers(5, 700) || bar.Covers(5, 661) {
		t.Fatal("Covers must follow the panel")
	}
}

func TestNarrowWindowsDropPartsInOrder(t *testing.T) {
	// Widths needed at scale 1 (inner width = window - 24): everything 528;
	// without the volume line 528-76 = 452; without the jumps 452-88 = 364;
	// without the time 364-128 = 236; without the speed 236-64 = 172; without
	// the translation 172-44 = 128; play and fullscreen alone 84.
	steps := []struct {
		inner   float64
		dropped []controlbar.Action
		kept    []controlbar.Action
	}{
		{528, nil, []controlbar.Action{controlbar.ActionVolume, controlbar.ActionBack}},
		{527, []controlbar.Action{controlbar.ActionVolume}, []controlbar.Action{controlbar.ActionBack, controlbar.ActionForward}},
		{452, []controlbar.Action{controlbar.ActionVolume}, []controlbar.Action{controlbar.ActionBack}},
		{451, []controlbar.Action{controlbar.ActionBack, controlbar.ActionForward}, []controlbar.Action{controlbar.ActionSpeed}},
		{235, []controlbar.Action{controlbar.ActionSpeed}, []controlbar.Action{controlbar.ActionTranslation, controlbar.ActionMute}},
		{171, []controlbar.Action{controlbar.ActionTranslation}, []controlbar.Action{controlbar.ActionMute}},
		{127, []controlbar.Action{controlbar.ActionMute}, []controlbar.Action{controlbar.ActionPlayPause, controlbar.ActionFullscreen, controlbar.ActionTrack}},
		{10, []controlbar.Action{controlbar.ActionMute}, []controlbar.Action{controlbar.ActionPlayPause, controlbar.ActionFullscreen}},
	}
	for _, step := range steps {
		bar := controlbar.Layout(layout.Rect{W: step.inner + 24, H: 400}, 1, fullState())
		for _, action := range step.dropped {
			if has(bar, action) {
				t.Errorf("inner width %v kept item %d", step.inner, action)
			}
		}
		for _, action := range step.kept {
			if !has(bar, action) {
				t.Errorf("inner width %v dropped item %d", step.inner, action)
			}
		}
	}
	// The time goes between 364 and 363.
	if bar := controlbar.Layout(layout.Rect{W: 364 + 24, H: 400}, 1, fullState()); !bar.Time.Valid() {
		t.Error("inner width 364 dropped the time")
	}
	if bar := controlbar.Layout(layout.Rect{W: 363 + 24, H: 400}, 1, fullState()); bar.Time.Valid() {
		t.Error("inner width 363 kept the time")
	}
}

func TestItemsFollowTheCapabilities(t *testing.T) {
	state := fullState()
	state.Capabilities = playback.Capabilities{}
	state.HasTranslation = false
	bar := controlbar.Layout(wide, 1, state)
	for _, action := range []controlbar.Action{controlbar.ActionPlayPause, controlbar.ActionBack, controlbar.ActionMute,
		controlbar.ActionVolume, controlbar.ActionTrack, controlbar.ActionSpeed, controlbar.ActionTranslation} {
		if item(t, bar, action).Enabled {
			t.Errorf("item %d enabled without its capability", action)
		}
	}
	if !item(t, bar, controlbar.ActionFullscreen).Enabled {
		t.Error("fullscreen is the app's own and is always available")
	}
	// The translation toggle is never shown as pressed without a translation.
	state = fullState()
	state.HasTranslation = false
	if item(t, controlbar.Layout(wide, 1, state), controlbar.ActionTranslation).Active {
		t.Error("translation pressed without a translation")
	}
	// A study mode that always shows the translation turns its button off;
	// it still shows the translation as shown.
	state = fullState()
	state.TranslationVisible, state.TranslationLocked = true, true
	if locked := item(t, controlbar.Layout(wide, 1, state), controlbar.ActionTranslation); locked.Enabled || !locked.Active {
		t.Errorf("locked translation = %+v", locked)
	}
	// A track without a known duration cannot seek.
	state = fullState()
	state.Duration = 0
	if item(t, controlbar.Layout(wide, 1, state), controlbar.ActionTrack).Enabled {
		t.Error("track enabled with an unknown duration")
	}
}

func TestToggles(t *testing.T) {
	state := fullState()
	state.Paused, state.Muted, state.Fullscreen = true, true, true
	bar := controlbar.Layout(wide, 1, state)
	if item(t, bar, controlbar.ActionPlayPause).Active || !item(t, bar, controlbar.ActionMute).Active ||
		!item(t, bar, controlbar.ActionFullscreen).Active || bar.VolumeLevel != 0 {
		t.Fatal("paused, muted and fullscreen not shown")
	}
	if got := item(t, bar, controlbar.ActionSpeed).Speed; got != 1 {
		t.Fatalf("speed button carries %v, want the current speed", got)
	}
}

func TestSpeedList(t *testing.T) {
	state := fullState()
	state.SpeedMenuOpen = true
	bar := controlbar.Layout(wide, 1, state)
	// 13 speeds in rows of 3: 5 rows. Width 3*58+2*6 = 186, height 5*28+2*6 =
	// 152; right edge on the speed button's (1224): x 1038; y 662-152-4 = 506.
	if bar.Menu != (layout.Rect{X: 1038, Y: 506, W: 186, H: 152}) {
		t.Fatalf("menu = %+v", bar.Menu)
	}
	choices := 0
	for _, it := range bar.Items {
		if it.Action == controlbar.ActionSpeedChoice {
			choices++
			if it.Active != (it.Speed == 1) {
				t.Errorf("speed %v active = %v", it.Speed, it.Active)
			}
		}
	}
	if choices != len(playback.SpeedSteps()) {
		t.Fatalf("%d choices, want %d", choices, len(playback.SpeedSteps()))
	}
	// First cell (0.25) at 1044,512; the 8th (1.0) is row 2, column 1:
	// 1044+58 = 1102, 512+2*28 = 568.
	if got := bar.HitTest(1050, 515); got.Action != controlbar.ActionSpeedChoice || got.Speed != 0.25 {
		t.Fatalf("first cell hit %+v", got)
	}
	if got := bar.HitTest(1110, 575); got.Speed != 1 {
		t.Fatalf("cell of 1x hit %+v", got)
	}
	if !bar.Covers(1100, 600) {
		t.Fatal("the open list must take the mouse")
	}
	// The band grows to hold the list and the tips above it: 506-34 = 472.
	if bar.Band.Y != 472 {
		t.Fatalf("band top = %v", bar.Band.Y)
	}
	// Without the speed capability the list never opens.
	state.Capabilities.Speed = false
	if bar := controlbar.Layout(wide, 1, state); bar.Menu.Valid() || has(bar, controlbar.ActionSpeedChoice) {
		t.Fatal("speed list opened without the capability")
	}
}

func TestSpeedListStaysInsideASmallWindow(t *testing.T) {
	state := fullState()
	state.SpeedMenuOpen = true
	area := layout.Rect{W: 400, H: 150}
	bar := controlbar.Layout(area, 1, state)
	if !bar.Menu.Valid() || bar.Menu.X < 0 || bar.Menu.Y < 0 || bar.Menu.Right() > 400 || bar.Band.Y < 0 {
		t.Fatalf("menu %+v band %+v outside the window", bar.Menu, bar.Band)
	}
}

func TestTrackPosition(t *testing.T) {
	bar := controlbar.Layout(wide, 1, fullState())
	duration := 100 * time.Second
	cases := []struct {
		x    float64
		want time.Duration
	}{
		{12, 0}, {1268, duration}, {12 + 628, 50 * time.Second}, {-5, 0}, {99999, duration},
		{math.NaN(), 0}, {math.Inf(1), duration}, {math.Inf(-1), 0},
	}
	for _, c := range cases {
		if got := bar.TrackPosition(c.x, duration); got != c.want {
			t.Errorf("TrackPosition(%v) = %v, want %v", c.x, got, c.want)
		}
	}
	if got := bar.TrackPosition(640, 0); got != 0 {
		t.Errorf("unknown duration gave %v", got)
	}
	if got := bar.TrackPosition(640, -time.Second); got != 0 {
		t.Errorf("negative duration gave %v", got)
	}
	if got := (controlbar.Bar{}).TrackPosition(640, duration); got != 0 {
		t.Errorf("a bar with no track gave %v", got)
	}
}

func TestVolumeAt(t *testing.T) {
	bar := controlbar.Layout(wide, 1, fullState())
	// Volume line from 192, 64 wide.
	for x, want := range map[float64]playback.Volume{192: 0, 224: 0.5, 256: 1, 100: 0, 400: 1} {
		if got := bar.VolumeAt(x); got != want {
			t.Errorf("VolumeAt(%v) = %v, want %v", x, got, want)
		}
	}
	if got := bar.VolumeAt(math.NaN()); got != 0 {
		t.Errorf("VolumeAt(NaN) = %v", got)
	}
}

func TestPlayedShare(t *testing.T) {
	for _, c := range []struct {
		position, duration time.Duration
		want               float64
	}{
		{0, time.Minute, 0}, {time.Minute, time.Minute, 1}, {2 * time.Minute, time.Minute, 1},
		{-time.Second, time.Minute, 0}, {time.Second, 0, 0}, {time.Second, -time.Minute, 0},
	} {
		state := fullState()
		state.Position, state.Duration = c.position, c.duration
		if got := controlbar.Layout(wide, 1, state).Played; got != c.want {
			t.Errorf("played(%v of %v) = %v, want %v", c.position, c.duration, got, c.want)
		}
	}
}

func TestScale(t *testing.T) {
	for dpi, want := range map[int]float64{96: 1, 144: 1.5, 192: 2, 0: 1, -96: 1} {
		if got := controlbar.Scale(dpi); got != want {
			t.Errorf("Scale(%d) = %v, want %v", dpi, got, want)
		}
	}
	bar := controlbar.Layout(wide, 2, fullState())
	if bar.Panel.H != 116 || bar.Panel.Y != 720-116 || item(t, bar, controlbar.ActionPlayPause).Bounds.W != 80 {
		t.Fatalf("scale 2: panel %+v", bar.Panel)
	}
}

func TestLayoutRejectsUnusableInput(t *testing.T) {
	for _, c := range []struct {
		area  layout.Rect
		scale float64
	}{
		{layout.Rect{}, 1}, {layout.Rect{W: -10, H: 100}, 1}, {layout.Rect{W: math.NaN(), H: 100}, 1},
		{wide, 0}, {wide, -1}, {wide, math.NaN()}, {wide, math.Inf(1)},
	} {
		if bar := controlbar.Layout(c.area, c.scale, fullState()); len(bar.Items) != 0 || bar.Band.Valid() {
			t.Errorf("Layout(%+v, %v) = %+v, want an empty bar", c.area, c.scale, bar)
		}
	}
}

func TestInZone(t *testing.T) {
	// 720 high: the zone is max(20% of 720, 2*58) = 144 pixels: from 576.
	for y, want := range map[float64]bool{576: true, 575.99: false, 719.99: true, 720: false, 0: false, math.NaN(): false} {
		if got := controlbar.InZone(wide, y, 1); got != want {
			t.Errorf("InZone(%v) = %v, want %v", y, got, want)
		}
	}
	// 200 high: 20% is 40, less than two panels (116): from 84.
	small := layout.Rect{W: 300, H: 200}
	if !controlbar.InZone(small, 84, 1) || controlbar.InZone(small, 83.9, 1) {
		t.Error("a small window's zone must be two panels high")
	}
	if controlbar.InZone(layout.Rect{}, 0, 1) {
		t.Error("an empty window has no zone")
	}
}

func FuzzHitTest(f *testing.F) {
	f.Add(1280.0, 720.0, 1.0, 600.0, 700.0, true)
	f.Add(10.0, 10.0, 3.0, -1.0, 5.0, false)
	f.Fuzz(func(t *testing.T, w, h, scale, x, y float64, menu bool) {
		state := fullState()
		state.SpeedMenuOpen = menu
		bar := controlbar.Layout(layout.Rect{W: w, H: h}, scale, state)
		hit := bar.HitTest(x, y)
		if hit.Action != controlbar.ActionNone && !hit.Bounds.Contains(x, y) {
			t.Fatalf("hit %+v does not contain %v,%v", hit, x, y)
		}
		if got := bar.TrackPosition(x, time.Hour); got < 0 || got > time.Hour {
			t.Fatalf("TrackPosition = %v", got)
		}
		if v := bar.VolumeAt(x); !v.Valid() {
			t.Fatalf("VolumeAt = %v", v)
		}
		for _, it := range bar.Items {
			if !it.Bounds.Valid() {
				t.Fatalf("item %+v has no usable bounds", it)
			}
		}
	})
}
