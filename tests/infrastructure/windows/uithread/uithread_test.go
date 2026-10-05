//go:build windows

package uithread_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/uithread"
)

func start(t *testing.T) *uithread.Thread {
	t.Helper()
	thread, err := uithread.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { thread.Close() })
	return thread
}

func TestDoAlwaysRunsOnTheSameOSThread(t *testing.T) {
	thread := start(t)
	ids := map[uint32]bool{}
	for range 20 {
		var id uint32
		if err := thread.Do(func() { id = windows.GetCurrentThreadId() }); err != nil {
			t.Fatalf("Do: %v", err)
		}
		ids[id] = true
	}
	if len(ids) != 1 {
		t.Fatalf("Do ran on %d OS threads, want 1", len(ids))
	}
	for id := range ids {
		if id == windows.GetCurrentThreadId() {
			t.Fatal("Do ran on the caller's thread")
		}
	}
}

func TestDoFromManyGoroutinesRunsEveryJobOnce(t *testing.T) {
	thread := start(t)
	var count atomic.Int64
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 20 {
				if err := thread.Do(func() { count.Add(1) }); err != nil {
					t.Errorf("Do: %v", err)
				}
			}
		})
	}
	wg.Wait()
	if got := count.Load(); got != 1000 {
		t.Fatalf("jobs run = %d, want 1000", got)
	}
}

func TestDoWaitsForTheJob(t *testing.T) {
	thread := start(t)
	done := false
	if err := thread.Do(func() { time.Sleep(30 * time.Millisecond); done = true }); err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("Do returned before the job finished")
	}
}

func TestCloseStopsTheLoopAndRefusesNewJobs(t *testing.T) {
	thread, err := uithread.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := thread.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-thread.Done():
	default:
		t.Fatal("Done not closed after Close")
	}
	if err := thread.Do(func() { t.Error("job ran after Close") }); !errors.Is(err, uithread.ErrClosed) {
		t.Fatalf("Do after Close = %v, want ErrClosed", err)
	}
	if err := thread.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// Jobs racing with Close either run or are refused, never lost: Do must not
// hang and must not report success for a job that did not run.
func TestJobsRacingWithCloseAreNeverLost(t *testing.T) {
	for range 20 {
		thread, err := uithread.Start()
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				ran := false
				err := thread.Do(func() { ran = true })
				if err == nil && !ran {
					t.Error("Do reported success for a job that did not run")
				}
				if err != nil && !errors.Is(err, uithread.ErrClosed) {
					t.Errorf("Do: %v", err)
				}
			})
		}
		closed := make(chan error, 1)
		go func() { closed <- thread.Close() }()
		finished := make(chan struct{})
		go func() { wg.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Fatal("Do hung while the thread was closing")
		}
		if err := <-closed; err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
}
