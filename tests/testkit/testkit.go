// Package testkit holds what many test packages share. Slow splits the quick
// check (go test -short, the default of scripts\check.ps1) from the full one
// (check.ps1 -Full), which runs before every merge and tag.
package testkit

import "testing"

// Slow marks a test that takes real time: windows that wait for the screen,
// real waits, huge inputs. The quick check skips it; the full check runs it.
func Slow(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("slow: runs in the full check (scripts\\check.ps1 -Full)")
	}
}
