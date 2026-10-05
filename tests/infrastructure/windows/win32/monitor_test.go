//go:build windows

package win32_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// Every machine has at least one monitor; exactly one is primary, each work
// area lies inside its monitor and the scaling is at least 100%.
func TestMonitorsDescribeEveryDisplay(t *testing.T) {
	monitors, err := win32.Monitors()
	if err != nil {
		t.Fatalf("Monitors: %v", err)
	}
	if len(monitors) == 0 {
		t.Fatal("no monitor found")
	}
	primaries := 0
	for i, m := range monitors {
		if m.Primary {
			primaries++
		}
		b, w := m.Bounds, m.WorkArea
		if b.Right <= b.Left || b.Bottom <= b.Top {
			t.Errorf("monitor %d has empty bounds %+v", i, b)
		}
		if w.Left < b.Left || w.Top < b.Top || w.Right > b.Right || w.Bottom > b.Bottom || w.Right <= w.Left {
			t.Errorf("monitor %d work area %+v is not inside %+v", i, w, b)
		}
		if m.DPI < 96 {
			t.Errorf("monitor %d reports %d DPI, below 100%% scaling", i, m.DPI)
		}
	}
	if primaries != 1 {
		t.Fatalf("%d primary monitors, want exactly 1", primaries)
	}
}

// Monitors can be listed again and again: the shared callback must not keep
// displays from an earlier call.
func TestMonitorsDoNotAccumulate(t *testing.T) {
	first, err := win32.Monitors()
	if err != nil {
		t.Fatal(err)
	}
	second, err := win32.Monitors()
	if err != nil || len(second) != len(first) {
		t.Fatalf("second listing = %d monitors (%v), first %d", len(second), err, len(first))
	}
}
