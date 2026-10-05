//go:build windows

package win32_test

import (
	"runtime"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// After the process-wide call, a thread that never chose its own awareness
// works in physical pixels: that is what makes the work area match the
// coordinates the players use at 125–200% scaling.
func TestProcessWideDPIAwarenessReachesNewThreads(t *testing.T) {
	win32.UsePerMonitorDPIForProcess()
	aware := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		aware <- win32.ThreadUsesPerMonitorDPI()
	}()
	if !<-aware {
		t.Fatal("a new thread is not per-monitor DPI aware after UsePerMonitorDPIForProcess")
	}
}
