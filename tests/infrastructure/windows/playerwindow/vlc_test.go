//go:build windows && amd64

package playerwindow_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/vlc"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

var (
	sampleVideo     = filepath.Join("..", "..", "..", "testdata", "video", "study-sample.mkv")
	printWindow     = windows.NewLazySystemDLL("user32.dll").NewProc("PrintWindow")
	enumChildWindow = windows.NewLazySystemDLL("user32.dll").NewProc("EnumChildWindows")
)

func vlcExecutable(t *testing.T) string {
	t.Helper()
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("defina LINGOREEL_VLC_TEST=1 para abrir o VLC de verdade")
	}
	for _, installation := range discovery.Detect(discovery.SystemEnvironment{}, []discovery.Hints{vlc.Plugin{}.Hints()}, nil) {
		if installation.ID == player.VLC {
			return installation.Path
		}
	}
	t.Fatal("VLC is not installed")
	return ""
}

// childrenOf lists the processes owning child windows of a window.
func childrenOf(parent windows.HWND) map[uint32]int {
	owners := map[uint32]int{}
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var pid uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err == nil {
			owners[pid]++
		}
		return 1
	})
	enumChildWindow.Call(uintptr(parent), callback, 0)
	return owners
}

// litPixels counts the pixels of the window's client area that are not
// black, through PrintWindow (which also reads what Direct3D draws).
func litPixels(t *testing.T, hwnd windows.HWND) int {
	t.Helper()
	client, err := win32.ClientRectOnScreen(hwnd)
	if err != nil {
		t.Fatal(err)
	}
	dib, err := win32.NewDIB(int(client.Right-client.Left), int(client.Bottom-client.Top))
	if err != nil {
		t.Fatal(err)
	}
	defer dib.Close()
	const clientOnly, fullContent = 1, 2
	printWindow.Call(uintptr(hwnd), uintptr(dib.DC), clientOnly|fullContent)
	lit := 0
	for i := 0; i+3 < len(dib.Pixels); i += 4 * 9 {
		if int(dib.Pixels[i])+int(dib.Pixels[i+1])+int(dib.Pixels[i+2]) > 60 {
			lit++
		}
	}
	return lit
}

func processRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	event, _ := windows.WaitForSingleObject(handle, 0)
	return event == uint32(windows.WAIT_TIMEOUT)
}

// TestVLCClosesWithTheAppWindow is the reported bug end to end: VLC draws
// the video inside the app's window, and the close button of that window
// ends VLC with no process left.
func TestVLCClosesWithTheAppWindow(t *testing.T) {
	executable := vlcExecutable(t)
	video, err := filepath.Abs(sampleVideo)
	if err != nil {
		t.Fatal(err)
	}
	w := open(t)
	host := windows.HWND(w.Handle())
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, Start: 4500 * time.Millisecond, Host: w.Handle()})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()
	pid := session.ProcessID()

	testkit.Eventually(t, "VLC's video window inside the app's window", func() bool { return childrenOf(host)[uint32(pid)] > 0 })
	testkit.Eventually(t, "the picture inside the app's window", func() bool { return litPixels(t, host) > 100 })
	if hasVisibleTopLevel(uint32(pid)) {
		t.Fatal("VLC opened a window of its own")
	}
	if err := session.SetVolume(0.5); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}

	if err := win32.PostMessage(host, win32.WM_CLOSE, 0, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.Closed():
	case <-time.After(3 * time.Second):
		t.Fatal("the close button did not reach the app")
	}
	// What the session does on Closed: the player first, then the window.
	start := time.Now()
	if err := session.Close(); err != nil {
		t.Fatalf("closing VLC: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing the window: %v", err)
	}
	if processRunning(pid) {
		t.Fatal("VLC is still running after the window closed")
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("closing took %v", took)
	}
}

func hasVisibleTopLevel(pid uint32) bool {
	found := false
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err == nil && owner == pid && windows.IsWindowVisible(hwnd) {
			found = true
		}
		return 1
	})
	_ = windows.EnumWindows(callback, unsafe.Pointer(nil))
	return found
}
