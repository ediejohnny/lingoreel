package contextmenu_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// fullState is a session with everything: a player that does all, two
// lines, three subtitle tracks and two audio tracks, inside the app.
func fullState() contextmenu.State {
	return contextmenu.State{
		Bar: controlbar.State{
			Paused: true, Speed: 1,
			Capabilities: playback.Capabilities{
				Pause: true, Seek: true, Speed: true, FrameStepForward: true, FrameStepBackward: true,
				AudioTrackSelection: true, Volume: true, MaxVolume: 2,
			},
			Study: controlbar.StudyState{
				HasPrimary: true, HasTranslation: true, Mode: study.ModeDual, Indicator: study.IndicatorBar,
				MinGap: 10 * time.Second, Lead: 5 * time.Second, Options: study.PracticePlan{Mode: study.EndPause},
				Saved: []controlbar.SavedItem{{Start: time.Minute, End: 2 * time.Minute}},
			},
		},
		Tracks: []media.Track{
			{Number: 2, Type: media.TrackAudio, Language: "en"},
			{Number: 3, Type: media.TrackAudio, Language: "pt"},
			{Number: 4, Type: media.TrackSubtitle, Language: "en", Readable: true},
			{Number: 5, Type: media.TrackSubtitle, Language: "pt-BR", Readable: true},
			{Number: 6, Type: media.TrackSubtitle, Language: "pt", Codec: "S_HDMV/PGS"},
		},
		Current: media.Selection{Audio: 2, Primary: 4, Translation: 5},
		App:     true,
	}
}

// walk visits every item, submenus' children included.
func walk(items []contextmenu.Item, visit func(item contextmenu.Item, siblings []contextmenu.Item, index int)) {
	for i, item := range items {
		visit(item, items, i)
		walk(item.Children, visit)
	}
}

// find returns the first item that matches, anywhere in the menu.
func find(items []contextmenu.Item, match func(contextmenu.Item) bool) (contextmenu.Item, bool) {
	var found contextmenu.Item
	ok := false
	walk(items, func(item contextmenu.Item, _ []contextmenu.Item, _ int) {
		if !ok && match(item) {
			found, ok = item, true
		}
	})
	return found, ok
}

func submenu(t *testing.T, items []contextmenu.Item, title contextmenu.Title) contextmenu.Item {
	t.Helper()
	item, ok := find(items, func(item contextmenu.Item) bool { return item.Kind == contextmenu.KindSubmenu && item.Title == title })
	if !ok {
		t.Fatalf("no submenu %d", title)
	}
	return item
}

func commandItem(items []contextmenu.Item, command shortcut.Command) (contextmenu.Item, bool) {
	return find(items, func(item contextmenu.Item) bool {
		return item.Action == contextmenu.ActionCommand && item.Command == command
	})
}

// The menu is tidy at every level, and every item does something it can
// be worded by.
func TestTheMenuIsTidyAndEveryItemSaysWhatItDoes(t *testing.T) {
	t.Parallel()
	for _, state := range []contextmenu.State{fullState(), {}} {
		menu := contextmenu.Build(state)
		if len(menu) == 0 {
			t.Fatal("an empty menu")
		}
		walk(menu, func(item contextmenu.Item, siblings []contextmenu.Item, i int) {
			if item.Kind == contextmenu.KindSeparator && (i == 0 || i == len(siblings)-1 || siblings[i-1].Kind == contextmenu.KindSeparator) {
				t.Errorf("a separator at %d of %d, or after another", i, len(siblings))
			}
			if item.Kind == contextmenu.KindItem && item.Action == contextmenu.ActionCommand && !item.Command.Known() {
				t.Errorf("a command item without a known command: %+v", item)
			}
			if item.Kind == contextmenu.KindSubmenu && item.Title == contextmenu.TitleNone {
				t.Errorf("a submenu without a title: %+v", item)
			}
			if (item.Radio || item.Kind == contextmenu.KindItem && item.Action == contextmenu.ActionEntry && item.Entry.Kind == controlbar.EntryToggle) && item.Icon != contextmenu.IconNone {
				t.Errorf("an item with a check mark also has an icon: %+v", item)
			}
		})
	}
}

// The speeds: exactly one checked; a speed between the presets joins them
// checked; a speed that is no number checks none.
func TestTheSpeedListChecksTheCurrentSpeed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		speed   playback.Speed
		checked int
		joined  bool
	}{
		{1, 1, false},
		{0.65, 1, true},
		{playback.Speed(math.NaN()), 0, false},
	} {
		state := fullState()
		state.Bar.Speed = c.speed
		speeds := submenu(t, contextmenu.Build(state), contextmenu.TitleSpeed)
		checked, found := 0, false
		for _, item := range speeds.Children {
			if item.Action != contextmenu.ActionSpeed {
				continue
			}
			if item.Checked {
				checked++
			}
			found = found || math.Abs(float64(item.Speed-c.speed)) < 1e-9
		}
		if checked != c.checked {
			t.Errorf("speed %v: %d checked, want %d", c.speed, checked, c.checked)
		}
		if c.joined && !found {
			t.Errorf("speed %v is not in the list", c.speed)
		}
	}
}

// A ready-made list takes the current value when it is none of them, and
// leaves out a value past the field's limits.
func TestReadyMadeValuesTakeTheCurrentOneInsideTheLimits(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.Bar.Study.MinGap = 11 * time.Second
	gaps := submenu(t, contextmenu.Build(state), contextmenu.TitleMinGap)
	var eleven contextmenu.Item
	for _, item := range gaps.Children {
		if item.Duration == 11*time.Second {
			eleven = item
		}
		if item.Duration < study.MinCueGap || item.Duration > study.MaxCueGap {
			t.Errorf("a min gap of %v is offered, out of the limits", item.Duration)
		}
	}
	if !eleven.Checked || eleven.Field != controlbar.FieldMinGap || eleven.Action != contextmenu.ActionPreset {
		t.Fatalf("the current 11 s = %+v, want a checked preset of the min gap", eleven)
	}

	state.Bar.Study.MinGap = 999 * time.Second
	for _, item := range submenu(t, contextmenu.Build(state), contextmenu.TitleMinGap).Children {
		if item.Duration == 999*time.Second || item.Checked {
			t.Fatalf("a current value past the limits shows: %+v", item)
		}
	}
}

// What the player cannot do is shown unavailable, never hidden.
func TestWhatThePlayerCannotDoIsUnavailable(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.Bar.Capabilities = playback.Capabilities{Pause: true, Seek: true}
	menu := contextmenu.Build(state)
	for _, command := range []shortcut.Command{shortcut.FrameBack, shortcut.FrameForward, shortcut.SpeedUp, shortcut.VolumeUp, shortcut.Mute} {
		item, found := commandItem(menu, command)
		if !found || item.Enabled {
			t.Errorf("%s: found %v, enabled %v; want it shown unavailable", command, found, item.Enabled)
		}
	}
	if speeds := submenu(t, menu, contextmenu.TitleSpeed); speeds.Enabled {
		t.Error("the speed list is enabled for a player without speeds")
	}
	for _, item := range submenu(t, menu, contextmenu.TitleAudioTrack).Children {
		if item.Enabled {
			t.Errorf("an audio track can be picked on a player that cannot switch: %+v", item)
		}
	}
	if seek, _ := commandItem(menu, shortcut.SeekBack); !seek.Enabled {
		t.Error("the seek the player can do is unavailable")
	}
}

// Without a translation its items are unavailable, and "none" is checked
// below; the list on top never offers none.
func TestWithoutATranslationItsItemsAreUnavailable(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.Bar.Study.HasTranslation = false
	state.Current.Translation = 0
	menu := contextmenu.Build(state)
	for _, command := range []shortcut.Command{shortcut.SyncTranslationEarlier, shortcut.SyncTranslationLater, shortcut.ShowBoth} {
		if item, _ := commandItem(menu, command); item.Enabled {
			t.Errorf("%s is enabled without a translation", command)
		}
	}
	below := submenu(t, menu, contextmenu.TitleTranslation)
	if first := below.Children[0]; first.Action != contextmenu.ActionSubtitle || first.Track.Number != 0 || !first.Checked {
		t.Fatalf("first item below = %+v, want none, checked", first)
	}
	for _, item := range submenu(t, menu, contextmenu.TitlePrimary).Children {
		if item.Action == contextmenu.ActionSubtitle && item.Track.Number == 0 {
			t.Fatal("the line on top offers none")
		}
		if item.Action == contextmenu.ActionSubtitle && item.Track.Number == 6 && item.Enabled {
			t.Fatal("a subtitle in pictures can be picked")
		}
	}
}

// A click on a line puts its own items first; away from one they are not there.
func TestALineUnderTheClickComesFirst(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.Under = &contextmenu.Under{Line: contextmenu.LineTranslation, Text: "Olá."}
	menu := contextmenu.Build(state)
	if menu[0].Action != contextmenu.ActionEditHere || menu[1].Action != contextmenu.ActionCopyHere || menu[1].Text != "Olá." ||
		menu[2].Action != contextmenu.ActionAppearance || menu[2].Page != contextmenu.PageTranslation || menu[3].Kind != contextmenu.KindSeparator {
		t.Fatalf("first items = %+v", menu[:4])
	}
	if first := contextmenu.Build(fullState())[0]; first.Action != contextmenu.ActionCommand || first.Command != shortcut.PlayPause {
		t.Fatalf("away from a line the menu starts with %+v, want play", first)
	}
}

// The development tool has no app: the app's own items are not there.
func TestWithoutTheAppItsItemsAreGone(t *testing.T) {
	t.Parallel()
	state := fullState()
	state.App = false
	state.Under = &contextmenu.Under{Line: contextmenu.LinePrimary, Text: "Hi."}
	menu := contextmenu.Build(state)
	for _, action := range []contextmenu.Action{contextmenu.ActionStopVideo, contextmenu.ActionOpenVideo, contextmenu.ActionSettings,
		contextmenu.ActionAddSubtitle, contextmenu.ActionAppearance, contextmenu.ActionExport} {
		if item, found := find(menu, func(item contextmenu.Item) bool { return item.Action == action }); found {
			t.Errorf("an app item is there without the app: %+v", item)
		}
	}
	if item, found := find(menu, func(item contextmenu.Item) bool { return item.Action == contextmenu.ActionClose }); !found || !item.Enabled {
		t.Error("closing the window needs no app, and is gone")
	}
}

// A hostile file with a thousand tracks does not make a thousand items.
func TestATrackListHasACeiling(t *testing.T) {
	t.Parallel()
	state := fullState()
	for i := range 1000 {
		state.Tracks = append(state.Tracks, media.Track{Number: uint64(100 + i), Type: media.TrackSubtitle, Readable: true})
	}
	top := submenu(t, contextmenu.Build(state), contextmenu.TitlePrimary)
	tracks := 0
	for _, item := range top.Children {
		if item.Action == contextmenu.ActionSubtitle {
			tracks++
		}
	}
	if tracks == 0 || tracks > 50 {
		t.Fatalf("%d tracks listed, want a ceiling", tracks)
	}
}
