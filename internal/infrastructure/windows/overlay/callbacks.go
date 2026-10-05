//go:build windows && amd64

package overlay

import (
	"sync"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

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
