//go:build windows && amd64

package overlay

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/gdiplus"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
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
	// noticeDuration is how long a notice stays, counted on the wall clock:
	// it is about the user's action, not about the video.
	noticeDuration = 1500 * time.Millisecond
	// noticeTimer identifies the window timer that removes the notice.
	noticeTimer = 1
)

// Options configures an overlay.
type Options struct {
	// ProcessID is the player process whose video window is followed.
	ProcessID int
	// Target is a window of this process to cover instead, such as the
	// window a player draws into; when set, ProcessID is not used.
	Target uintptr
	// Window, when set, names the player's main window as its plugin knows
	// it, instead of the largest window of the process; zero means not yet.
	Window func() uintptr
	// Area, when set, names the window inside the player's where the
	// picture is (its list and buttons stay uncovered); zero, or a window
	// gone or hidden, covers the whole player window.
	Area func() uintptr
	// AreaOnly covers Area and nothing else: with it hidden or gone, the
	// overlay hides instead of covering the whole window, which is the
	// app's own with its controls around the video.
	AreaOnly bool
	// VideoWidth and VideoHeight give the picture's aspect ratio, to keep the
	// text inside the picture when the window has black bars; zero fills the
	// window.
	VideoWidth, VideoHeight int
	Layout                  appearance.Layout
	Primary                 appearance.SubtitleStyle
	Translation             appearance.SubtitleStyle
	// PreviewColor and FillColor are the Modo Karaokê's colors: a line
	// shown before its time, and the part of a line already spoken.
	PreviewColor, FillColor appearance.Color
	// FindTimeout bounds the wait for the player window; zero means 10 s.
	FindTimeout time.Duration
	// Logger receives the errors that have no caller, such as a failure to
	// follow the player window; nil discards them.
	Logger *slog.Logger
}

// The overlay is the session's display.
var _ session.Display = (*Overlay)(nil)

// Overlay is a subtitle window over a player window.
type Overlay struct {
	thread   *uithread.Thread
	renderer *gdiplus.Renderer
	options  Options
	logger   *slog.Logger

	// Owned by the UI thread. area is the window last covered inside the
	// target (the target itself without Options.Area).
	target  windows.HWND
	area    windows.HWND
	window  windows.HWND
	hooks   []windows.Handle
	dib     *win32.DIB
	bounds  win32.Rect
	dpi     int
	shown   bool
	content session.Screen
	notice  string
	// reserve is the height at the bottom kept free for the control bar.
	reserve int

	done     chan struct{}
	doneOnce sync.Once

	// released is closed once the teardown ran and the UI thread ended;
	// releaseErr is set before that.
	releaseOnce sync.Once
	released    chan struct{}
	releaseErr  error

	// Content waiting to be drawn; only the latest counts.
	pendingMu sync.Mutex
	pending   session.Screen
	scheduled bool
	drawErr   error

	// boxes are where each line was last drawn, in the covered window's
	// client pixels, for LineAt; a zero box was not drawn. fonts are the
	// fonts they were drawn in.
	boxesMu sync.Mutex
	boxes   [2]layout.Rect
	fonts   [2]layout.Font
}

// Spot is where a line was drawn and in which font: the place of a box that
// corrects the line where it shows.
type Spot struct {
	Box  layout.Rect
	Font layout.Font
}

// callbackTable routes Win32 callbacks to their overlay. Window procedures
// and WinEvent hooks receive only a handle, and a Go pointer may not be kept
// in Win32 memory (GWLP_USERDATA) without breaking Go's pointer rules, so the
// process keeps this one table, shared by every overlay.
type callbackTable struct {
	mu      sync.Mutex
	windows map[windows.HWND]*Overlay
	hooks   map[windows.Handle]*Overlay
}

func (t *callbackTable) addWindow(window windows.HWND, o *Overlay) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.windows[window] = o
}

func (t *callbackTable) addHook(hook windows.Handle, o *Overlay) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hooks[hook] = o
}

// takeWindow returns the overlay of a window and forgets the window.
func (t *callbackTable) takeWindow(window windows.HWND) *Overlay {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := t.windows[window]
	delete(t.windows, window)
	return o
}

func (t *callbackTable) window(window windows.HWND) *Overlay {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.windows[window]
}

func (t *callbackTable) hook(hook windows.Handle) *Overlay {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hooks[hook]
}

// forget removes every entry of an overlay.
func (t *callbackTable) forget(o *Overlay) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, hook := range o.hooks {
		delete(t.hooks, hook)
	}
	delete(t.windows, o.window)
}

var (
	registerOnce      sync.Once
	registerErr       error
	windowProc        = syscall.NewCallback(dispatchWindow)
	winEventProc      = syscall.NewCallback(dispatchWinEvent)
	callbacks         = &callbackTable{windows: map[windows.HWND]*Overlay{}, hooks: map[windows.Handle]*Overlay{}}
	classNameUTF16, _ = windows.UTF16PtrFromString(className)
	noticeStyle       = appearance.NoticeStyle()
)

// Open waits for the player's video window and puts the overlay over it.
func Open(ctx context.Context, options Options) (*Overlay, error) {
	target := windows.HWND(options.Target)
	if target == 0 {
		if options.ProcessID <= 0 {
			return nil, fmt.Errorf("invalid player process id %d", options.ProcessID)
		}
		find := func() (windows.HWND, bool) { return findTarget(uint32(options.ProcessID)) }
		if options.Window != nil {
			find = func() (windows.HWND, bool) { hwnd := windows.HWND(options.Window()); return hwnd, hwnd != 0 }
		}
		found, err := waitForTarget(ctx, find, cmp.Or(options.FindTimeout, defaultFindFor))
		if err != nil {
			return nil, fmt.Errorf("finding the player window: %w", err)
		}
		target = found
	}
	thread, err := uithread.Start()
	if err != nil {
		return nil, fmt.Errorf("starting the overlay thread: %w", err)
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	o := &Overlay{
		thread: thread, options: options, logger: logger, target: target,
		done: make(chan struct{}), released: make(chan struct{}),
	}
	if err := o.onUIThread(o.setup); err != nil {
		// Without waiting: when the player hangs, the teardown runs as soon as
		// the player recovers or is ended, and the thread ends with it.
		o.release()
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

func waitForTarget(ctx context.Context, find func() (windows.HWND, bool), timeout time.Duration) (windows.HWND, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		if target, found := find(); found {
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
		return fmt.Errorf("starting GDI+: %w", err)
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
	callbacks.addWindow(window, o)

	for _, span := range [][2]uint32{
		{win32.EVENT_SYSTEM_MINIMIZESTART, win32.EVENT_SYSTEM_MINIMIZEEND},
		{win32.EVENT_OBJECT_DESTROY, win32.EVENT_OBJECT_LOCATIONCHANGE},
	} {
		var hook windows.Handle
		var err error
		if o.options.Target != 0 {
			hook, err = win32.SetWinEventHookInProcess(span[0], span[1], winEventProc)
		} else {
			hook, err = win32.SetWinEventHook(span[0], span[1], uint32(o.options.ProcessID), winEventProc)
		}
		if err != nil {
			return fmt.Errorf("following the player window: %w", err)
		}
		callbacks.addHook(hook, o)
		o.hooks = append(o.hooks, hook)
	}
	return o.follow()
}

// Show replaces the content. It never waits for the drawing, so a hung
// player cannot stall the session: the latest content is drawn as soon as
// the UI thread is free, and a drawing error is returned by the next call.
func (o *Overlay) Show(content session.Screen) error {
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

// Notify shows a notice in the top left corner of the video for
// noticeDuration; a new notice replaces the current one and starts the time
// again. Like Show, it never waits for the drawing.
func (o *Overlay) Notify(text string) error {
	return o.thread.Post(func() {
		if o.window == 0 {
			return
		}
		o.notice = text
		err := o.redraw()
		if err == nil {
			err = win32.SetTimer(o.window, noticeTimer, uint32(noticeDuration/time.Millisecond))
		}
		if err != nil {
			o.logger.Warn("overlay could not show a notice", "error", err)
		}
	})
}

// ReserveBottom keeps the subtitles above a band of the given height at the
// bottom of the window, where the control bar shows; zero frees it. Like
// Show, it never waits for the drawing.
func (o *Overlay) ReserveBottom(pixels int) error {
	return o.thread.Post(func() {
		if o.window == 0 || o.reserve == max(pixels, 0) {
			return
		}
		o.reserve = max(pixels, 0)
		if err := o.redraw(); err != nil {
			o.logger.Warn("overlay could not make room for the controls", "error", err)
		}
	})
}

// Restyle draws the subtitles with a new look from now on. Like Show, it
// never waits for the drawing.
func (o *Overlay) Restyle(look Look) error {
	return o.thread.Post(func() {
		o.options.Layout, o.options.Primary, o.options.Translation = look.Layout, look.Primary, look.Translation
		o.options.PreviewColor, o.options.FillColor = look.PreviewColor, look.FillColor
		if o.window == 0 {
			return
		}
		if err := o.redraw(); err != nil {
			o.logger.Warn("overlay could not draw the new look", "error", err)
		}
	})
}

// clearNotice runs on the UI thread when the notice's time is up.
func (o *Overlay) clearNotice() {
	win32.KillTimer(o.window, noticeTimer)
	o.notice = ""
	if err := o.redraw(); err != nil {
		o.logger.Warn("overlay could not remove a notice", "error", err)
	}
}

// Done is closed when the player window goes away.
func (o *Overlay) Done() <-chan struct{} {
	return o.done
}

// Close removes the overlay and stops its thread. If a hung player holds
// the UI thread, Close gives up after uiTimeout and reports it; the removal
// still happens as soon as the player recovers or is ended, and a later
// Close then returns without error.
func (o *Overlay) Close() error {
	o.release()
	timer := time.NewTimer(uiTimeout)
	defer timer.Stop()
	select {
	case <-o.released:
		return o.releaseErr
	case <-timer.C:
		return ErrPlayerNotResponding
	}
}

// release queues the teardown and ends the UI thread after it, once. Its
// goroutine belongs to the overlay and ends when released is closed; it may
// wait for a hung player, whose window the overlay's thread depends on.
func (o *Overlay) release() {
	o.releaseOnce.Do(func() {
		go func() {
			defer close(o.released)
			err := o.thread.Do(o.teardown)
			if errors.Is(err, uithread.ErrClosed) {
				err = nil
			}
			o.releaseErr = errors.Join(err, o.thread.Close())
		}()
	})
}

// teardown runs on the UI thread and is safe to call more than once.
func (o *Overlay) teardown() {
	callbacks.forget(o)
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
	o.area = o.target
	if o.options.Area != nil {
		switch area := windows.HWND(o.options.Area()); {
		case area != 0 && windows.IsWindow(area) && windows.IsWindowVisible(area):
			o.area = area
		case o.options.AreaOnly:
			o.hide()
			return nil
		}
	}
	bounds, err := win32.ClientRectOnScreen(o.area)
	if err != nil {
		return fmt.Errorf("reading the player window: %w", err)
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
	// A move to a monitor with another scaling keeps the size in pixels but
	// changes fixed-size text and the minimum font: lay out again.
	case resized || o.dib == nil || win32.DpiForWindow(o.window) != o.dpi:
		return o.redraw()
	case moved:
		if err := win32.MoveWindow(o.window, bounds.Left, bounds.Top); err != nil {
			return fmt.Errorf("moving the overlay: %w", err)
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
			return fmt.Errorf("creating the overlay bitmap: %w", err)
		}
		o.dib = dib
	} else {
		o.dib.Clear()
	}
	// The overlay's own window: its thread is per-monitor aware, so this is
	// the DPI of the monitor it is on, whatever the player's awareness.
	o.dpi = win32.DpiForWindow(o.window)
	area := layout.Rect{W: float64(width), H: float64(height)}
	floor := 0.0
	if o.reserve > 0 {
		floor = float64(height - o.reserve)
	}
	frame := layout.Compute(layout.Input{
		Floor:       floor,
		Video:       layout.VideoRect(area, o.options.VideoWidth, o.options.VideoHeight),
		DPI:         o.dpi,
		Layout:      o.options.Layout,
		Primary:     layout.Text{Style: o.options.Primary, Content: o.content.Primary},
		Translation: layout.Text{Style: o.options.Translation, Content: o.content.Translation},
		Notice:      layout.Text{Style: noticeStyle, Content: o.notice},
	}, o.renderer)
	look := gdiplus.Look{
		Primary: o.options.Primary, Translation: o.options.Translation,
		TranslationVisible: o.content.TranslationVisible, Notice: noticeStyle,
		PrimaryHidden: o.content.PrimaryHidden, Preview: o.options.PreviewColor, FillColor: o.options.FillColor,
	}
	if cue := o.content.Cue; cue != nil {
		look.Cue = &gdiplus.CueLook{Indicator: cue.Indicator, Progress: cue.Progress, Seconds: cue.Seconds}
	} else if o.content.Fill > 0 {
		look.FillWidths = layout.FillWidths(frame.Primary, o.content.Fill, o.renderer)
	}
	boxes := drawnBoxes(frame, o.content)
	var fonts [2]layout.Font
	if frame.Primary != nil {
		fonts[session.LinePrimary] = frame.Primary.Font
	}
	if frame.Translation != nil {
		fonts[session.LineTranslation] = frame.Translation.Font
	}
	o.boxesMu.Lock()
	o.boxes, o.fonts = boxes, fonts
	o.boxesMu.Unlock()
	if o.content.Highlighted {
		look.Highlight = gdiplus.OutlineAround(boxes[o.content.Highlight], o.dpi)
	}
	if err := o.renderer.Draw(o.dib, frame, look); err != nil {
		return fmt.Errorf("drawing the subtitles: %w", err)
	}
	position := win32.Point{X: o.bounds.Left, Y: o.bounds.Top}
	if err := win32.UpdateLayered(o.window, position, win32.Size{CX: int32(width), CY: int32(height)}, o.dib.DC); err != nil {
		return fmt.Errorf("showing the overlay: %w", err)
	}
	if o.coverable() {
		o.show()
	}
	return nil
}

// coverable tells whether there is something to cover: the player's window
// shown and, for an overlay of its area only, that area shown.
func (o *Overlay) coverable() bool {
	if win32.IsIconic(o.target) || !windows.IsWindowVisible(o.target) {
		return false
	}
	if !o.options.AreaOnly {
		return true
	}
	area := windows.HWND(o.options.Area())
	return area != 0 && windows.IsWindow(area) && windows.IsWindowVisible(area)
}

// LineAt tells which line was drawn at a point of the covered window's
// client area, so a click there can pick it for editing.
func (o *Overlay) LineAt(x, y float64) (session.Line, bool) {
	o.boxesMu.Lock()
	defer o.boxesMu.Unlock()
	for _, line := range []session.Line{session.LinePrimary, session.LineTranslation} {
		if box := o.boxes[line]; box.W > 0 && box.H > 0 && box.Contains(x, y) {
			return line, true
		}
	}
	return 0, false
}

// SpotOf is where a line was last drawn and in which font, in the covered
// window's client pixels; false when the line is not on screen.
func (o *Overlay) SpotOf(line session.Line) (Spot, bool) {
	if line != session.LinePrimary && line != session.LineTranslation {
		return Spot{}, false
	}
	o.boxesMu.Lock()
	defer o.boxesMu.Unlock()
	box := o.boxes[line]
	if box.W <= 0 || box.H <= 0 {
		return Spot{}, false
	}
	return Spot{Box: box, Font: o.fonts[line]}, true
}

// drawnBoxes are the boxes of the lines a frame draws: a hidden line or one
// without text has none.
func drawnBoxes(frame layout.Frame, content session.Screen) [2]layout.Rect {
	var boxes [2]layout.Rect
	if frame.Primary != nil && !content.PrimaryHidden && content.Primary != "" {
		boxes[session.LinePrimary] = frame.Primary.Bounds
	}
	if frame.Translation != nil && content.TranslationVisible && content.Translation != "" {
		boxes[session.LineTranslation] = frame.Translation.Bounds
	}
	return boxes
}

// dispatchWindow is the window procedure of every overlay window.
func dispatchWindow(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case win32.WM_NCHITTEST:
		return win32.HTTRANSPARENT
	case win32.WM_MOUSEACTIVATE:
		return win32.MA_NOACTIVATE
	case win32.WM_TIMER:
		if o := callbacks.window(hwnd); o != nil && wparam == noticeTimer {
			o.clearNotice()
			return 0
		}
	case win32.WM_DESTROY:
		if o := callbacks.takeWindow(hwnd); o != nil {
			// The system destroys owned windows with their owner: the player
			// window is gone, so the overlay ends too.
			o.window = 0
			o.teardown()
		}
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

// concerns reports a window whose changes move the overlay: the player's
// window, the one being covered, and — for a plugin that names a picture
// inside the player's window — any window within it, because the picture
// may be hidden, shown or replaced while the session runs.
func (o *Overlay) concerns(hwnd windows.HWND) bool {
	if hwnd == o.target || hwnd == o.area {
		return true
	}
	if o.options.AreaOnly {
		// The area is the overlay's own: no other window of the player's
		// counts, and the area matters even while it is hidden.
		return hwnd == windows.HWND(o.options.Area())
	}
	return o.options.Area != nil && win32.RootOwner(hwnd) == o.target
}

// dispatchWinEvent receives the player window's changes on the UI thread.
func dispatchWinEvent(hook windows.Handle, event uint32, hwnd windows.HWND, object, child int32, _ uint32, _ uint32) uintptr {
	o := callbacks.hook(hook)
	if o == nil || object != win32.OBJID_WINDOW || child != win32.CHILDID_SELF || !o.concerns(hwnd) {
		return 0
	}
	if event == win32.EVENT_OBJECT_DESTROY && hwnd == o.target {
		o.teardown()
		return 0
	}
	// No caller to report to: log it; the next event retries.
	if err := o.follow(); err != nil {
		o.logger.Warn("overlay could not follow the player window", "error", err)
	}
	return 0
}
