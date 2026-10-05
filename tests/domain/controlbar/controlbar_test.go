package controlbar_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// The tests check how the parts relate (order, no overlap, inside the panel,
// a click finds its item), not the pixel each one sits at, so the sizes in
// controlbar.go can change without them.
var wide = layout.Rect{W: 1280, H: 720}

func fullState() controlbar.State {
	return controlbar.State{
		Position: 25 * time.Second, Duration: 100 * time.Second, Speed: 1, Volume: 0.5,
		HasTranslation: true, TranslationVisible: true,
		Capabilities: playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true, FrameStepForward: true, FrameStepBackwardEmulated: true},
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

func inside(outer, inner layout.Rect) bool {
	return inner.X >= outer.X && inner.Right() <= outer.Right() && inner.Y >= outer.Y && inner.Bottom() <= outer.Bottom()
}

func overlap(a, b layout.Rect) bool {
	return a.X < b.Right() && b.X < a.Right() && a.Y < b.Bottom() && b.Y < a.Bottom()
}

func center(r layout.Rect) (float64, float64) { return r.X + r.W/2, r.Y + r.H/2 }

func TestTheBarIsLaidOutInOrder(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	if bar.Panel.X != 0 || bar.Panel.W != wide.W || bar.Panel.Bottom() != wide.H || !(bar.Panel.H > 0) {
		t.Fatalf("panel = %+v, want the window's whole bottom", bar.Panel)
	}
	// The band holds the panel and the tips above it.
	if !(bar.Band.Y < bar.Panel.Y) || bar.Band.Bottom() != bar.Panel.Bottom() {
		t.Fatalf("band = %+v, panel %+v", bar.Band, bar.Panel)
	}
	if bar.Played != 0.25 {
		t.Fatalf("played = %v, want 25 s of 100", bar.Played)
	}

	parts := []layout.Rect{bar.Time, bar.Status}
	for _, it := range bar.Items {
		if !inside(bar.Panel, it.Bounds) {
			t.Errorf("item %d at %+v leaves the panel %+v", it.Action, it.Bounds, bar.Panel)
		}
		parts = append(parts, it.Bounds)
	}
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			if overlap(parts[i], parts[j]) {
				t.Errorf("%+v overlaps %+v", parts[i], parts[j])
			}
		}
	}

	row := []layout.Rect{
		item(t, bar, controlbar.ActionPlayPause).Bounds, item(t, bar, controlbar.ActionBack).Bounds,
		item(t, bar, controlbar.ActionForward).Bounds, item(t, bar, controlbar.ActionFrameBack).Bounds,
		item(t, bar, controlbar.ActionFrameForward).Bounds, item(t, bar, controlbar.ActionMute).Bounds,
		item(t, bar, controlbar.ActionVolume).Bounds, bar.Time, bar.Status,
		menuButton(t, bar, controlbar.MenuPassage).Bounds, menuButton(t, bar, controlbar.MenuKaraoke).Bounds,
		menuButton(t, bar, controlbar.MenuSubtitles).Bounds, item(t, bar, controlbar.ActionTranslation).Bounds,
		item(t, bar, controlbar.ActionSpeed).Bounds, item(t, bar, controlbar.ActionFullscreen).Bounds,
	}
	for i := 1; i < len(row); i++ {
		if !row[i].Valid() || row[i].X < row[i-1].Right() || row[i].Y != row[0].Y || row[i].H != row[0].H {
			t.Errorf("part %d of the row at %+v, after %+v", i, row[i], row[i-1])
		}
	}
	// The track runs above the buttons; the volume line sits in its button.
	track := item(t, bar, controlbar.ActionTrack).Bounds
	if !inside(track, bar.Track) || track.Bottom() > row[0].Y {
		t.Fatalf("track %+v in its hit area %+v, buttons from y %v", bar.Track, track, row[0].Y)
	}
	if volume := item(t, bar, controlbar.ActionVolume).Bounds; !inside(volume, bar.VolumeTrack) || bar.VolumeLevel != 0.5 {
		t.Fatalf("volume line %+v level %v in %+v", bar.VolumeTrack, bar.VolumeLevel, volume)
	}
}

// Every item takes the clicks at its center, at its top-left corner and just
// inside its far edges; the gaps and the padding take none.
func TestHitTestFindsEachItem(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	for _, it := range bar.Items {
		b := it.Bounds
		cx, cy := center(b)
		for _, p := range [][2]float64{{cx, cy}, {b.X, b.Y}, {b.Right() - 0.01, b.Bottom() - 0.01}} {
			if got := bar.HitTest(p[0], p[1]); got.Action != it.Action || got.Menu != it.Menu {
				t.Errorf("HitTest(%v, %v) = %d, want item %d", p[0], p[1], got.Action, it.Action)
			}
		}
	}
	play, back, full := item(t, bar, controlbar.ActionPlayPause), item(t, bar, controlbar.ActionBack), item(t, bar, controlbar.ActionFullscreen)
	_, y := center(play.Bounds)
	for _, x := range []float64{(play.Bounds.Right() + back.Bounds.X) / 2, play.Bounds.Right(), full.Bounds.Right(), math.NaN()} {
		if got := bar.HitTest(x, y); got.Action != controlbar.ActionNone {
			t.Errorf("HitTest(%v, %v) = %d, want nothing", x, y, got.Action)
		}
	}
	if !bar.Covers(5, y) || bar.Covers(5, bar.Panel.Y-0.01) {
		t.Fatal("Covers must follow the panel")
	}
}

// As the window narrows, the parts leave in a fixed order and never come
// back; play and fullscreen always stay.
func TestNarrowWindowsDropPartsInOrder(t *testing.T) {
	t.Parallel()
	present := func(action controlbar.Action) func(controlbar.Bar) bool {
		return func(bar controlbar.Bar) bool { return has(bar, action) }
	}
	parts := []struct {
		name    string
		group   int // parts of a group leave together
		present func(controlbar.Bar) bool
	}{
		{"study text", 0, func(bar controlbar.Bar) bool { return bar.Status.Valid() }},
		{"volume", 1, present(controlbar.ActionVolume)},
		{"frame back", 2, present(controlbar.ActionFrameBack)},
		{"frame forward", 2, present(controlbar.ActionFrameForward)},
		{"back", 3, present(controlbar.ActionBack)},
		{"forward", 3, present(controlbar.ActionForward)},
		{"time", 4, func(bar controlbar.Bar) bool { return bar.Time.Valid() }},
		{"speed", 5, present(controlbar.ActionSpeed)},
		{"translation", 6, present(controlbar.ActionTranslation)},
		{"mute", 7, present(controlbar.ActionMute)},
		{"menus", 8, present(controlbar.ActionMenu)},
	}
	gone := make([]float64, len(parts)) // the widest window without the part
	for width := 1300.0; width >= 10; width-- {
		bar := controlbar.Layout(layout.Rect{W: width, H: 400}, 1, fullState())
		if !has(bar, controlbar.ActionPlayPause) || !has(bar, controlbar.ActionFullscreen) {
			t.Fatalf("width %v lost play or fullscreen", width)
		}
		for i, part := range parts {
			switch shown := part.present(bar); {
			case gone[i] == 0 && !shown:
				gone[i] = width
			case gone[i] != 0 && shown:
				t.Fatalf("%s came back at width %v after leaving at %v", part.name, width, gone[i])
			}
		}
	}
	for i, part := range parts {
		if gone[i] == 0 {
			t.Errorf("%s never left", part.name)
			continue
		}
		if i == 0 {
			continue
		}
		together := part.group == parts[i-1].group
		if together && gone[i] != gone[i-1] || !together && !(gone[i] < gone[i-1]) {
			t.Errorf("%s left at width %v, %s at %v", parts[i-1].name, gone[i-1], part.name, gone[i])
		}
	}
}

// The frame buttons follow the player: one forward, and one back when the
// player steps back or the session emulates it with an exact seek.
func TestFrameButtonsFollowThePlayer(t *testing.T) {
	t.Parallel()
	state := fullState()
	bar := controlbar.Layout(wide, 1, state)
	if !item(t, bar, controlbar.ActionFrameBack).Enabled || !item(t, bar, controlbar.ActionFrameForward).Enabled {
		t.Fatal("frame buttons off with the capabilities")
	}
	state.Capabilities.FrameStepForward, state.Capabilities.FrameStepBackwardEmulated = false, false
	bar = controlbar.Layout(wide, 1, state)
	if item(t, bar, controlbar.ActionFrameBack).Enabled || item(t, bar, controlbar.ActionFrameForward).Enabled {
		t.Fatal("frame buttons on without the capabilities")
	}
	state.Capabilities.FrameStepBackward = true
	if !item(t, controlbar.Layout(wide, 1, state), controlbar.ActionFrameBack).Enabled {
		t.Fatal("a player stepping back did not turn the button on")
	}
}

func TestItemsFollowTheCapabilities(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// The speed list opens above its button, inside the window, with every speed
// and the current one marked; a click on a cell picks its speed.
func TestSpeedList(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.SpeedMenuOpen = true
	bar := controlbar.Layout(wide, 1, state)
	speed := item(t, bar, controlbar.ActionSpeed)
	if !inside(wide, bar.Menu) || bar.Menu.Bottom() > bar.Panel.Y || bar.Menu.Right() != speed.Bounds.Right() {
		t.Fatalf("list %+v, panel %+v, speed button %+v", bar.Menu, bar.Panel, speed.Bounds)
	}
	if !(bar.Band.Y < bar.Menu.Y) {
		t.Fatalf("band top %v leaves no room for the tips above the list at %v", bar.Band.Y, bar.Menu.Y)
	}
	choices := 0
	for _, it := range bar.Items {
		if it.Action != controlbar.ActionSpeedChoice {
			continue
		}
		choices++
		if !inside(bar.Menu, it.Bounds) || it.Active != (it.Speed == 1) {
			t.Errorf("speed %v at %+v, active %v", it.Speed, it.Bounds, it.Active)
		}
		if got := bar.HitTest(center(it.Bounds)); got.Action != controlbar.ActionSpeedChoice || got.Speed != it.Speed {
			t.Errorf("the cell of %v picked %+v", it.Speed, got)
		}
	}
	if choices != len(playback.SpeedSteps()) {
		t.Fatalf("%d choices, want %d", choices, len(playback.SpeedSteps()))
	}
	if !bar.Covers(center(bar.Menu)) {
		t.Fatal("the open list must take the mouse")
	}
	// Without the speed capability the list never opens.
	state.Capabilities.Speed = false
	if bar := controlbar.Layout(wide, 1, state); bar.Menu.Valid() || has(bar, controlbar.ActionSpeedChoice) {
		t.Fatal("speed list opened without the capability")
	}
}

func TestSpeedListStaysInsideASmallWindow(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.SpeedMenuOpen = true
	area := layout.Rect{W: 400, H: 150}
	bar := controlbar.Layout(area, 1, state)
	if !bar.Menu.Valid() || bar.Menu.X < 0 || bar.Menu.Y < 0 || bar.Menu.Right() > 400 || bar.Band.Y < 0 {
		t.Fatalf("menu %+v band %+v outside the window", bar.Menu, bar.Band)
	}
}

func TestTrackPosition(t *testing.T) {
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	duration := 100 * time.Second
	track := bar.Track
	cases := []struct {
		x    float64
		want time.Duration
	}{
		{track.X, 0}, {track.Right(), duration}, {track.X + track.W/2, 50 * time.Second}, {-5, 0}, {99999, duration},
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
	t.Parallel()
	bar := controlbar.Layout(wide, 1, fullState())
	line := bar.VolumeTrack
	for x, want := range map[float64]playback.Volume{line.X: 0, line.X + line.W/2: 0.5, line.Right(): 1, line.X - 50: 0, line.Right() + 50: 1} {
		if got := bar.VolumeAt(x); got != want {
			t.Errorf("VolumeAt(%v) = %v, want %v", x, got, want)
		}
	}
	if got := bar.VolumeAt(math.NaN()); got != 0 {
		t.Errorf("VolumeAt(NaN) = %v", got)
	}
}

func TestPlayedShare(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	for dpi, want := range map[int]float64{96: 1, 144: 1.5, 192: 2, 0: 1, -96: 1} {
		if got := controlbar.Scale(dpi); got != want {
			t.Errorf("Scale(%d) = %v, want %v", dpi, got, want)
		}
	}
	// At 200% everything is twice as big.
	one, two := controlbar.Layout(wide, 1, fullState()), controlbar.Layout(wide, 2, fullState())
	if two.Panel.H != 2*one.Panel.H || two.Panel.Bottom() != wide.H ||
		item(t, two, controlbar.ActionPlayPause).Bounds.W != 2*item(t, one, controlbar.ActionPlayPause).Bounds.W {
		t.Fatalf("scale 2: panel %+v, at scale 1 %+v", two.Panel, one.Panel)
	}
}

func TestLayoutRejectsUnusableInput(t *testing.T) {
	t.Parallel()
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

// The bar shows when the mouse is in the bottom fifth of the video, or in
// the two panels' height at the bottom when that is taller.
func TestInZone(t *testing.T) {
	t.Parallel()
	for _, area := range []layout.Rect{wide, {W: 300, H: 200}} {
		top := area.H - math.Max(0.2*area.H, 2*controlbar.PanelHeight(1))
		for y, want := range map[float64]bool{top: true, top - 0.01: false, area.H - 0.01: true, area.H: false, 0: false, math.NaN(): false} {
			if got := controlbar.InZone(area, y, 1); got != want {
				t.Errorf("InZone(%v high, %v) = %v, want %v", area.H, y, got, want)
			}
		}
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
