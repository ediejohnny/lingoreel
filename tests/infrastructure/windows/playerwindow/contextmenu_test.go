package playerwindow_test

import (
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// fakeActions is the app behind the menu: it records what it was asked.
type fakeActions struct {
	mu    sync.Mutex
	calls []string
	// stop runs inside StopVideo, as the app's own end of the video does.
	stop func()
}

func (a *fakeActions) record(call string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, call)
}

func (a *fakeActions) asked() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.calls)
}

func (a *fakeActions) OpenSettings(page string)            { a.record("settings " + page) }
func (a *fakeActions) SwitchSubtitle(session.Line, uint64) { a.record("subtitle") }
func (a *fakeActions) SwitchAudio(uint64)                  { a.record("audio") }
func (a *fakeActions) AddSubtitle(session.Line)            { a.record("add") }
func (a *fakeActions) ExportSubtitle(session.Line)         { a.record("export") }
func (a *fakeActions) CopyText(text string)                { a.record("copy " + text) }
func (a *fakeActions) OpenVideo()                          { a.record("open") }
func (a *fakeActions) StopVideo() {
	a.record("stop")
	if a.stop != nil {
		a.stop()
	}
}

// menuWindow is a player window whose menu never shows on screen: pick
// stands for the user, choosing in the menu built (a real HMENU) at the
// point given.
type menuWindow struct {
	w      *playerwindow.Window
	host   windows.HWND
	remote *fakeRemote
	mu     sync.Mutex
	pick   func(menu windows.Handle, x, y int32) uint32
	opened []win32.Point
}

func openMenuWindow(t *testing.T, app playerwindow.AppActions) *menuWindow {
	t.Helper()
	m := &menuWindow{}
	options := playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), Placement: offScreen, VideoWidth: 1280, VideoHeight: 720,
		Look:       overlay.Look{Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle()},
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, Menu: ptbr.MenuTexts(nil)},
		NoActivate: true,
		Choose: func(menu uintptr, x, y int32) uint32 {
			m.mu.Lock()
			m.opened = append(m.opened, win32.Point{X: x, Y: y})
			pick := m.pick
			m.mu.Unlock()
			if pick == nil {
				return 0
			}
			return pick(windows.Handle(menu), x, y)
		},
	}
	if app != nil {
		options.App = app
	}
	w, err := playerwindow.Open(t.Context(), options)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	m.w, m.host = w, windows.HWND(w.Handle())
	m.remote = newRemote(&recordingPlayer{})
	m.remote.mode = study.ModeOnDemand
	w.Attach(m.remote)
	return m
}

func (m *menuWindow) choose(pick func(menu windows.Handle, x, y int32) uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pick = pick
}

func (m *menuWindow) opens() []win32.Point {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.opened)
}

// openAt asks for the menu as a right click at a point on screen would.
func (m *menuWindow) openAt(t *testing.T, x, y int32) {
	t.Helper()
	post(t, m.host, win32.WM_CONTEXTMENU, uintptr(m.host), at(int(x), int(y)))
}

// itemAt finds an item by the texts on the way to it (the audio submenu, mute),
// each compared without its key; ok is false when there is none.
func itemAt(menu windows.Handle, path ...string) (win32.MenuItem, bool) {
	for depth, name := range path {
		found := false
		for i := range win32.MenuItemCount(menu) {
			item, err := win32.MenuItemAt(menu, i)
			if err != nil || item.Separator {
				continue
			}
			text, _, _ := strings.Cut(item.Text, "\t")
			if text != name {
				continue
			}
			if depth == len(path)-1 {
				return item, true
			}
			menu, found = item.SubMenu, true
			break
		}
		if !found {
			return win32.MenuItem{}, false
		}
	}
	return win32.MenuItem{}, false
}

// picking chooses the item at a path; the test fails when it is not there.
func picking(t *testing.T, path ...string) func(windows.Handle, int32, int32) uint32 {
	return func(menu windows.Handle, _, _ int32) uint32 {
		item, ok := itemAt(menu, path...)
		if !ok || item.Disabled {
			t.Errorf("no item to choose at %q (found %v, disabled %v)", path, ok, item.Disabled)
			return 0
		}
		return item.ID
	}
}

// A window opened on the app's stage takes the app's actions from the
// stage, as the app opens its windows: its menu has the app's items, and
// they reach the app.
func TestAWindowOnTheStageReachesTheApp(t *testing.T) {
	actions := &fakeActions{}
	stage := playerwindow.NewStage(playerwindow.StageOptions{
		Parent: newFakeApp(t, offScreenApp, true).handle,
		App:    func() playerwindow.AppActions { return actions },
	})
	stage.SetLayout(playerwindow.StageLayout{Visible: true, Insets: panelOpen})
	settings := make(chan bool, 1)
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), VideoWidth: 1280, VideoHeight: 720,
		Look:       overlay.Look{Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle()},
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, Menu: ptbr.MenuTexts(nil)},
		NoActivate: true, Stage: stage,
		Choose: func(menu uintptr, _, _ int32) uint32 {
			item, found := itemAt(windows.Handle(menu), ptbr.T("menu.settings"))
			settings <- found
			return item.ID
		},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	w.Attach(newRemote(&recordingPlayer{}))
	host := windows.HWND(w.Handle())
	post(t, host, win32.WM_CONTEXTMENU, uintptr(host), at(10, 10))
	select {
	case found := <-settings:
		if !found {
			t.Fatal("the menu of a window on the stage has none of the app's items")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the menu did not open")
	}
	testkit.Eventually(t, "the settings asked of the app", func() bool { return slices.Contains(actions.asked(), "settings general") })
}

// A right click opens the menu where the mouse is; the menu key opens it
// at the center of the video.
func TestTheMenuOpensWhereAsked(t *testing.T) {
	m := openMenuWindow(t, nil)
	client, err := win32.ClientRectOnScreen(m.host)
	if err != nil {
		t.Fatal(err)
	}
	m.openAt(t, client.Left+30, client.Top+40)
	testkit.Eventually(t, "the menu at the click", func() bool { return len(m.opens()) == 1 })
	post(t, m.host, win32.WM_KEYDOWN, win32.VK_APPS, 1)
	testkit.Eventually(t, "the menu from the menu key", func() bool { return len(m.opens()) == 2 })
	center := win32.Point{X: (client.Left + client.Right) / 2, Y: (client.Top + client.Bottom) / 2}
	if got := m.opens(); got[0] != (win32.Point{X: client.Left + 30, Y: client.Top + 40}) || got[1] != center {
		t.Fatalf("the menu opened at %v, want the click and then %v", got, center)
	}
}

// What is chosen in the menu reaches the session: a study mode, a speed, a
// sync step, a ready-made value and the sound.
func TestMenuChoicesReachTheSession(t *testing.T) {
	m := openMenuWindow(t, nil)
	cases := []struct {
		path []string
		done func() bool
	}{
		{[]string{ptbr.T("menu.title.studyMode"), ptbr.ModeName(study.ModeDual)}, func() bool { return slices.Contains(m.remote.changed(), "mode "+string(study.ModeDual)) }},
		{[]string{ptbr.T("menu.title.subtitles"), ptbr.T("menu.title.sync"), ptbr.CommandName(shortcut.SyncTranslationLater)}, func() bool { return slices.Contains(m.remote.ran(), "sync-translation-later") }},
		{[]string{ptbr.T("menu.title.karaoke"), ptbr.T("bar.stepper.minGap"), ptbr.Seconds(15 * time.Second)}, func() bool { return m.remote.Karaoke().Rules.MinGap == 15*time.Second }},
		{[]string{ptbr.T("menu.title.audio"), ptbr.T("menu.mute")}, func() bool { return slices.Contains(m.remote.ran(), "mute") }},
	}
	m.remote.karaoke.Rules = study.DefaultCueRules()
	for i, c := range cases {
		m.choose(picking(t, c.path...))
		m.openAt(t, 10, 10)
		testkit.Eventually(t, strings.Join(c.path, " › "), c.done)
		testkit.Eventually(t, "the menu to have opened", func() bool { return len(m.opens()) == i+1 })
	}
	// A speed goes to the player itself.
	player := &recordingPlayer{}
	m.remote = newRemote(player)
	m.w.Attach(m.remote)
	m.choose(picking(t, ptbr.T("menu.title.playback"), ptbr.T("menu.title.speed"), ptbr.Speed(1.25)))
	m.openAt(t, 10, 10)
	testkit.Eventually(t, "the speed asked of the player", func() bool { return player.saw("speed") })
}

// Each command shows the key the profile gives it, as changed; a command
// without a key shows none.
func TestTheMenuShowsTheKeysAsConfigured(t *testing.T) {
	m := openMenuWindow(t, nil)
	keys := shortcut.Defaults()
	keys[shortcut.PlayPause] = shortcut.Chord{Key: "P"}
	keys[shortcut.Mute] = shortcut.Chord{}
	m.w.SetShortcuts(keys)
	texts := make(chan [2]string, 1)
	m.choose(func(menu windows.Handle, _, _ int32) uint32 {
		pause, _ := itemAt(menu, ptbr.T("bar.tip.pause"))
		mute, _ := itemAt(menu, ptbr.T("menu.title.audio"), ptbr.T("menu.mute"))
		texts <- [2]string{pause.Text, mute.Text}
		return 0
	})
	m.openAt(t, 10, 10)
	select {
	case got := <-texts:
		if got != [2]string{ptbr.T("bar.tip.pause") + "\tP", ptbr.T("menu.mute")} {
			t.Fatalf("items read %q, want the changed key and none", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the menu did not open")
	}
}

// The pictures are as large as the monitor's small icons; the menu goes
// once closed, and the pictures with the window.
func TestTheMenuAndItsPicturesAreFreed(t *testing.T) {
	m := openMenuWindow(t, nil)
	type seen struct {
		menu, bitmap windows.Handle
	}
	saw := make(chan seen, 1)
	m.choose(func(menu windows.Handle, _, _ int32) uint32 {
		pause, _ := itemAt(menu, ptbr.T("bar.tip.pause"))
		saw <- seen{menu, pause.Bitmap}
		return 0
	})
	m.openAt(t, 10, 10)
	var got seen
	select {
	case got = <-saw:
	case <-time.After(5 * time.Second):
		t.Fatal("the menu did not open")
	}
	// The monitor's small icon size, asked of Windows by the test itself.
	small, _, _ := procGetSystemMetricsForDpi.Call(smCXSmallIcon, uintptr(win32.DpiForWindow(m.host)))
	size := int(small)
	if width, height, bits, ok := win32.BitmapBits(got.bitmap); !ok || width != size || height != size || bits != 32 {
		t.Fatalf("the picture is %dx%d at %d bits (%v), want %dx%d at 32", width, height, bits, ok, size, size)
	}
	testkit.Eventually(t, "the menu to be destroyed", func() bool { return !win32.IsMenu(got.menu) })
	if err := m.w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := win32.BitmapBits(got.bitmap); ok {
		t.Fatal("a menu picture outlived the window")
	}
}

// A click on a subtitle puts its own items first, and copying sends its
// text to the app.
func TestALineUnderTheClickCanBeCopied(t *testing.T) {
	app := &fakeActions{}
	m := openMenuWindow(t, app)
	m.remote.visible = session.Visible{Primary: []subtitle.Event{{ID: "e1", OriginalText: "Hello there."}}}
	if err := m.w.Show(session.Screen{Primary: "Hello there.", Translation: "Olá.", TranslationVisible: true}); err != nil {
		t.Fatal(err)
	}
	var box struct{ X, Y float64 }
	testkit.Eventually(t, "the line drawn", func() bool {
		bounds, ok := m.w.LineBounds(session.LinePrimary)
		box.X, box.Y = bounds.X+bounds.W/2, bounds.Y+bounds.H/2
		return ok
	})
	client, err := win32.ClientRectOnScreen(m.host)
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan string, 1)
	m.choose(func(menu windows.Handle, x, y int32) uint32 {
		item, _ := win32.MenuItemAt(menu, 0)
		first <- item.Text
		return picking(t, ptbr.T("menu.copyHere"))(menu, x, y)
	})
	m.openAt(t, client.Left+int32(box.X), client.Top+int32(box.Y))
	select {
	case text := <-first:
		if text != ptbr.T("menu.editHere") {
			t.Fatalf("the menu over a line starts with %q", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the menu did not open")
	}
	testkit.Eventually(t, "the line copied", func() bool { return slices.Contains(app.asked(), "copy Hello there.") })
}

// The development tool has no app: its items are not in the menu, and
// closing the window still is.
func TestWithoutTheAppItsItemsAreNotInTheMenu(t *testing.T) {
	m := openMenuWindow(t, nil)
	found := make(chan [2]bool, 1)
	m.choose(func(menu windows.Handle, _, _ int32) uint32 {
		_, settings := itemAt(menu, ptbr.T("menu.settings"))
		_, closing := itemAt(menu, ptbr.T("menu.close"))
		found <- [2]bool{settings, closing}
		return 0
	})
	m.openAt(t, 10, 10)
	select {
	case got := <-found:
		if got != [2]bool{false, true} {
			t.Fatalf("settings in the menu %v, closing %v; want only closing", got[0], got[1])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the menu did not open")
	}
}

var procPeekMessage = windows.NewLazySystemDLL("user32.dll").NewProc("PeekMessageW")

// procGetSystemMetricsForDpi and smCXSmallIcon read the width of a small
// icon at a scaling (SM_CXSMICON).
var procGetSystemMetricsForDpi = windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetricsForDpi")

const smCXSmallIcon = 49

// pumpUntil dispatches the thread's messages, as a menu's own loop does,
// until done is closed or the time runs out.
func pumpUntil(done <-chan struct{}, limit time.Duration) bool {
	const pmRemove = 0x0001
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return true
		default:
		}
		var msg win32.Msg
		if ok, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmRemove); ok != 0 {
			win32.DispatchMessage(&msg)
			continue
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// A window closed while its menu shows (the video ended meanwhile) takes
// the menu with it, and the item chosen does not run.
func TestAWindowClosedWithItsMenuOpenDropsTheChoice(t *testing.T) {
	m := openMenuWindow(t, nil)
	closed := make(chan error, 1)
	m.choose(func(menu windows.Handle, x, y int32) uint32 {
		mute := picking(t, ptbr.T("menu.title.audio"), ptbr.T("menu.mute"))(menu, x, y)
		go func() { closed <- m.w.Close() }()
		if !pumpUntil(m.w.Done(), 3*time.Second) {
			t.Error("the window did not go while its menu showed")
		}
		return mute
	})
	m.openAt(t, 10, 10)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close never returned")
	}
	if slices.Contains(m.remote.ran(), "mute") {
		t.Fatal("the item chosen in the menu of a closed window ran")
	}
}

// menuShowing tells whether a popup menu shows on screen (its window class
// is "#32768").
func menuShowing() bool {
	class, _ := windows.UTF16PtrFromString("#32768")
	menu, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(class)), 0)
	return menu != 0 && windows.IsWindowVisible(windows.HWND(menu))
}

var procFindWindow = windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")

// The real menu, on screen (only with LINGOREEL_MENU_TEST=1: it shows
// on the monitor for a moment, so nothing else may be in use), opened as the
// app opens it, on the stage of an app window with the app's actions: what
// closes a menu (a click elsewhere cancels the mode) closes it, the window
// then opens it again, and a window closed while its menu shows goes well
// within the time a stuck thread is given.
func TestARealMenuClosesAndLetsTheWindowGo(t *testing.T) {
	if os.Getenv("LINGOREEL_MENU_TEST") != "1" {
		t.Skip("set LINGOREEL_MENU_TEST=1 to show a real menu on screen")
	}
	stage := playerwindow.NewStage(playerwindow.StageOptions{
		Parent: newFakeApp(t, offScreenApp, true).handle,
		App:    func() playerwindow.AppActions { return &fakeActions{} },
	})
	stage.SetLayout(playerwindow.StageLayout{Visible: true, Insets: panelOpen})
	w, err := playerwindow.Open(t.Context(), playerwindow.Options{
		Title: ptbr.WindowTitle("v.mkv"), VideoWidth: 1280, VideoHeight: 720,
		Labels:     playerwindow.Labels{Tip: ptbr.BarTips(nil), Item: ptbr.BarItem, Time: ptbr.BarTime, Menu: ptbr.MenuTexts(nil)},
		NoActivate: true, Stage: stage,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	host := windows.HWND(w.Handle())
	w.Attach(newRemote(&recordingPlayer{}))
	client, err := win32.ClientRectOnScreen(host)
	if err != nil {
		t.Fatal(err)
	}
	const wmCancelMode = 0x001F
	for round := range 2 {
		post(t, host, win32.WM_CONTEXTMENU, uintptr(host), at(int(client.Left+20), int(client.Top+20)))
		testkit.Eventually(t, "the menu on screen", menuShowing)
		if round == 0 {
			post(t, host, wmCancelMode, 0, 0)
			testkit.Eventually(t, "the menu closed", func() bool { return !menuShowing() })
		}
	}
	started := time.Now()
	if err := w.Close(); err != nil {
		t.Fatalf("Close with the menu open: %v", err)
	}
	if took := time.Since(started); took > time.Second {
		t.Fatalf("closing with the menu open took %v", took)
	}
	testkit.Eventually(t, "the menu gone with the window", func() bool { return !menuShowing() })
}

// Ending the video from the menu runs off the window's thread: the app's
// end of the session closes the window, which waits on that thread.
func TestEndingTheVideoFromTheMenuDoesNotHang(t *testing.T) {
	app := &fakeActions{}
	m := openMenuWindow(t, app)
	closed := make(chan error, 1)
	app.stop = func() { closed <- m.w.Close() }
	m.choose(picking(t, ptbr.T("menu.stopVideo")))
	m.openAt(t, 10, 10)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("closing the window from the app's end of the video: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ending the video from the menu hung")
	}
}
