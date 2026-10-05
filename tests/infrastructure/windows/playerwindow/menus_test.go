package playerwindow_test

import (
	"math"
	"slices"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// menuRig is a player window whose bar shows, driving a fake session with
// the Modo Karaokê's defaults and two saved passages.
type menuRig struct {
	t      *testing.T
	w      *playerwindow.Window
	host   windows.HWND
	bar    windows.HWND
	remote *fakeRemote
	// adjust changes the study state the tests lay the bar out with, to match
	// values given to the fake session.
	adjust func(*controlbar.StudyState)
}

func newMenuRig(t *testing.T) *menuRig {
	t.Helper()
	return newMenuRigFor(t, &recordingPlayer{})
}

// framePlayer is a player that steps frames both ways.
type framePlayer struct{ *recordingPlayer }

func (p framePlayer) Capabilities() playback.Capabilities {
	capabilities := p.recordingPlayer.Capabilities()
	capabilities.FrameStepForward, capabilities.FrameStepBackward = true, true
	return capabilities
}

func newMenuRigFor(t *testing.T, p session.Player) *menuRig {
	t.Helper()
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		// Tall enough for the longest menu with its saved passages.
		Title: ptbr.WindowTitle("v.mkv"), Placement: player.Placement{X: -3000, Y: 100, Width: 1280, Height: 720}, VideoWidth: 1280, VideoHeight: 720,
		Labels: playerwindow.Labels{
			Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime,
			Entry: ptbr.BarEntries(nil), Status: ptbr.BarStatus,
		},
		NoActivate: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	remote := newRemoteFor(p)
	remote.mode = study.ModeOnDemand
	remote.plan, remote.snap = study.PracticePlan{Mode: study.EndPause}, true
	remote.karaoke = session.Karaoke{Rules: study.DefaultCueRules(), Fill: true}
	remote.indicator = study.IndicatorBar
	remote.saved = []watch.SavedPassage{
		{Name: "Loja", Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second}, Plan: study.PracticePlan{Mode: study.EndPause}},
		{Name: "Rua", Segment: study.Segment{Start: 30 * time.Second, End: 34 * time.Second}, Plan: study.PracticePlan{Mode: study.EndLoop}},
	}
	w.Attach(remote)
	r := &menuRig{t: t, w: w, host: windows.HWND(w.Handle()), remote: remote}
	r.bar = controlBar(r.host)
	r.hover()
	return r
}

// hover puts the mouse in the bar's zone and waits for the bar.
func (r *menuRig) hover() {
	r.t.Helper()
	post(r.t, r.host, win32.WM_MOUSEMOVE, 0, at(640, 700))
	testkit.Eventually(r.t, "the bar to show", func() bool { return windows.IsWindowVisible(r.bar) })
}

// layout is the bar as the window lays it out for a state of the session.
func (r *menuRig) layout(open controlbar.Menu) controlbar.Bar {
	r.t.Helper()
	client, err := win32.ClientRectOnScreen(r.host)
	if err != nil {
		r.t.Fatal(err)
	}
	area := layout.Rect{W: float64(client.Right - client.Left), H: float64(client.Bottom - client.Top)}
	state := controlbar.State{
		Duration: 100 * time.Second, Speed: 1, HasTranslation: true, TranslationVisible: true,
		Capabilities: r.remote.Controls().Capabilities(), OpenMenu: open,
		Study: controlbar.StudyState{
			Snap: true, Options: study.PracticePlan{Mode: study.EndPause}, Saved: make([]controlbar.SavedItem, len(r.remote.SavedPassages())),
			Indicator: study.IndicatorBar, MinGap: study.DefaultCueRules().MinGap, Lead: study.DefaultCueRules().Lead, Fill: true,
			Mode: study.ModeOnDemand, HasPrimary: true, HasTranslation: true,
		},
	}
	if r.adjust != nil {
		r.adjust(&state.Study)
	}
	return controlbar.Layout(area, controlbar.Scale(win32.DpiForWindow(r.host)), state)
}

// closeMenu closes the open menu with Esc and brings the bar back.
func (r *menuRig) closeMenu() {
	r.t.Helper()
	closed := r.layout(controlbar.MenuNone)
	post(r.t, r.host, win32.WM_KEYDOWN, 0, key(scanEscape, false, false))
	testkit.Eventually(r.t, "the menu to close", func() bool {
		return !windows.IsWindowVisible(r.bar) || r.barHeight() <= math.Ceil(closed.Band.H)
	})
	r.hover()
}

// click presses the bar at the center of a rectangle of the host's client
// area, in the bar window's own pixels.
func (r *menuRig) click(bar controlbar.Bar, bounds layout.Rect) {
	r.t.Helper()
	x := bounds.X + bounds.W/2 - bar.Band.X
	y := bounds.Y + bounds.H/2 - math.Floor(bar.Band.Y)
	post(r.t, r.bar, win32.WM_LBUTTONDOWN, 1, at(int(x), int(y)))
}

func (r *menuRig) button(bar controlbar.Bar, menu controlbar.Menu) layout.Rect {
	r.t.Helper()
	for _, it := range bar.Items {
		if it.Action == controlbar.ActionMenu && it.Menu == menu {
			return it.Bounds
		}
	}
	r.t.Fatalf("no button for menu %d (bar %v wide)", menu, bar.Panel.W)
	return layout.Rect{}
}

func (r *menuRig) entry(bar controlbar.Bar, match func(controlbar.Entry) bool) layout.Rect {
	r.t.Helper()
	for _, it := range bar.MenuItems {
		if it.Action == controlbar.ActionEntry && match(it.Entry) {
			return it.Bounds
		}
	}
	r.t.Fatal("no such entry in the open menu")
	return layout.Rect{}
}

// open opens a menu by its button and waits for the bar to grow to hold it.
func (r *menuRig) open(menu controlbar.Menu) controlbar.Bar {
	r.t.Helper()
	closed := r.layout(controlbar.MenuNone)
	r.click(closed, r.button(closed, menu))
	opened := r.layout(menu)
	testkit.Eventually(r.t, "the menu to open", func() bool {
		rect, err := win32.WindowRect(r.bar)
		return err == nil && float64(rect.Bottom-rect.Top) >= math.Floor(opened.Band.H)
	})
	return opened
}

func (r *menuRig) barHeight() float64 {
	rect, err := win32.WindowRect(r.bar)
	if err != nil {
		r.t.Fatal(err)
	}
	return float64(rect.Bottom - rect.Top)
}

func waitFor(t *testing.T, what string, list func() []string, want string) {
	t.Helper()
	testkit.Eventually(t, what, func() bool { return slices.Contains(list(), want) })
}

// The passage menu marks a passage and changes its options; a command
// closes the menu, an option keeps it open for the next one.
func TestThePassageMenuDrivesTheSession(t *testing.T) {
	testkit.Slow(t)
	r := newMenuRig(t)
	bar := r.open(controlbar.MenuPassage)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Command == "passage-mark-start" }))
	waitFor(t, "the start mark", r.remote.ran, "passage-mark-start")
	closed := r.layout(controlbar.MenuNone)
	testkit.Eventually(t, "the menu to close after a command", func() bool { return r.barHeight() <= math.Ceil(closed.Band.H) })

	// Options keep the menu open; the goal changes a value, not the rows.
	bar = r.open(controlbar.MenuPassage)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldGoal && e.Delta > 0 }))
	testkit.Eventually(t, "the goal up", func() bool { plan, _ := r.remote.PassageOptions(); return plan.Goal == 1 })
	if r.barHeight() < math.Floor(bar.Band.H) {
		t.Fatal("an option closed the menu")
	}
	// The latest saved passage studies as it was kept, and closes the menu.
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Kind == controlbar.EntryPassage && e.Index == 1 }))
	waitFor(t, "the saved passage", r.remote.changed, "study saved 1")
	testkit.Eventually(t, "the menu to close after studying", func() bool { return r.barHeight() <= math.Ceil(closed.Band.H) })

	// Looping on (last: it adds the gap's row to the menu).
	bar = r.open(controlbar.MenuPassage)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldLoop }))
	testkit.Eventually(t, "looping on", func() bool { plan, _ := r.remote.PassageOptions(); return plan.Mode == study.EndLoop })
}

func TestTheKaraokeMenuDrivesTheSession(t *testing.T) {
	r := newMenuRig(t)
	bar := r.open(controlbar.MenuKaraoke)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldKaraoke }))
	testkit.Eventually(t, "the Modo Karaokê on", func() bool { return r.remote.Karaoke().Enabled })
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool {
		return e.Field == controlbar.FieldIndicator && e.Value == string(study.IndicatorCountdown)
	}))
	testkit.Eventually(t, "the countdown", func() bool { return r.remote.Indicator() == study.IndicatorCountdown })
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldMinGap && e.Delta < 0 }))
	testkit.Eventually(t, "the gap one second shorter", func() bool {
		return r.remote.Karaoke().Rules.MinGap == study.DefaultCueRules().MinGap-time.Second
	})
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldRepeat && e.Delta > 0 }))
	testkit.Eventually(t, "a repeat", func() bool { return r.remote.LineRepeats() == 1 })
}

func TestTheSubtitlesMenuDrivesTheSession(t *testing.T) {
	r := newMenuRig(t)
	bar := r.open(controlbar.MenuSubtitles)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool {
		return e.Field == controlbar.FieldMode && e.Value == string(study.ModeDual)
	}))
	testkit.Eventually(t, "the dual mode", func() bool { return r.remote.Mode() == study.ModeDual })
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Command == "sync-translation-later" }))
	waitFor(t, "a sync step", r.remote.ran, "sync-translation-later")
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldResetSync }))
	waitFor(t, "the sync zeroed", r.remote.changed, "reset sync 1")
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldEdit && e.Value == "primary" }))
	waitFor(t, "the editor", r.remote.changed, "editor 0")
}

// Esc and a click on the video close an open menu without running anything.
func TestAMenuClosesWithEscOrAClickOnTheVideo(t *testing.T) {
	r := newMenuRig(t)
	closed := r.layout(controlbar.MenuNone)
	for _, close := range []func(){
		func() { post(t, r.host, win32.WM_KEYDOWN, 0, key(scanEscape, false, false)) },
		func() { post(t, r.host, win32.WM_LBUTTONDOWN, 1, at(320, 100)) },
	} {
		r.open(controlbar.MenuKaraoke)
		close()
		testkit.Eventually(t, "the menu to close", func() bool {
			return !windows.IsWindowVisible(r.bar) || r.barHeight() <= math.Ceil(closed.Band.H)
		})
		r.hover()
	}
	if got := r.remote.ran(); len(got) != 0 {
		t.Fatalf("closing a menu ran %q", got)
	}
	if got := r.remote.changed(); len(got) != 0 {
		t.Fatalf("closing a menu changed %q", got)
	}
}

// The frame buttons step one frame each way through the session; with a
// player that cannot, they are off and do nothing.
func TestTheFrameButtonsStepFrames(t *testing.T) {
	testkit.Slow(t)
	off := newMenuRig(t)
	bar := off.layout(controlbar.MenuNone)
	for _, it := range bar.Items {
		if it.Action == controlbar.ActionFrameBack || it.Action == controlbar.ActionFrameForward {
			off.click(bar, it.Bounds)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if got := off.remote.ran(); len(got) != 0 {
		t.Fatalf("frame buttons of a player without frames ran %q", got)
	}

	r := newMenuRigFor(t, framePlayer{&recordingPlayer{}})
	bar = r.layout(controlbar.MenuNone)
	for _, action := range []controlbar.Action{controlbar.ActionFrameBack, controlbar.ActionFrameForward} {
		found := false
		for _, it := range bar.Items {
			if it.Action == action {
				r.click(bar, it.Bounds)
				found = true
			}
		}
		if !found {
			t.Fatalf("no button %d in a bar %v wide", action, bar.Panel.W)
		}
	}
	testkit.Eventually(t, "both frame steps", func() bool { return len(r.remote.ran()) >= 2 })
	if got := r.remote.ran(); !slices.Equal(got, []string{"frame-back", "frame-forward"}) {
		t.Fatalf("ran %q", got)
	}
}

// A menu row changes only what it names, inside its limits; a saved passage
// of a longer cut is listed but cannot be studied.
func TestTheMenusChangeOnlyWhatTheyName(t *testing.T) {
	testkit.Slow(t)
	r := newMenuRig(t)
	r.remote.mu.Lock()
	r.remote.repeats = 3
	r.remote.plan = study.PracticePlan{} // no end mode yet
	r.remote.saved = append(r.remote.saved, watch.SavedPassage{Name: "Créditos", Segment: study.Segment{Start: 200 * time.Second, End: 210 * time.Second}})
	r.remote.mu.Unlock()
	r.adjust = func(s *controlbar.StudyState) { s.Repeat = 3 }
	rules := study.DefaultCueRules()

	bar := r.open(controlbar.MenuKaraoke)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldRepeat && e.Delta < 0 }))
	testkit.Eventually(t, "one repeat less", func() bool { return r.remote.LineRepeats() == 2 })
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldFill }))
	testkit.Eventually(t, "the fill off and nothing else", func() bool { return r.remote.Karaoke() == session.Karaoke{Rules: rules} })

	// The passage past the end does nothing and the menu stays; an option set
	// on a plan with no end mode gives it the one that pauses.
	r.closeMenu()
	bar = r.open(controlbar.MenuPassage)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Kind == controlbar.EntryPassage && e.Index == 2 }))
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldProgressive }))
	testkit.Eventually(t, "progressive on", func() bool {
		plan, _ := r.remote.PassageOptions()
		return plan == study.PracticePlan{Mode: study.EndPause, Progressive: true}
	})
	if slices.Contains(r.remote.changed(), "study saved 2") {
		t.Fatal("a saved passage past the end of the video was studied")
	}

	// Half a step below its maximum, the gap goes up to it, not past it.
	r.closeMenu()
	near := study.PracticePlan{Mode: study.EndLoop, Gap: study.MaxLoopGap - 500*time.Millisecond}
	r.remote.mu.Lock()
	r.remote.plan = near
	r.remote.mu.Unlock()
	r.adjust = func(s *controlbar.StudyState) { s.Repeat, s.Options = 3, near }
	bar = r.open(controlbar.MenuPassage)
	r.click(bar, r.entry(bar, func(e controlbar.Entry) bool { return e.Field == controlbar.FieldGap && e.Delta > 0 }))
	testkit.Eventually(t, "the gap at its maximum", func() bool {
		plan, _ := r.remote.PassageOptions()
		return plan == study.PracticePlan{Mode: study.EndLoop, Gap: study.MaxLoopGap}
	})
}
