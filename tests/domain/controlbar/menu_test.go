package controlbar_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

func studyState() controlbar.StudyState {
	return controlbar.StudyState{
		Snap: true, Options: study.PracticePlan{Mode: study.EndPause},
		Karaoke: false, Indicator: study.IndicatorBar, MinGap: 10 * time.Second, Lead: 5 * time.Second, Fill: true,
		Mode: study.ModeOnDemand, HasPrimary: true, HasTranslation: true, TranslationOn: false,
	}
}

func find(entries []controlbar.Entry, match func(controlbar.Entry) bool) (controlbar.Entry, bool) {
	for _, entry := range entries {
		if match(entry) {
			return entry, true
		}
	}
	return controlbar.Entry{}, false
}

func command(entries []controlbar.Entry, c shortcut.Command) (controlbar.Entry, bool) {
	return find(entries, func(e controlbar.Entry) bool { return e.Kind == controlbar.EntryCommand && e.Command == c })
}

func stepButton(entries []controlbar.Entry, field controlbar.Field, delta int) controlbar.Entry {
	entry, _ := find(entries, func(e controlbar.Entry) bool {
		return e.Kind == controlbar.EntryStepper && e.Field == field && e.Delta == delta
	})
	return entry
}

// The passage menu marks, leaves and resumes a passage, and holds its
// options and the latest saved passages.
func TestThePassageMenu(t *testing.T) {
	s := studyState()
	entries := controlbar.Entries(controlbar.MenuPassage, s)
	if start, ok := command(entries, shortcut.PassageMarkStart); !ok || !start.Enabled {
		t.Fatalf("mark start = %+v %v", start, ok)
	}
	if end, ok := command(entries, shortcut.PassageMarkEnd); !ok || end.Enabled {
		t.Fatalf("mark end without a start = %+v %v, want present and off", end, ok)
	}
	if _, ok := command(entries, shortcut.PassageStop); ok {
		t.Fatal("leave offered with no passage")
	}
	if _, ok := find(entries, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldGap }); ok {
		t.Fatal("the gap shown while the passage pauses at the end")
	}
	if snap, _ := find(entries, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldSnap }); !snap.Checked || snap.Kind != controlbar.EntryToggle {
		t.Fatalf("snap = %+v", snap)
	}

	s.MarkSet, s.Active = true, true
	entries = controlbar.Entries(controlbar.MenuPassage, s)
	if end, _ := command(entries, shortcut.PassageMarkEnd); !end.Enabled {
		t.Fatal("mark end off with a start marked")
	}
	if stop, ok := command(entries, shortcut.PassageStop); !ok || !stop.Enabled {
		t.Fatal("no way to leave the passage being studied")
	}
	s.Suspended = true
	entries = controlbar.Entries(controlbar.MenuPassage, s)
	if _, ok := command(entries, shortcut.PassageResume); !ok {
		t.Fatal("no way back into a suspended passage")
	}
	if _, ok := command(entries, shortcut.PassageStop); ok {
		t.Fatal("leave and resume offered together")
	}

	// Looping: the gap, its buttons at the limits.
	s.Options = study.PracticePlan{Mode: study.EndLoop, Gap: 0, Goal: study.MaxGoal}
	entries = controlbar.Entries(controlbar.MenuPassage, s)
	if down, up := stepButton(entries, controlbar.FieldGap, -1), stepButton(entries, controlbar.FieldGap, +1); down.Enabled || !up.Enabled {
		t.Fatalf("gap 0: down %v up %v", down.Enabled, up.Enabled)
	}
	if down, up := stepButton(entries, controlbar.FieldGoal, -1), stepButton(entries, controlbar.FieldGoal, +1); !down.Enabled || up.Enabled {
		t.Fatalf("goal at the maximum: down %v up %v", down.Enabled, up.Enabled)
	}
	s.Options.Gap = study.MaxLoopGap
	if up := stepButton(controlbar.Entries(controlbar.MenuPassage, s), controlbar.FieldGap, +1); up.Enabled {
		t.Fatal("gap went past its maximum")
	}

	// The latest six saved passages; one of another cut cannot be studied.
	for i := range 8 {
		s.Saved = append(s.Saved, controlbar.SavedItem{Start: time.Duration(i) * time.Minute, End: time.Duration(i)*time.Minute + 10*time.Second, Outside: i == 7})
	}
	var indexes []int
	for _, entry := range controlbar.Entries(controlbar.MenuPassage, s) {
		if entry.Kind == controlbar.EntryPassage {
			indexes = append(indexes, entry.Index)
			if entry.Enabled == (entry.Index == 7) {
				t.Errorf("saved passage %d enabled %v", entry.Index, entry.Enabled)
			}
		}
	}
	if len(indexes) != 6 || indexes[0] != 2 || indexes[5] != 7 {
		t.Fatalf("saved passages listed %v, want the latest six (2..7)", indexes)
	}
	// Without a line on top there is nothing to fit the passage to.
	s.HasPrimary = false
	if snap, _ := find(controlbar.Entries(controlbar.MenuPassage, s), func(e controlbar.Entry) bool { return e.Field == controlbar.FieldSnap }); snap.Enabled {
		t.Error("snap on without a line on top")
	}
}

func TestTheKaraokeMenu(t *testing.T) {
	s := studyState()
	entries := controlbar.Entries(controlbar.MenuKaraoke, s)
	on, _ := find(entries, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldKaraoke })
	if on.Checked || !on.Enabled {
		t.Fatalf("karaoke toggle = %+v", on)
	}
	bar, _ := find(entries, func(e controlbar.Entry) bool {
		return e.Field == controlbar.FieldIndicator && e.Value == string(study.IndicatorBar)
	})
	countdown, _ := find(entries, func(e controlbar.Entry) bool {
		return e.Field == controlbar.FieldIndicator && e.Value == string(study.IndicatorCountdown)
	})
	if !bar.Checked || countdown.Checked || bar.Kind != controlbar.EntryChoice {
		t.Fatalf("indicator choices: bar %+v countdown %+v", bar, countdown)
	}
	for _, c := range []shortcut.Command{shortcut.ReplayLine, shortcut.RepeatPhrase} {
		if entry, ok := command(entries, c); !ok || !entry.Enabled {
			t.Errorf("%s missing or off", c)
		}
	}
	// The timing's limits close their buttons.
	s.MinGap, s.Lead, s.Repeat = study.MinCueGap, study.MaxCueLead, study.MaxRepeats
	entries = controlbar.Entries(controlbar.MenuKaraoke, s)
	if stepButton(entries, controlbar.FieldMinGap, -1).Enabled || !stepButton(entries, controlbar.FieldMinGap, +1).Enabled {
		t.Error("minimum gap at its minimum")
	}
	if stepButton(entries, controlbar.FieldLead, +1).Enabled || !stepButton(entries, controlbar.FieldLead, -1).Enabled {
		t.Error("lead at its maximum")
	}
	if stepButton(entries, controlbar.FieldRepeat, +1).Enabled {
		t.Error("repeats at their maximum")
	}
	// One step inside the limits, the step toward them is still there.
	step, _ := controlbar.Step(controlbar.FieldMinGap)
	s.MinGap, s.Lead = study.MinCueGap+step, study.MaxCueLead-step
	entries = controlbar.Entries(controlbar.MenuKaraoke, s)
	if !stepButton(entries, controlbar.FieldMinGap, -1).Enabled || !stepButton(entries, controlbar.FieldLead, +1).Enabled {
		t.Error("one step inside the limits the timing cannot reach them")
	}
	// Without a line on top there is nothing to preview or repeat.
	s.HasPrimary = false
	entries = controlbar.Entries(controlbar.MenuKaraoke, s)
	if on, _ := find(entries, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldKaraoke }); on.Enabled {
		t.Error("karaoke on without a line on top")
	}
	if replay, _ := command(entries, shortcut.ReplayLine); replay.Enabled {
		t.Error("replay without a line on top")
	}
}

func TestTheSubtitlesMenu(t *testing.T) {
	s := studyState()
	entries := controlbar.Entries(controlbar.MenuSubtitles, s)
	checked := 0
	for _, entry := range entries {
		if entry.Field == controlbar.FieldMode && entry.Checked {
			checked++
			if entry.Value != string(study.ModeOnDemand) {
				t.Errorf("mode %q checked", entry.Value)
			}
		}
	}
	if checked != 1 {
		t.Fatalf("%d modes checked", checked)
	}
	for _, c := range []shortcut.Command{shortcut.SyncPrimaryEarlier, shortcut.SyncPrimaryLater, shortcut.SyncTranslationEarlier, shortcut.SyncTranslationLater} {
		if _, ok := command(entries, c); !ok {
			t.Errorf("%s missing", c)
		}
	}
	edits := 0
	for _, entry := range entries {
		if entry.Field == controlbar.FieldEdit && entry.Kind == controlbar.EntryAction {
			edits++
		}
	}
	if edits != 2 {
		t.Fatalf("%d edit entries, want one per line", edits)
	}
	// A mode that always shows the translation leaves it nothing to toggle.
	s.Mode = study.ModeAlwaysTranslated
	entries = controlbar.Entries(controlbar.MenuSubtitles, s)
	if toggle, _ := find(entries, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldTranslation }); toggle.Enabled {
		t.Error("translation toggle on in the mode that always shows it")
	}
	// Without a translation, none of its entries works.
	s = studyState()
	s.HasTranslation = false
	for _, entry := range controlbar.Entries(controlbar.MenuSubtitles, s) {
		if entry.Enabled && (entry.Command == shortcut.SyncTranslationLater || entry.Field == controlbar.FieldMode ||
			(entry.Field == controlbar.FieldEdit && entry.Value == "translation")) {
			t.Errorf("entry %+v on without a translation", entry)
		}
	}
	if len(controlbar.Entries(controlbar.MenuNone, s)) != 0 {
		t.Fatal("no menu has entries")
	}
}

// An open menu sits above its button, inside the window; its buttons take
// the clicks before its rows, and its titles take none.
func TestAnOpenMenuIsLaidOutAboveItsButton(t *testing.T) {
	state := fullState()
	state.OpenMenu, state.Study = controlbar.MenuKaraoke, studyState()
	bar := controlbar.Layout(wide, 1, state)
	button := menuButton(t, bar, controlbar.MenuKaraoke)
	if !button.Active {
		t.Fatal("the open menu's button is not shown pressed")
	}
	panel := bar.MenuPanel
	if !panel.Valid() || panel.Right() > wide.W || panel.X < 0 || panel.Bottom() > bar.Panel.Y || panel.Y < 0 {
		t.Fatalf("menu panel %+v outside the window or over the bar", panel)
	}
	// Right edge on the button's, as the speed list.
	if math.Abs(panel.Right()-button.Bounds.Right()) > 1e-9 {
		t.Fatalf("panel right %v, button right %v", panel.Right(), button.Bounds.Right())
	}
	if bar.Band.Y > panel.Y {
		t.Fatalf("band from %v leaves out the menu from %v", bar.Band.Y, panel.Y)
	}
	entries := controlbar.Entries(controlbar.MenuKaraoke, state.Study)
	if len(bar.MenuItems) != len(entries) {
		t.Fatalf("%d menu items for %d entries", len(bar.MenuItems), len(entries))
	}
	for _, it := range bar.MenuItems {
		if !it.Bounds.Valid() || it.Bounds.X < panel.X-1e-9 || it.Bounds.Right() > panel.Right()+1e-9 ||
			it.Bounds.Y < panel.Y-1e-9 || it.Bounds.Bottom() > panel.Bottom()+1e-9 {
			t.Errorf("menu item %+v outside its panel %+v", it, panel)
		}
		center := layout.Rect{X: it.Bounds.X + it.Bounds.W/2, Y: it.Bounds.Y + it.Bounds.H/2}
		hit := bar.HitTest(center.X, center.Y)
		switch {
		case it.Action == controlbar.ActionEntryText && it.Entry.Kind != controlbar.EntryStepper && hit.Action != controlbar.ActionNone:
			t.Errorf("a title or line took a click: %+v", hit)
		case it.Action == controlbar.ActionEntry && hit.Entry != it.Entry:
			t.Errorf("center of %+v hit %+v", it.Entry, hit.Entry)
		}
	}
	if !bar.Covers(panel.X+1, panel.Y+1) {
		t.Fatal("the open menu must take the mouse")
	}
	// A click on the bar's buttons still works under an open menu.
	if hit := bar.HitTest(20, 700); hit.Action != controlbar.ActionPlayPause {
		t.Fatalf("play under an open menu hit %+v", hit)
	}
}

func menuButton(t *testing.T, bar controlbar.Bar, menu controlbar.Menu) controlbar.Item {
	t.Helper()
	for _, it := range bar.Items {
		if it.Action == controlbar.ActionMenu && it.Menu == menu {
			return it
		}
	}
	t.Fatalf("no button for menu %d", menu)
	return controlbar.Item{}
}

// In a low window the menu keeps what fits above the bar, from the top: the
// saved passages are the first to go.
func TestAMenuInALowWindowKeepsWhatFits(t *testing.T) {
	state := fullState()
	state.OpenMenu, state.Study = controlbar.MenuPassage, studyState()
	for i := range 6 {
		state.Study.Saved = append(state.Study.Saved, controlbar.SavedItem{Start: time.Duration(i) * time.Minute, End: time.Duration(i)*time.Minute + 5*time.Second})
	}
	tall := controlbar.Layout(wide, 1, state)
	low := controlbar.Layout(layout.Rect{W: 1280, H: 300}, 1, state)
	if len(low.MenuItems) >= len(tall.MenuItems) || len(low.MenuItems) == 0 {
		t.Fatalf("low window kept %d of %d items", len(low.MenuItems), len(tall.MenuItems))
	}
	if low.MenuPanel.Y < 0 || low.MenuPanel.Bottom() > low.Panel.Y {
		t.Fatalf("low menu %+v outside the room above the bar %+v", low.MenuPanel, low.Panel)
	}
	if first := low.MenuItems[0].Entry; first.Command != shortcut.PassageMarkStart {
		t.Fatalf("low menu starts with %+v, want the start mark", first)
	}
	for _, it := range low.MenuItems {
		if it.Entry.Kind == controlbar.EntryPassage {
			t.Fatal("a saved passage stayed while the options did not all fit")
		}
	}
}

func FuzzMenus(f *testing.F) {
	f.Add(1280.0, 720.0, 1.0, 600.0, 400.0, 1, 3)
	f.Add(300.0, 160.0, 2.0, 10.0, 10.0, 3, 0)
	f.Fuzz(func(t *testing.T, w, h, scale, x, y float64, menu, saved int) {
		state := fullState()
		state.OpenMenu = controlbar.Menu(((menu % 4) + 4) % 4)
		state.Study = studyState()
		for i := range ((saved % 10) + 10) % 10 {
			state.Study.Saved = append(state.Study.Saved, controlbar.SavedItem{Start: time.Duration(i) * time.Second, End: time.Duration(i+2) * time.Second})
		}
		area := layout.Rect{W: w, H: h}
		bar := controlbar.Layout(area, scale, state)
		hit := bar.HitTest(x, y)
		if hit.Action != controlbar.ActionNone && !hit.Bounds.Contains(x, y) {
			t.Fatalf("hit %+v does not contain %v,%v", hit, x, y)
		}
		for _, it := range bar.MenuItems {
			if !it.Bounds.Valid() {
				t.Fatalf("menu item %+v has no usable bounds", it)
			}
		}
		if bar.MenuPanel.Valid() && (bar.MenuPanel.Y < area.Y-1e-6 || bar.MenuPanel.Bottom() > bar.Panel.Y+1e-6) {
			t.Fatalf("menu panel %+v outside the room %+v / %+v", bar.MenuPanel, area, bar.Panel)
		}
	})
}
