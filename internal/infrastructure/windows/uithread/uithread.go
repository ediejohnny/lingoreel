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

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// ErrClosed reports a call on a thread that already stopped.
var ErrClosed = errors.New("UI thread closed")

// wakeMessage tells the loop that functions are waiting in the queue.
const wakeMessage = win32.WM_APP + 1

// Thread is a running message loop.
type Thread struct {
	id   uint32
	mu   sync.Mutex
	jobs []func()
	done chan struct{}
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
	t.id = windows.GetCurrentThreadId()
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
		if msg.HWnd == 0 && msg.Message == wakeMessage {
			t.runJobs()
			continue
		}
		win32.DispatchMessage(&msg)
	}
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
	_ = win32.PostThreadMessage(t.id, wakeMessage)
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
	_ = win32.PostThreadMessage(t.id, wakeMessage)
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
