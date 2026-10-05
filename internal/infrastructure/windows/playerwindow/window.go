//go:build windows && amd64

// Package playerwindow is the app's own window for watching a video: the
// player draws the picture inside it, the subtitle overlay covers it, and a
// control bar shows over its bottom when the mouse goes there. Owning the
// window lets the app close it by any means (VLC 3 ignores the close button
// of a window of its own), go fullscreen and take the keys.
package playerwindow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// ErrNotResponding reports a window thread stuck, which a hung player
// drawing inside the window can cause.
var ErrNotResponding = errors.New("player window not responding")

const (
	hostClass     = "LingoReelWindow"
	barClass      = "LingoReelControls"
	defaultWidth  = 960
	defaultHeight = 540
	defaultOrigin = 100
	// uiTimeout bounds every wait on the window's thread.
	uiTimeout = 3 * time.Second
	// refreshEvery redraws the visible bar for the time and the progress.
	refreshEvery = 250 * time.Millisecond
	// cursorIdle hides the mouse over a fullscreen video left still.
	cursorIdle = 2 * time.Second
	// actionQueue bounds the controls waiting for the player.
	actionQueue = 16
)

// Window timers.
const (
	timerRefresh = 1 + iota
	timerHide
	timerCursor
	timerEdit
)

// Labels word what the bar shows; the presentation layer gives them.
type Labels struct {
	// Tip is an item's tip with its key in keys, such as "Pausar (Espaço)".
	Tip func(item controlbar.Item, state controlbar.State, keys shortcut.Map) string
	// Item is the text on an item: the speeds.
	Item func(item controlbar.Item) string
	// Time is the "current / total" text.
	Time func(position, duration time.Duration) string
	// Entry words a menu entry: its text, the value it shows (a stepper's
	// "10 s") and the key of its command.
	Entry func(entry controlbar.Entry, state controlbar.State, keys shortcut.Map) controlbar.EntryText
	// Status is the study text on the bar: "Início: 10:38".
	Status func(state controlbar.StudyState) string
	// EditHint shows under the box that corrects a line; EditEmpty is told
	// when Enter would save an empty line.
	EditHint, EditEmpty string
	// Menu words an item of the menu the right button opens: its text and
	// the key of its command in keys.
	Menu func(item contextmenu.Item, keys shortcut.Map) contextmenu.Text
}

// Options configures a window.
type Options struct {
	// Title is the window's title.
	Title string
	// Placement is the client area on screen; zero opens a default size.
	Placement player.Placement
	// VideoWidth and VideoHeight keep the subtitles on the picture.
	VideoWidth, VideoHeight int
	Look                    overlay.Look
	Labels                  Labels
	// Logger receives errors without a caller; nil discards them.
	Logger *slog.Logger
	// NoActivate shows the window without taking the focus, for tests and
	// diagnostics run while the user works.
	NoActivate bool
	// Stage puts the video inside the app's own window, where the stage
	// says, instead of in a window of its own; nil opens a window of its
	// own at Placement.
	Stage *Stage
	// App does what the menu over the video asks of the app (the settings,
	// files, another video); nil takes the stage's, and with no stage leaves
	// those items out.
	App AppActions
	// Choose stands for the menu on screen, for tests: it gets the menu
	// built (an HMENU) and the point on screen, and returns the ID of the
	// item to carry out, zero for none. Nil shows the real menu.
	Choose func(menu uintptr, x, y int32) uint32
	// Chrome is how the bar and the box for corrections look; nil takes the
	// stage's, and with no stage the app's defaults.
	Chrome *Chrome
}

// Chrome is how the controls over the video look: the bar and its menus,
// how long the bar stays after the mouse leaves it, the box that corrects a
// line, and the app's theme, which the menu over the video follows.
type Chrome struct {
	Bar       appearance.BarStyle
	HideDelay time.Duration
	Edit      appearance.EditStyle
	Theme     Theme
}

// Theme is the app's light or dark look; ThemeSystem follows Windows' apps.
type Theme int

const (
	ThemeSystem Theme = iota
	ThemeLight
	ThemeDark
)

// DefaultChrome is the controls' look before the user changes it.
func DefaultChrome() Chrome {
	bar := settings.DefaultBar()
	return Chrome{Bar: bar.Style, HideDelay: bar.HideDelay.Std(), Edit: appearance.DefaultEditStyle()}
}

// The window is a watch session's player window, and changes its look.
var (
	_ watch.Window     = (*Window)(nil)
	_ session.Restyler = (*Window)(nil)
)

// Window is an open player window.
type Window struct {
	thread  *uithread.Thread
	overlay *overlay.Overlay
	options Options
	logger  *slog.Logger
	actions chan func() error

	// Owned by the UI thread. On a stage, parent is the app's window, host
	// a child of it, and hooks follow the app's window.
	host, bar  windows.HWND
	parent     windows.HWND
	hooks      []windows.Handle
	renderer   *gdiplus.Renderer
	dib        *win32.DIB
	layout     controlbar.Bar
	visibility controlbar.Visibility
	chrome     Chrome
	barShown   bool
	speedMenu  bool
	// openMenu is the bar's menu shown, MenuNone for none.
	openMenu   controlbar.Menu
	hover      controlbar.Item
	drag       controlbar.Action
	dragX      float64
	fullscreen bool
	restore    win32.WindowPlacement
	style      uint32
	hideCursor bool
	tracking   [2]bool // WM_MOUSELEAVE asked for the host and the bar
	// revealKey is the translation key held down, to send its release.
	revealKey shortcut.Key
	// editing is the box correcting a line over the video, if open.
	editing *editor
	// menuOpen tells the context menu is on screen; icons are its pictures,
	// kept for the next menus and freed with the window (after the menu,
	// when the window goes while it shows: iconsLeft).
	menuOpen  bool
	icons     map[iconKey]windows.Handle
	iconsLeft bool

	remoteMu sync.Mutex
	remote   watch.Remote
	keys     atomic.Pointer[shortcut.Map]

	closed      chan struct{}
	closedOnce  sync.Once
	done        chan struct{}
	doneOnce    sync.Once
	releaseOnce sync.Once
	released    chan struct{}
	releaseErr  error
}

var (
	registerOnce  sync.Once
	registerErr   error
	hostProc      = syscall.NewCallback(dispatchHost)
	barProc       = syscall.NewCallback(dispatchBar)
	windowsByHWND = struct {
		sync.Mutex
		m map[windows.HWND]*Window
	}{m: map[windows.HWND]*Window{}}
	hostClassName, _ = windows.UTF16PtrFromString(hostClass)
	barClassName, _  = windows.UTF16PtrFromString(barClass)
)

// Open creates the window, shows it in front and puts the subtitle overlay
// over it. The player then draws into Handle.
func Open(ctx context.Context, options Options) (*Window, error) {
	thread, err := uithread.Start()
	if err != nil {
		return nil, fmt.Errorf("starting the player window thread: %w", err)
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if options.App == nil {
		// A window on the app's stage does what its menu asks through the app.
		options.App = appOf(options.Stage)
	}
	chrome := DefaultChrome()
	switch {
	case options.Stage != nil:
		chrome = options.Stage.Chrome()
	case options.Chrome != nil:
		chrome = *options.Chrome
	}
	w := &Window{
		thread: thread, options: options, logger: logger,
		actions: make(chan func() error, actionQueue),
		closed:  make(chan struct{}), done: make(chan struct{}), released: make(chan struct{}),
	}
	w.setChrome(chrome)
	var setupErr error
	if err := thread.Do(func() { setupErr = w.setup() }); err != nil || setupErr != nil {
		w.release()
		return nil, errors.Join(err, setupErr)
	}
	go w.runActions()
	covered := overlay.Options{
		Target: uintptr(w.host), VideoWidth: options.VideoWidth, VideoHeight: options.VideoHeight,
		Look: options.Look, Logger: logger,
	}
	if options.Stage != nil {
		// Over the video only: the app's controls around it stay uncovered.
		host := uintptr(w.host)
		covered.Target, covered.Area, covered.AreaOnly = uintptr(w.parent), func() uintptr { return host }, true
	}
	w.overlay, err = overlay.Open(ctx, covered)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("opening the subtitles over the player window: %w", err), w.Close())
	}
	if options.Stage != nil {
		options.Stage.attach(w)
	}
	return w, nil
}

// setup runs on the UI thread.
func (w *Window) setup() error {
	registerOnce.Do(registerClasses)
	if registerErr != nil {
		return fmt.Errorf("registering the player window classes: %w", registerErr)
	}
	renderer, err := gdiplus.NewRenderer()
	if err != nil {
		return fmt.Errorf("starting GDI+: %w", err)
	}
	w.renderer = renderer

	title, err := windows.UTF16PtrFromString(w.options.Title)
	if err != nil {
		title, _ = windows.UTF16PtrFromString("LingoReel")
	}
	if w.options.Stage != nil {
		if err := w.createOnStage(title); err != nil {
			return err
		}
	} else {
		bounds := initialBounds(w.options.Placement)
		host, err := win32.CreateTitledWindow(0, win32.WS_OVERLAPPEDWINDOW|win32.WS_CLIPCHILDREN, hostClassName, title, bounds, 0, win32.ModuleHandle())
		if err != nil {
			return fmt.Errorf("creating the player window: %w", err)
		}
		w.host = host
		remember(host, w)
		fitClient(host, bounds)
	}

	// Owned by the top-level window: above it, hidden with it.
	const barStyle = win32.WS_EX_LAYERED | win32.WS_EX_NOACTIVATE | win32.WS_EX_TOOLWINDOW
	bar, err := win32.CreateWindow(barStyle, win32.WS_POPUP, barClassName, w.frame(), win32.ModuleHandle())
	if err != nil {
		return fmt.Errorf("creating the control bar: %w", err)
	}
	w.bar = bar
	remember(bar, w)

	if w.options.Stage != nil {
		// The stage shows it where its layout says; the app's window keeps the keys.
		w.layoutHost()
		return nil
	}
	if w.options.NoActivate {
		win32.ShowWindow(w.host, win32.SW_SHOWNOACTIVATE)
		return nil
	}
	win32.ShowWindow(w.host, win32.SW_SHOWNORMAL)
	win32.SetForegroundWindow(w.host)
	return nil
}

func registerClasses() {
	large, small := windows.Handle(0), windows.Handle(0)
	if path, err := os.Executable(); err == nil {
		large, small = win32.ExecutableIcons(path)
	}
	if _, registerErr = win32.RegisterClass(&win32.WndClassEx{
		Style: win32.CS_DBLCLKS, WndProc: hostProc, Instance: win32.ModuleHandle(),
		Icon: large, IconSm: small, Cursor: win32.ArrowCursor(), Background: win32.BlackBrush(),
		ClassName: hostClassName,
	}); registerErr != nil {
		return
	}
	if _, registerErr = win32.RegisterClass(&win32.WndClassEx{
		WndProc: barProc, Instance: win32.ModuleHandle(), Cursor: win32.ArrowCursor(), ClassName: barClassName,
	}); registerErr != nil {
		return
	}
	registerErr = registerEditor()
}

// createOnStage creates the window as a child of the app's, hidden until
// the stage places it, and follows the app's window. Runs on the UI thread.
//
// The app's window belongs to another thread: the child's creation and end
// are kept from it (WS_EX_NOPARENTNOTIFY), so this thread never waits on
// that one to open or close the video.
func (w *Window) createOnStage(title *uint16) error {
	parent := w.options.Stage.parent()
	if parent == 0 || !windows.IsWindow(parent) {
		return ErrNoAppWindow
	}
	w.parent = parent
	const style = win32.WS_CHILD | win32.WS_CLIPCHILDREN | win32.WS_CLIPSIBLINGS
	host, err := win32.CreateTitledWindow(win32.WS_EX_NOPARENTNOTIFY, style, hostClassName, title, win32.Rect{}, parent, win32.ModuleHandle())
	if err != nil {
		return fmt.Errorf("creating the player window in the app's: %w", err)
	}
	w.host = host
	remember(host, w)
	// The app's window erases its background (white, Wails') without
	// clipping its children: when it grows (maximized, fullscreen) that paint
	// covers the parts of the video VLC paints the plain way, the black bars,
	// and nothing repaints them. Clipping its children keeps it off them.
	if style := win32.Style(parent); style&win32.WS_CLIPCHILDREN == 0 {
		win32.SetStyle(parent, style|win32.WS_CLIPCHILDREN)
	}
	if err := w.followParent(); err != nil {
		return fmt.Errorf("following the app's window: %w", err)
	}
	return nil
}

// frame is the top-level window the video shows in: the app's on a stage,
// the window's own otherwise.
func (w *Window) frame() windows.HWND {
	if w.options.Stage != nil {
		return w.parent
	}
	return w.host
}

// showing tells whether the video is on screen: shown, and its top-level
// window not minimized.
func (w *Window) showing() bool {
	return w.host != 0 && windows.IsWindowVisible(w.host) && !win32.IsIconic(w.frame())
}

// initialBounds is where the client area opens: the placement, or a
// default size when there is none.
func initialBounds(placement player.Placement) win32.Rect {
	if !placement.Set() {
		return win32.Rect{Left: defaultOrigin, Top: defaultOrigin, Right: defaultOrigin + defaultWidth, Bottom: defaultOrigin + defaultHeight}
	}
	return win32.Rect{
		Left: int32(placement.X), Top: int32(placement.Y),
		Right: int32(placement.X + placement.Width), Bottom: int32(placement.Y + placement.Height),
	}
}

// fitClient grows the window around its client area so the client area,
// not the whole window, takes the wanted bounds; the frame's size depends
// on the monitor's scaling, so it is measured instead of computed.
func fitClient(hwnd windows.HWND, client win32.Rect) {
	outer, err := win32.WindowRect(hwnd)
	if err != nil {
		return
	}
	inner, err := win32.ClientRectOnScreen(hwnd)
	if err != nil {
		return
	}
	bounds := win32.Rect{
		Left: client.Left - (inner.Left - outer.Left), Top: client.Top - (inner.Top - outer.Top),
		Right: client.Right + (outer.Right - inner.Right), Bottom: client.Bottom + (outer.Bottom - inner.Bottom),
	}
	_ = win32.SetWindowPos(hwnd, 0, bounds, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
}

// Handle is the window the player draws into.
func (w *Window) Handle() uintptr {
	return uintptr(w.host)
}

// Closed is closed when the user closes the window.
func (w *Window) Closed() <-chan struct{} {
	return w.closed
}

// Attach hands the window the session its controls drive.
func (w *Window) Attach(remote watch.Remote) {
	w.remoteMu.Lock()
	w.remote = remote
	w.remoteMu.Unlock()
}

func (w *Window) currentRemote() watch.Remote {
	w.remoteMu.Lock()
	defer w.remoteMu.Unlock()
	return w.remote
}

// SetShortcuts changes the keys the window takes.
func (w *Window) SetShortcuts(keys shortcut.Map) {
	resolved := keys.Resolved()
	w.keys.Store(&resolved)
	_ = w.thread.Post(w.refreshBar) // the tips show the keys; the window may be closing
}

// shortcuts are the keys the window takes: the defaults until a session
// gives its own.
func (w *Window) shortcuts() shortcut.Map {
	if keys := w.keys.Load(); keys != nil {
		return *keys
	}
	return shortcut.Defaults()
}

// Show draws the subtitles.
func (w *Window) Show(content session.Screen) error {
	return w.overlay.Show(content)
}

// LineBounds is where a line was last drawn, in the window's client pixels;
// false when it is not on screen.
func (w *Window) LineBounds(line session.Line) (layout.Rect, bool) {
	spot, ok := w.overlay.SpotOf(line)
	return spot.Box, ok
}

// Restyle draws the subtitles with a new look.
func (w *Window) Restyle(look overlay.Look) error {
	return w.overlay.Restyle(look)
}

// Notify shows a short notice over the video.
func (w *Window) Notify(text string) error {
	return w.overlay.Notify(text)
}

// Done is closed when the window is gone.
func (w *Window) Done() <-chan struct{} {
	return w.done
}

// Close removes the subtitles and the window. If the thread is stuck on a
// hung player, Close gives up after uiTimeout; the window still goes as soon
// as the player is ended.
func (w *Window) Close() error {
	var overlayErr error
	if w.overlay != nil {
		overlayErr = w.overlay.Close()
	}
	w.release()
	timer := time.NewTimer(uiTimeout)
	defer timer.Stop()
	select {
	case <-w.released:
		return errors.Join(overlayErr, w.releaseErr)
	case <-timer.C:
		return errors.Join(overlayErr, ErrNotResponding)
	}
}

// release queues the teardown and ends the thread after it, once.
func (w *Window) release() {
	w.releaseOnce.Do(func() {
		go func() {
			defer close(w.released)
			err := w.thread.Do(w.teardown)
			if errors.Is(err, uithread.ErrClosed) {
				err = nil
			}
			w.releaseErr = errors.Join(err, w.thread.Close())
			close(w.actions)
		}()
	})
}

// teardown runs on the UI thread and is safe to call more than once.
func (w *Window) teardown() {
	w.finishEdit(false, false)
	// Jobs run inside the menu's loop: the menu closes, and whatever was
	// chosen in it is dropped once it returns to a window that is gone.
	if w.menuOpen {
		win32.EndMenu()
		w.iconsLeft = true
	} else {
		w.freeIcons()
	}
	if stage := w.options.Stage; stage != nil {
		stage.detach(w)
		w.unfollowParent()
		// The app's window as it was: out of the fullscreen, with the keys.
		w.setFullscreen(false)
		if w.host != 0 && win32.Focus() == w.host && windows.IsWindow(w.parent) {
			win32.SetFocus(w.parent)
		}
	}
	for _, hwnd := range []windows.HWND{w.bar, w.host} {
		forget(hwnd)
		if hwnd != 0 && windows.IsWindow(hwnd) {
			_ = win32.DestroyWindow(hwnd)
		}
	}
	w.bar, w.host = 0, 0
	w.dib.Close()
	w.dib = nil
	if w.renderer != nil {
		w.renderer.Close()
		w.renderer = nil
	}
	w.doneOnce.Do(func() { close(w.done) })
}

// runActions sends the controls to the player one at a time, off the UI
// thread: the player may take a moment to answer, and two quick jumps must
// both count. It ends when the window is released.
func (w *Window) runActions() {
	for action := range w.actions {
		if err := action(); err != nil && !errors.Is(err, session.ErrNotSupported) {
			w.logger.Warn("player window control failed", "error", err)
		}
		_ = w.thread.Post(w.refreshBar) // the window may be closing
	}
}

// do queues a control for the player; a full queue drops it, as a key
// held by the user repeats anyway.
func (w *Window) do(action func(watch.Remote) error) {
	remote := w.currentRemote()
	if remote == nil {
		return
	}
	select {
	case w.actions <- func() error { return action(remote) }:
	default:
	}
}

func remember(hwnd windows.HWND, w *Window) {
	windowsByHWND.Lock()
	defer windowsByHWND.Unlock()
	windowsByHWND.m[hwnd] = w
}

func forget(hwnd windows.HWND) {
	windowsByHWND.Lock()
	defer windowsByHWND.Unlock()
	delete(windowsByHWND.m, hwnd)
}

func lookup(hwnd windows.HWND) *Window {
	windowsByHWND.Lock()
	defer windowsByHWND.Unlock()
	return windowsByHWND.m[hwnd]
}
