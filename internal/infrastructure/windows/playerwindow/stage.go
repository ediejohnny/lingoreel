//go:build windows && amd64

package playerwindow

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The stage is the place for the video inside the app's own window: there
// is one window, the app's, and the player draws into a part of it. The
// app's controls take the rest (its header on top, its panel beside), and
// they tell the stage how much room they take; the video goes fullscreen
// by making the app's window cover the monitor.

// ErrNoAppWindow reports a stage asked to open a window before the app's
// own window exists.
var ErrNoAppWindow = errors.New("the app window is not there")

// Stage is the place for the video inside the app's window. It outlives the
// sessions: each window opened on it takes its layout. Safe for concurrent
// use.
type Stage struct {
	options StageOptions

	mu      sync.Mutex
	layout  StageLayout
	current *Window
}

// StageOptions configures a stage.
type StageOptions struct {
	// Parent is the app's window, where the video goes.
	Parent func() uintptr
	// OnFullscreen is told each time the video goes fullscreen or leaves
	// it, whatever asked for it: a key, the bar, two clicks or the app.
	OnFullscreen func(on bool)
	// App returns what does the menu over the video asks of the app, asked
	// each time a window opens on the stage (the app may come after the
	// stage). Nil leaves those items out.
	App func() AppActions
}

// appOf is what a stage's windows ask of the app; none without a stage.
func appOf(stage *Stage) AppActions {
	if stage == nil || stage.options.App == nil {
		return nil
	}
	return stage.options.App()
}

// StageLayout is where the video goes inside the app's window.
type StageLayout struct {
	// Visible shows the video; the app hides it behind its other screens.
	Visible bool
	// Insets is the room the app's own controls take around the video, in
	// pixels at 100% scaling.
	Insets layout.Insets
}

// NewStage returns a stage on the app's window.
func NewStage(options StageOptions) *Stage {
	return &Stage{options: options}
}

// SetLayout places the video: in the open window at once, and in the next
// ones. Insets that are no size count as none.
func (s *Stage) SetLayout(next StageLayout) {
	s.mu.Lock()
	s.layout = next
	w := s.current
	s.mu.Unlock()
	if w != nil {
		_ = w.thread.Post(w.layoutHost) // the window may be closing
	}
}

// Layout is where the video goes now.
func (s *Stage) Layout() StageLayout {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.layout
}

// SetFullscreen puts the open video in fullscreen, or back; without an open
// video it does nothing.
func (s *Stage) SetFullscreen(on bool) {
	s.mu.Lock()
	w := s.current
	s.mu.Unlock()
	if w != nil {
		_ = w.thread.Post(func() { w.setFullscreen(on) })
	}
}

// Opener returns what a watch session uses to open its player window on
// this stage, as Opener does for a window of its own.
func (s *Stage) Opener(look func() overlay.Look, labels Labels, title func(video string) string, logger *slog.Logger) func(context.Context, watch.WindowRequest) (watch.Window, error) {
	return opener(s, look, labels, title, logger)
}

// parent is the app's window, zero before it exists.
func (s *Stage) parent() windows.HWND {
	if s.options.Parent == nil {
		return 0
	}
	return windows.HWND(s.options.Parent())
}

func (s *Stage) attach(w *Window) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = w
}

func (s *Stage) detach(w *Window) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == w {
		s.current = nil
	}
}

// fullscreenChanged tells the app, off the window's thread: the app may
// answer by laying out its screen, which comes back to the stage.
func (s *Stage) fullscreenChanged(on bool) {
	if s.options.OnFullscreen != nil {
		go s.options.OnFullscreen(on)
	}
}

// The app window's changes the stage follows: it moved or changed size (the
// video follows), its children changed order (the video goes back on top of
// the app's own), or it was minimized and restored.
var (
	stageProc  = syscall.NewCallback(dispatchStageEvent)
	stageHooks = struct {
		sync.Mutex
		m map[windows.Handle]*Window
	}{m: map[windows.Handle]*Window{}}
)

// followParent hooks the app window's changes. Runs on the UI thread.
func (w *Window) followParent() error {
	for _, span := range [][2]uint32{
		{win32.EVENT_SYSTEM_MINIMIZESTART, win32.EVENT_SYSTEM_MINIMIZEEND},
		{win32.EVENT_OBJECT_REORDER, win32.EVENT_OBJECT_REORDER},
		{win32.EVENT_OBJECT_LOCATIONCHANGE, win32.EVENT_OBJECT_LOCATIONCHANGE},
	} {
		hook, err := win32.SetWinEventHookInProcess(span[0], span[1], stageProc)
		if err != nil {
			return err
		}
		stageHooks.Lock()
		stageHooks.m[hook] = w
		stageHooks.Unlock()
		w.hooks = append(w.hooks, hook)
	}
	return nil
}

// unfollowParent removes the hooks. Runs on the UI thread.
func (w *Window) unfollowParent() {
	for _, hook := range w.hooks {
		win32.UnhookWinEvent(hook)
		stageHooks.Lock()
		delete(stageHooks.m, hook)
		stageHooks.Unlock()
	}
	w.hooks = nil
}

// dispatchStageEvent receives the app window's changes on the UI thread.
func dispatchStageEvent(hook windows.Handle, event uint32, hwnd windows.HWND, object, child int32, _ uint32, _ uint32) uintptr {
	stageHooks.Lock()
	w := stageHooks.m[hook]
	stageHooks.Unlock()
	if w == nil || hwnd != w.parent || w.host == 0 {
		return 0
	}
	switch event {
	case win32.EVENT_OBJECT_REORDER:
		// Its children changed order: back on top of them, unless already.
		if win32.TopChild(w.parent) != w.host {
			w.layoutHost()
		}
	case win32.EVENT_OBJECT_LOCATIONCHANGE:
		if object == win32.OBJID_WINDOW && child == win32.CHILDID_SELF {
			w.layoutHost()
		}
	case win32.EVENT_SYSTEM_MINIMIZESTART:
		w.visibility.Hide()
		w.refreshBar()
	case win32.EVENT_SYSTEM_MINIMIZEEND:
		w.layoutHost()
	}
	return 0
}

// layoutHost puts the embedded window where the stage says, above the app
// window's own children, and shows or hides it. Runs on the UI thread.
func (w *Window) layoutHost() {
	stage := w.options.Stage
	if stage == nil || w.host == 0 || !windows.IsWindow(w.parent) {
		return
	}
	client, err := win32.ClientRectOnScreen(w.parent)
	if err != nil {
		return
	}
	place := stage.Layout()
	size := layout.Rect{W: float64(client.Right - client.Left), H: float64(client.Bottom - client.Top)}
	area := layout.VideoArea(size, place.Insets, win32.DpiForWindow(w.parent), w.fullscreen)
	if !(place.Visible || w.fullscreen) || !area.Valid() {
		if windows.IsWindowVisible(w.host) {
			win32.ShowWindow(w.host, win32.SW_HIDE)
		}
		w.refreshBar()
		return
	}
	bounds := win32.Rect{Left: int32(area.X), Top: int32(area.Y), Right: int32(area.X + area.W), Bottom: int32(area.Y + area.H)}
	// HWND_TOP: above the app's own children, which draw its screen.
	_ = win32.SetWindowPos(w.host, 0, bounds, win32.SWP_NOACTIVATE|win32.SWP_SHOWWINDOW)
	w.refreshBar()
}
