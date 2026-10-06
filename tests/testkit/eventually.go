package testkit

import (
	"testing"
	"time"
)

// waitLimit is how long Eventually waits; it only matters when a test fails,
// so it is generous for windows and goroutines on a loaded machine.
const waitLimit = 5 * time.Second

// Eventually waits for a condition that another goroutine, a window or a
// fake player makes true, and fails the test naming what never came.
func Eventually(t testing.TB, what string, condition func() bool) {
	t.Helper()
	EventuallyWithin(t, what, waitLimit, condition)
}

// EventuallyWithin is Eventually for a condition that takes real work to
// come, such as hearing stretches of a film at the pace of the app, which
// the race detector and a full parallel run slow down many times.
func EventuallyWithin(t testing.TB, what string, limit time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
