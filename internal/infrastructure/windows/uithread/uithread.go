//go:build windows

// Package uithread runs a Windows message loop on a dedicated OS thread.
// Windows, their hooks and GDI+ objects belong to the thread that created
// them, so everything that touches them runs through Do.
package uithread

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// ErrClosed reports a call on a thread that already stopped.
var ErrClosed = errors.New("UI thread closed")

// wakeMessage tells the loop that functions are waiting in the queue.
const wakeMessage = win32.WM_APP + 1

// className is the class of the message-only window that wakes each loop.
// A window message, unlike a thread message, reaches its window through any
// modal loop on the thread (a menu, a window being dragged or sized), so
// jobs run even while one is open.
const className = "LingoReelUIThread"

var (
	registerOnce      sync.Once
	registerErr       error
	wakeProc          = syscall.NewCallback(dispatchWake)
	classNameUTF16, _ = windows.UTF16PtrFromString(className)
	// threads maps each wake window to its thread, for the shared window
	// procedure.
	threadsMu sync.Mutex
	threads   = map[windows.HWND]*Thread{}
)

// Thread is a running message loop.
type Thread struct {
	window windows.HWND
	mu     sync.Mutex
	jobs   []func()
	done   chan struct{}
	// closed is set once Close was called or the loop ended.
	closed bool
	err    error
}

// Start launches the thread and waits until its message queue exists.
func Start() (*Thread, error) {
	t := &Thread{done: make(chan struct{})}
	ready := make(chan error, 1)
	go t.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Thread) loop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	win32.UsePerMonitorDPI()
	var msg win32.Msg
	win32.PeekMessage(&msg) // creates the queue before anyone posts to it
	window, err := wakeWindow()
	if err != nil {
		ready <- err
		return
	}
	t.window = window
	threadsMu.Lock()
	threads[window] = t
	threadsMu.Unlock()
	defer func() {
		threadsMu.Lock()
		delete(threads, window)
		threadsMu.Unlock()
		_ = win32.DestroyWindow(window) // only fails for a window already gone
	}()
	ready <- nil

	for {
		ok, err := win32.GetMessage(&msg)
		if err != nil {
			t.finish(fmt.Errorf("message loop: %w", err))
			return
		}
		if !ok {
			t.finish(nil)
			return
		}
		win32.DispatchMessage(&msg)
	}
}

// wakeWindow creates the thread's message-only window.
func wakeWindow() (windows.HWND, error) {
	registerOnce.Do(func() {
		_, registerErr = win32.RegisterClass(&win32.WndClassEx{
			WndProc: wakeProc, Instance: win32.ModuleHandle(), ClassName: classNameUTF16,
		})
	})
	if registerErr != nil {
		return 0, fmt.Errorf("registering the UI thread window class: %w", registerErr)
	}
	window, err := win32.CreateWindow(0, 0, classNameUTF16, win32.HWND_MESSAGE, win32.ModuleHandle())
	if err != nil {
		return 0, fmt.Errorf("creating the UI thread window: %w", err)
	}
	return window, nil
}

// dispatchWake runs a thread's queued jobs when its window is woken; it is
// called on that thread, by the loop or by a modal loop running there.
func dispatchWake(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
	if message == wakeMessage {
		threadsMu.Lock()
		t := threads[hwnd]
		threadsMu.Unlock()
		if t != nil {
			t.runJobs()
		}
		return 0
	}
	return win32.DefWindowProc(hwnd, message, wparam, lparam)
}

func (t *Thread) finish(err error) {
	t.mu.Lock()
	t.closed, t.err = true, err
	t.mu.Unlock()
	t.runJobs() // jobs queued after the loop decided to stop see ErrClosed
}

func (t *Thread) runJobs() {
	t.mu.Lock()
	jobs := t.jobs
	t.jobs = nil
	t.mu.Unlock()
	for _, job := range jobs {
		job()
	}
}

// Do runs fn on the UI thread and waits for it. Calling Do from the UI thread
// itself (from inside fn) would deadlock; code running there calls directly.
func (t *Thread) Do(fn func()) error {
	finished := make(chan struct{})
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrClosed
	}
	t.jobs = append(t.jobs, func() {
		defer close(finished)
		fn()
	})
	t.mu.Unlock()
	// A job queued while the loop was open always runs exactly once: by the
	// loop at its next wake-up, or by finish when the loop stops. A failed
	// post (the loop is on its way out) therefore changes nothing.
	_ = win32.PostMessage(t.window, wakeMessage, 0, 0)
	<-finished
	return nil
}

// Post queues fn to run on the UI thread and returns at once. It is for work
// whose caller must never wait on the UI thread, which a hung window owned by
// another process can block.
func (t *Thread) Post(fn func()) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrClosed
	}
	t.jobs = append(t.jobs, fn)
	t.mu.Unlock()
	// As in Do, a failed wake-up means the loop is stopping and finish runs
	// the job.
	_ = win32.PostMessage(t.window, wakeMessage, 0, 0)
	return nil
}

// Done is closed when the loop has ended.
func (t *Thread) Done() <-chan struct{} {
	return t.done
}

// Close ends the loop after the functions already queued, and waits for it.
func (t *Thread) Close() error {
	t.mu.Lock()
	already := t.closed
	t.mu.Unlock()
	if !already {
		if err := t.Do(win32.PostQuitMessage); err != nil && !errors.Is(err, ErrClosed) {
			return err
		}
	}
	<-t.done
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}
