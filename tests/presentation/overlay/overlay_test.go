//go:build windows && amd64

package overlay_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/vlc"
	"lingoreel/internal/infrastructure/windows/win32"
	"lingoreel/internal/presentation/overlay"
)

var sampleVideo = filepath.Join("..", "..", "testdata", "video", "study-sample.mkv")

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procGetWindow   = user32.NewProc("GetWindow")
	procShowWindow  = user32.NewProc("ShowWindow")
	procPostMessage = user32.NewProc("PostMessageW")
)

const (
	gwOwner    = 4
	swMinimize = 6
	swRestore  = 9
)

func TestOpenRejectsAnInvalidProcess(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if _, err := overlay.Open(t.Context(), overlay.Options{ProcessID: pid}); err == nil {
			t.Errorf("Open(pid %d) succeeded", pid)
		}
	}
}

func TestOpenGivesUpWhenThePlayerHasNoWindow(t *testing.T) {
	// This test process has no visible window.
	start := time.Now()
	_, err := overlay.Open(t.Context(), overlay.Options{ProcessID: os.Getpid(), FindTimeout: 300 * time.Millisecond})
	if !errors.Is(err, overlay.ErrNoPlayerWindow) {
		t.Fatalf("Open = %v, want ErrNoPlayerWindow", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Open waited %v for a 300 ms timeout", elapsed)
	}
}

func TestOpenStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := overlay.Open(ctx, overlay.Options{ProcessID: os.Getpid()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open = %v, want context.Canceled", err)
	}
}

// ownWindows finds this process's overlay windows.
func overlayWindows(t *testing.T) []windows.HWND {
	t.Helper()
	var found []windows.HWND
	pid := uint32(os.Getpid())
	callback := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		windows.GetWindowThreadProcessId(hwnd, &owner)
		if owner == pid {
			name := make([]uint16, 64)
			n, _ := windows.GetClassName(hwnd, &name[0], int32(len(name)))
			if windows.UTF16ToString(name[:n]) == "LingoReelOverlay" {
				found = append(found, hwnd)
			}
		}
		return 1
	})
	windows.EnumWindows(callback, unsafe.Pointer(nil))
	return found
}

func waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestOverlayFollowsTheRealVLC opens VLC, puts the overlay over it and checks
// ownership, geometry, minimize/restore and the end with the player.
func TestOverlayFollowsTheRealVLC(t *testing.T) {
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("defina LINGOREEL_VLC_TEST=1 para abrir o VLC de verdade")
	}
	var executable string
	for _, installation := range discovery.Detect(discovery.SystemEnvironment{}, nil) {
		if installation.ID == player.VLC {
			executable = installation.Path
		}
	}
	if executable == "" {
		t.Fatal("VLC is not installed")
	}
	video, err := filepath.Abs(sampleVideo)
	if err != nil {
		t.Fatal(err)
	}
	session, err := vlc.Launch(t.Context(), executable, player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true, Start: 4500 * time.Millisecond})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer session.Close()

	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: session.ProcessID(), VideoWidth: 320, VideoHeight: 180,
		Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer screen.Close()
	if err := screen.Show(overlay.Content{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true}); err != nil {
		t.Fatalf("Show: %v", err)
	}

	windowsFound := overlayWindows(t)
	if len(windowsFound) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(windowsFound))
	}
	window := windowsFound[0]
	target, _, _ := procGetWindow.Call(uintptr(window), gwOwner)
	var targetPID uint32
	windows.GetWindowThreadProcessId(windows.HWND(target), &targetPID)
	if int(targetPID) != session.ProcessID() {
		t.Fatalf("overlay owner belongs to pid %d, want VLC's %d", targetPID, session.ProcessID())
	}

	sameBounds := func() bool {
		a, errA := win32.ClientRectOnScreen(window)
		b, errB := win32.ClientRectOnScreen(windows.HWND(target))
		return errA == nil && errB == nil && a == b
	}
	waitUntil(t, "the overlay to cover the player's client area", sameBounds)
	if !windows.IsWindowVisible(window) {
		t.Fatal("overlay not visible over a visible player")
	}

	procShowWindow.Call(target, swMinimize)
	waitUntil(t, "the overlay to hide with the minimized player", func() bool { return !windows.IsWindowVisible(window) })
	procShowWindow.Call(target, swRestore)
	waitUntil(t, "the overlay to come back", func() bool { return windows.IsWindowVisible(window) && sameBounds() })

	// Closing the player window ends the overlay.
	procPostMessage.Call(target, win32.WM_CLOSE, 0, 0)
	select {
	case <-screen.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("overlay still open after the player window closed")
	}
	if left := overlayWindows(t); len(left) != 0 {
		t.Fatalf("%d overlay windows left after the player closed", len(left))
	}
	if err := screen.Close(); err != nil {
		t.Fatalf("Close after the player ended: %v", err)
	}
}
