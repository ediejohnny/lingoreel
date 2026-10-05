package testkit

import (
	"slices"
	"sync"
	"time"
)

// Timers stands in for time.AfterFunc: what it is given runs only when the
// test calls Fire, so a test of a delay never waits for it.
type Timers struct {
	mu      sync.Mutex
	pending []*func()
}

// Later has the shape of time.AfterFunc's use: it keeps run until Fire and
// returns what stops it, true when it was still waiting.
func (t *Timers) Later(_ time.Duration, run func()) (stop func() bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	waiting := &run
	t.pending = append(t.pending, waiting)
	return func() bool {
		t.mu.Lock()
		defer t.mu.Unlock()
		before := len(t.pending)
		t.pending = slices.DeleteFunc(t.pending, func(p *func()) bool { return p == waiting })
		return len(t.pending) < before
	}
}

// Pending is how many runs wait.
func (t *Timers) Pending() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pending)
}

// Fire runs everything that waits, outside the lock: a run may ask for
// another, which waits for the next Fire.
func (t *Timers) Fire() {
	t.mu.Lock()
	due := t.pending
	t.pending = nil
	t.mu.Unlock()
	for _, run := range due {
		(*run)()
	}
}
