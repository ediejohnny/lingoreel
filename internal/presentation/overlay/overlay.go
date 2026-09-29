//go:build windows && amd64

package overlay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/infrastructure/windows/gdiplus"
	"lingoreel/internal/infrastructure/windows/uithread"
	"lingoreel/internal/infrastructure/windows/win32"
)

var (
	// ErrNoPlayerWindow reports a player process that never showed a window.
	ErrNoPlayerWindow = errors.New("player window not found")
	// ErrPlayerNotResponding reports a UI thread stuck on a hung player.
	ErrPlayerNotResponding = errors.New("player window not responding")
)

const (
	className      = "LingoReelOverlay"
	findInterval   = 100 * time.Millisecond
	defaultFindFor = 10 * time.Second
	// uiTimeout bounds every wait on the UI thread. Owning a window of the
	// player ties the overlay's thread to the player's: a hung player can
	// block it, and the session must not hang with it.
	uiTimeout = 3 * time.Second
)

// Options configures an overlay.
type Options struct {
	// ProcessID is the player process whose video window is followed.
	ProcessID int
	// VideoWidth and VideoHeight give the picture's aspect ratio, to keep the
	// text inside the picture when the window has black bars; zero fills the
	// window.
	VideoWidth, VideoHeight int
	Layout                  appearance.Layout
	Primary                 appearance.SubtitleStyle
	Translation             appearance.SubtitleStyle
	// FindTimeout bounds the wait for the player window; zero means 10 s.
	FindTimeout time.Duration
}

// Content is what the overlay shows.
type Content struct {
	Primary            string
	Translation        string
	TranslationVisible bool
}

// Overlay is a subtitle window over a player window.
type Overlay struct {
	thread   *uithread.Thread
	renderer *gdiplus.Renderer
	options  Options

	// Owned by the UI thread.
	target  windows.HWND
	window  windows.HWND
	hooks   []windows.Handle
	dib     *win32.DIB
	bounds  win32.Rect
	shown   bool
	content Content

	done     chan struct{}
	doneOnce sync.Once

	// Content waiting to be drawn; only the latest counts.
	pendingMu sync.Mutex
	pending   Content
	scheduled bool
	drawErr   error
}

var (
	registerOnce      sync.Once
	registerErr       error
	windowProc        = syscall.NewCallback(dispatchWindow)
	winEventProc      = syscall.NewCallback(dispatchWinEvent)
	registryMu        sync.Mutex
	byWindow          = map[windows.HWND]*Overlay{}
	byHook            = map[windows.Handle]*Overlay{}
	classNameUTF16, _ = windows.UTF16PtrFromString(className)
)

// Open waits for the player's video window and puts the overlay over it.
func Open(ctx context.Context, options Options) (*Overlay, error) {
	if options.ProcessID <= 0 {
		return nil, fmt.Errorf("invalid player process id %d", options.ProcessID)
	}
	target, err := waitForTarget(ctx, uint32(options.ProcessID), cmpOr(options.FindTimeout, defaultFindFor))
	if err != nil {
		return nil, err
	}
	thread, err := uithread.Start()
	if err != nil {
		return nil, err
	}
	o := &Overlay{thread: thread, options: options, target: target, done: make(chan struct{})}
	if err := o.onUIThread(o.setup); err != nil {
		if errors.Is(err, ErrPlayerNotResponding) {
			// The UI thread is stuck on the player; the cleanup finishes when
			// the player recovers or exits, without holding the caller.
			go o.Close()
			return nil, err
		}
		o.Close()
		return nil, err
	}
	return o, nil
}

// onUIThread runs fn on the UI thread and waits at most uiTimeout for it.
func (o *Overlay) onUIThread(fn func() error) error {
	result := make(chan error, 1)
	go func() {
		var fnErr error
		err := o.thread.Do(func() { fnErr = fn() })
		result <- errors.Join(err, fnErr)
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(uiTimeout):
		return ErrPlayerNotResponding
	}
}

func cmpOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func waitForTarget(ctx context.Context, processID uint32, timeout time.Duration) (windows.HWND, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		if target, found := findTarget(processID); found {
			return target, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-deadline.C:
			return 0, ErrNoPlayerWindow
		case <-time.After(findInterval):
		}
	}
}

// enumeration collects windows during one EnumWindows call. Callbacks made
// with syscall.NewCallback are never freed, so a single one serves every
// search, serialized by the mutex.
var enumeration struct {
	sync.Mutex
	processID  uint32
	candidates []Candidate
}

var enumProc = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	var owner uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err == nil && owner == enumeration.processID {
		c := Candidate{Handle: uintptr(hwnd), Visible: windows.IsWindowVisible(hwnd)}
		if r, err := win32.ClientRectOnScreen(hwnd); err == nil {
			c.Width, c.Height = int(r.Right-r.Left), int(r.Bottom-r.Top)
		}
		enumeration.candidates = append(enumeration.candidates, c)
	}
	return 1 // keep enumerating
})

// findTarget lists the process's top-level windows and picks the video one.
func findTarget(processID uint32) (windows.HWND, bool) {
	enumeration.Lock()
	defer enumeration.Unlock()
	enumeration.processID, enumeration.candidates = processID, nil
	// A failed enumeration leaves no candidates, which reads as "not yet".
	_ = windows.EnumWindows(enumProc, nil)
	chosen, found := ChooseTarget(enumeration.candidates)
	return windows.HWND(chosen.Handle), found
}

// setup runs on the UI thread: renderer, window, hooks and the first frame.
func (o *Overlay) setup() error {
	registerOnce.Do(func() {
		_, registerErr = win32.RegisterClass(&win32.WndClassEx{
			WndProc: windowProc, Instance: win32.ModuleHandle(), ClassName: classNameUTF16,
		})
	})
	if registerErr != nil {
		return fmt.Errorf("registering the overlay window class: %w", registerErr)
	}
	renderer, err := gdiplus.NewRenderer()
	if err != nil {
		return err
	}
	o.renderer = renderer

	// Owned by the player window: always right above it, hidden when it is
	// minimized, destroyed with it.
	const exStyle = win32.WS_EX_LAYERED | win32.WS_EX_TRANSPARENT | win32.WS_EX_NOACTIVATE | win32.WS_EX_TOOLWINDOW
	window, err := win32.CreateWindow(exStyle, win32.WS_POPUP, classNameUTF16, o.target, win32.ModuleHandle())
	if err != nil {
		return fmt.Errorf("creating the overlay window: %w", err)
	}
	o.window = window
	registryMu.Lock()
	byWindow[window] = o
	registryMu.Unlock()

	for _, span := range [][2]uint32{
		{win32.EVENT_SYSTEM_MINIMIZESTART, win32.EVENT_SYSTEM_MINIMIZEEND},
		{win32.EVENT_OBJECT_DESTROY, win32.EVENT_OBJECT_LOCATIONCHANGE},
	} {
		hook, err := win32.SetWinEventHook(span[0], span[1], uint32(o.options.ProcessID), winEventProc)
		if err != nil {
			return fmt.Errorf("following the player window: %w", err)
		}
		registryMu.Lock()
		byHook[hook] = o
		registryMu.Unlock()
		o.hooks = append(o.hooks, hook)
	}
	return o.follow()
}

// Show replaces the content. It never waits for the drawing, so a hung
// player cannot stall the session: the latest content is drawn as soon as
// the UI thread is free, and a drawing error is returned by the next call.
func (o *Overlay) Show(content Content) error {
	o.pendingMu.Lock()
	o.pending = content
	already := o.scheduled
	o.scheduled = true
	lastErr := o.drawErr
	o.drawErr = nil
	o.pendingMu.Unlock()
	if already {
		return lastErr
	}
	return errors.Join(lastErr, o.thread.Post(o.drawPending))
}

// drawPending runs on the UI thread and draws the latest content.
func (o *Overlay) drawPending() {
	o.pendingMu.Lock()
	content := o.pending
	o.scheduled = false
	o.pendingMu.Unlock()
	if o.window == 0 {
		return
	}
	o.content = content
	if err := o.redraw(); err != nil {
		o.pendingMu.Lock()
		o.drawErr = err
		o.pendingMu.Unlock()
	}
}

// Done is closed when the player window goes away.
func (o *Overlay) Done() <-chan struct{} {
	return o.done
}

// Close removes the overlay and stops its thread. If a hung player holds
// the UI thread, Close gives up after uiTimeout and reports it.
func (o *Overlay) Close() error {
	err := o.onUIThread(func() error { o.teardown(); return nil })
	switch {
	case errors.Is(err, ErrPlayerNotResponding):
		return err
	case errors.Is(err, uithread.ErrClosed):
		err = nil
	}
	closed := make(chan error, 1)
	go func() { closed <- o.thread.Close() }()
	select {
	case closeErr := <-closed:
		return errors.Join(err, closeErr)
	case <-time.After(uiTimeout):
		return ErrPlayerNotResponding
	}
}

// teardown runs on the UI thread and is safe to call more than once.
func (o *Overlay) teardown() {
	registryMu.Lock()
	for _, hook := range o.hooks {
		delete(byHook, hook)
	}
	delete(byWindow, o.window)
	registryMu.Unlock()
	for _, hook := range o.hooks {
		win32.UnhookWinEvent(hook)
	}
	o.hooks = nil
	if o.window != 0 && windows.IsWindow(o.window) {
		_ = win32.DestroyWindow(o.window) // already gone when the owner closed
	}
	o.window = 0
	o.dib.Close()
	o.dib = nil
	if o.renderer != nil {
		o.renderer.Close()
		o.renderer = nil
	}
	o.doneOnce.Do(func() { close(o.done) })
}

// follow matches the overlay to the player's client area: hidden while the
// player is minimized or hidden, moved when it moves, redrawn when it resizes.
func (o *Overlay) follow() error {
	if !windows.IsWindow(o.target) {
		o.teardown()
		return nil
	}
	if win32.IsIconic(o.target) || !windows.IsWindowVisible(o.target) {
		o.hide()
		return nil
	}
	bounds, err := win32.ClientRectOnScreen(o.target)
	if err != nil {
		return err
	}
	resized := bounds.Right-bounds.Left != o.bounds.Right-o.bounds.Left || bounds.Bottom-bounds.Top != o.bounds.Bottom-o.bounds.Top
	moved := bounds != o.bounds
	o.bounds = bounds
	if bounds.Right <= bounds.Left || bounds.Bottom <= bounds.Top {
		// No client area (a window shrunk to its title bar): nothing to cover.
		o.hide()
		return nil
	}
	switch {
	case resized || o.dib == nil:
		return o.redraw()
	case moved:
		if err := win32.MoveWindow(o.window, bounds.Left, bounds.Top); err != nil {
			return err
		}
	}
	o.show()
	return nil
}

func (o *Overlay) hide() {
	if o.shown {
		win32.ShowWindow(o.window, win32.SW_HIDE)
		o.shown = false
	}
}

func (o *Overlay) show() {
	if !o.shown {
		win32.ShowWindow(o.window, win32.SW_SHOWNA)
		o.shown = true
	}
}

// redraw lays out and paints the current content over the whole client area.
func (o *Overlay) redraw() error {
	width, height := int(o.bounds.Right-o.bounds.Left), int(o.bounds.Bottom-o.bounds.Top)
	if width <= 0 || height <= 0 {
		o.hide()
		return nil
	}
	if o.dib == nil || o.dib.Width != width || o.dib.Height != height {
		o.dib.Close()
		dib, err := win32.NewDIB(width, height)
		if err != nil {
			o.dib = nil
			return err
		}
		o.dib = dib
	} else {
		o.dib.Clear()
	}
	area := layout.Rect{W: float64(width), H: float64(height)}
	frame := layout.Compute(layout.Input{
		Video:       layout.VideoRect(area, o.options.VideoWidth, o.options.VideoHeight),
		DPI:         win32.DpiForWindow(o.target),
		Layout:      o.options.Layout,
		Primary:     layout.Text{Style: o.options.Primary, Content: o.content.Primary},
		Translation: layout.Text{Style: o.options.Translation, Content: o.content.Translation},
	}, o.renderer)
	look := gdiplus.Look{Primary: o.options.Primary, Translation: o.options.Translation, TranslationVisible: o.content.TranslationVisible}
	if err := o.renderer.Draw(o.dib, frame, look); err != nil {
		return err
	}
	position := win32.Point{X: o.bounds.Left, Y: o.bounds.Top}
	if err := win32.UpdateLayered(o.window, position, win32.Size{CX: int32(width), CY: int32(height)}, o.dib.DC); err != nil {
		return fmt.Errorf("showing the overlay: %w", err)
	}
	if !win32.IsIconic(o.target) && windows.IsWindowVisible(o.target) {
		o.show()
	}
	return nil
}

// dispatchWindow is the window procedure of every overlay window.
func dispatchWindow(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case win32.WM_NCHITTEST:
		return win32.HTTRANSPARENT
	case win32.WM_MOUSEACTIVATE:
		return win32.MA_NOACTIVATE
	case win32.WM_DESTROY:
		registryMu.Lock()
		o := byWindow[hwnd]
		delete(byWindow, hwnd)
		registryMu.Unlock()
		if o != nil {
			// The system destroys owned windows with their owner: the player
			// window is gone, so the overlay ends too.
			o.window = 0
			o.teardown()
		}
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// dispatchWinEvent receives the player window's changes on the UI thread.
func dispatchWinEvent(hook windows.Handle, event uint32, hwnd windows.HWND, object, child int32, _ uint32, _ uint32) uintptr {
	registryMu.Lock()
	o := byHook[hook]
	registryMu.Unlock()
	if o == nil || hwnd != o.target || object != win32.OBJID_WINDOW || child != win32.CHILDID_SELF {
		return 0
	}
	if event == win32.EVENT_OBJECT_DESTROY {
		o.teardown()
		return 0
	}
	// Errors here have no caller to report to; the next event retries.
	_ = o.follow()
	return 0
}
