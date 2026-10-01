//go:build windows && amd64

// Package hotkeys delivers keyboard shortcuts pressed in the player. They are
// registered only while a window of the player is in the foreground, so the
// combinations keep working in every other program.
package hotkeys

import (
	"errors"
	"fmt"
	"log/slog"
	"math/bits"
	"slices"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"lingoreel/internal/infrastructure/windows/uithread"
	"lingoreel/internal/infrastructure/windows/win32"
)

// ErrInvalidBinding reports a combination that cannot be a shortcut.
var ErrInvalidBinding = errors.New("invalid shortcut")

const (
	className = "LingoReelHotkeys"
	// maxModifiers keeps shortcuts to three keys: two modifiers and a key.
	maxModifiers = 2
	// releasePoll is how often a held key is checked for its release, which
	// RegisterHotKey does not report.
	releasePoll = 50 * time.Millisecond
	// releaseTimer identifies the window timer of that check.
	releaseTimer = 1
	// pressBuffer holds presses the session has not taken yet; past it the
	// oldest key repeats are the ones lost.
	pressBuffer = 64
)

// Binding is a key combination: win32 MOD_* modifiers and a virtual key code.
// Keys are matched by their code, not their character, so Shift+, works on
// layouts where it types "<" (ABNT2).
type Binding struct {
	Modifiers uint32
	Key       uint32
}

// Validate accepts at most two modifiers and one key, and refuses Alt+Shift,
// which switches the keyboard layout on Windows.
func (b Binding) Validate() error {
	const known = win32.MOD_ALT | win32.MOD_CONTROL | win32.MOD_SHIFT
	switch {
	case b.Key == 0 || b.Key > 0xFE:
		return fmt.Errorf("%w: key %#x", ErrInvalidBinding, b.Key)
	case b.Modifiers&^known != 0 || bits.OnesCount32(b.Modifiers) > maxModifiers:
		return fmt.Errorf("%w: modifiers %#x", ErrInvalidBinding, b.Modifiers)
	case b.Modifiers&(win32.MOD_ALT|win32.MOD_SHIFT) == win32.MOD_ALT|win32.MOD_SHIFT:
		return fmt.Errorf("%w: Alt+Shift switches the keyboard layout", ErrInvalidBinding)
	}
	return nil
}

// Press is one press of a shortcut, by its index in Options.Bindings.
// Repeat tells that the key stayed down since the previous press: the
// keyboard repeats a held key.
type Press struct {
	Index  int
	Repeat bool
}

// Options configures the shortcuts.
type Options struct {
	// ProcessID is the player: shortcuts work while one of its windows is
	// in the foreground.
	ProcessID int
	Bindings  []Binding
	// Logger receives problems that have no caller, such as a combination
	// another program already took; nil discards them.
	Logger *slog.Logger
}

// Hotkeys listens to the shortcuts of one player.
type Hotkeys struct {
	thread  *uithread.Thread
	options Options
	logger  *slog.Logger
	presses chan Press

	// Owned by the UI thread.
	window     windows.HWND
	hook       windows.Handle
	registered bool
	down       []bool
	polling    bool

	closeOnce sync.Once
	closeErr  error
}

// callbackTable routes the window procedure and the WinEvent hook to their
// Hotkeys, as the overlay does: Win32 callbacks receive only handles.
type callbackTable struct {
	mu      sync.Mutex
	windows map[windows.HWND]*Hotkeys
	hooks   map[windows.Handle]*Hotkeys
}

func (t *callbackTable) byWindow(window windows.HWND) *Hotkeys {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.windows[window]
}

func (t *callbackTable) byHook(hook windows.Handle) *Hotkeys {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hooks[hook]
}

func (t *callbackTable) add(h *Hotkeys) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.windows[h.window] = h
	if h.hook != 0 {
		t.hooks[h.hook] = h
	}
}

func (t *callbackTable) forget(h *Hotkeys) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.windows, h.window)
	delete(t.hooks, h.hook)
}

var (
	registerOnce      sync.Once
	registerErr       error
	windowProc        = syscall.NewCallback(dispatchWindow)
	winEventProc      = syscall.NewCallback(dispatchWinEvent)
	callbacks         = &callbackTable{windows: map[windows.HWND]*Hotkeys{}, hooks: map[windows.Handle]*Hotkeys{}}
	classNameUTF16, _ = windows.UTF16PtrFromString(className)
)

// Start begins listening. The shortcuts are registered at once if the
// player is in the foreground, and whenever it comes back to it.
func Start(options Options) (*Hotkeys, error) {
	if options.ProcessID <= 0 {
		return nil, fmt.Errorf("invalid player process id %d", options.ProcessID)
	}
	if len(options.Bindings) == 0 {
		return nil, fmt.Errorf("%w: no shortcuts", ErrInvalidBinding)
	}
	for i, binding := range options.Bindings {
		if err := binding.Validate(); err != nil {
			return nil, fmt.Errorf("shortcut %d: %w", i, err)
		}
		if slices.Contains(options.Bindings[:i], binding) {
			return nil, fmt.Errorf("%w: shortcut %d repeats another one", ErrInvalidBinding, i)
		}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	thread, err := uithread.Start()
	if err != nil {
		return nil, fmt.Errorf("starting the shortcuts thread: %w", err)
	}
	h := &Hotkeys{
		thread: thread, options: options, logger: logger,
		presses: make(chan Press, pressBuffer), down: make([]bool, len(options.Bindings)),
	}
	var setupErr error
	if err := thread.Do(func() { setupErr = h.setup() }); err != nil || setupErr != nil {
		_ = thread.Do(h.teardown)
		return nil, errors.Join(err, setupErr, thread.Close())
	}
	return h, nil
}

// setup runs on the UI thread.
func (h *Hotkeys) setup() error {
	registerOnce.Do(func() {
		_, registerErr = win32.RegisterClass(&win32.WndClassEx{
			WndProc: windowProc, Instance: win32.ModuleHandle(), ClassName: classNameUTF16,
		})
	})
	if registerErr != nil {
		return fmt.Errorf("registering the shortcuts window class: %w", registerErr)
	}
	window, err := win32.CreateWindow(0, 0, classNameUTF16, win32.HWND_MESSAGE, win32.ModuleHandle())
	if err != nil {
		return fmt.Errorf("creating the shortcuts window: %w", err)
	}
	h.window = window
	// Every process: the foreground moves between programs.
	hook, err := win32.SetWinEventHook(win32.EVENT_SYSTEM_FOREGROUND, win32.EVENT_SYSTEM_FOREGROUND, 0, winEventProc)
	if err != nil {
		return fmt.Errorf("following the foreground window: %w", err)
	}
	h.hook = hook
	callbacks.add(h)
	h.follow(win32.ForegroundWindow())
	return nil
}

// Presses delivers the shortcuts as they are pressed; it is closed by Close.
func (h *Hotkeys) Presses() <-chan Press {
	return h.presses
}

// Close releases the shortcuts and stops listening.
func (h *Hotkeys) Close() error {
	h.closeOnce.Do(func() {
		err := h.thread.Do(h.teardown)
		if errors.Is(err, uithread.ErrClosed) {
			err = nil
		}
		h.closeErr = errors.Join(err, h.thread.Close())
		close(h.presses)
	})
	return h.closeErr
}

// teardown runs on the UI thread and is safe to call more than once.
func (h *Hotkeys) teardown() {
	h.unregister()
	callbacks.forget(h)
	if h.hook != 0 {
		win32.UnhookWinEvent(h.hook)
		h.hook = 0
	}
	if h.window != 0 {
		_ = win32.DestroyWindow(h.window) // only fails for a window already gone
		h.window = 0
	}
}

// follow registers the shortcuts while the player is in the foreground and
// releases them as soon as another program is.
func (h *Hotkeys) follow(foreground windows.HWND) {
	var owner uint32
	if foreground != 0 {
		if _, err := windows.GetWindowThreadProcessId(foreground, &owner); err != nil {
			owner = 0
		}
	}
	if int(owner) == h.options.ProcessID {
		h.register()
	} else {
		h.unregister()
	}
}

func (h *Hotkeys) register() {
	if h.registered {
		return
	}
	h.registered = true
	for i, binding := range h.options.Bindings {
		if err := win32.RegisterHotKey(h.window, int32(i), binding.Modifiers, binding.Key); err != nil {
			h.logger.Warn("a shortcut is taken by another program", "modifiers", binding.Modifiers, "key", binding.Key, "error", err)
		}
	}
}

func (h *Hotkeys) unregister() {
	if !h.registered {
		return
	}
	h.registered = false
	for i := range h.options.Bindings {
		win32.UnregisterHotKey(h.window, int32(i))
	}
	clear(h.down)
	h.stopPolling()
}

// pressed handles one WM_HOTKEY.
func (h *Hotkeys) pressed(index int) {
	if index < 0 || index >= len(h.down) {
		return
	}
	press := Press{Index: index, Repeat: h.down[index]}
	h.down[index] = true
	if !h.polling {
		if err := win32.SetTimer(h.window, releaseTimer, uint32(releasePoll/time.Millisecond)); err == nil {
			h.polling = true
		} else {
			// Without the check every press counts as a new one: no fast step.
			clear(h.down)
		}
	}
	select {
	case h.presses <- press:
	default:
		h.logger.Warn("shortcut press dropped: the session is not taking them")
	}
}

// checkReleases marks the keys that went up; it stops once none is down.
func (h *Hotkeys) checkReleases() {
	anyDown := false
	for i, binding := range h.options.Bindings {
		if h.down[i] && !win32.KeyDown(binding.Key) {
			h.down[i] = false
		}
		anyDown = anyDown || h.down[i]
	}
	if !anyDown {
		h.stopPolling()
	}
}

func (h *Hotkeys) stopPolling() {
	if h.polling {
		win32.KillTimer(h.window, releaseTimer)
		h.polling = false
	}
}

func dispatchWindow(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	h := callbacks.byWindow(hwnd)
	switch {
	case h == nil:
	case message == win32.WM_HOTKEY:
		h.pressed(int(wparam))
		return 0
	case message == win32.WM_TIMER && wparam == releaseTimer:
		h.checkReleases()
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

func dispatchWinEvent(hook windows.Handle, event uint32, hwnd windows.HWND, object, child int32, _ uint32, _ uint32) uintptr {
	if h := callbacks.byHook(hook); h != nil && event == win32.EVENT_SYSTEM_FOREGROUND {
		h.follow(hwnd)
	}
	return 0
}
