//go:build windows && amd64

package overlay_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	user32           = windows.NewLazySystemDLL("user32.dll")
	procGetWindow    = user32.NewProc("GetWindow")
	procShowWindow   = user32.NewProc("ShowWindow")
	procSetWindowPos = user32.NewProc("SetWindowPos")
	procPostMessage  = user32.NewProc("PostMessageW")
)

const (
	gwOwner       = 4
	swHide        = 0
	swShowNA      = 8
	swMinimize    = 6
	swRestore     = 9
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	wsOverlapped  = 0x00CF0000 // WS_OVERLAPPEDWINDOW

	helperVariable = "LINGOREEL_FAKE_PLAYER_WINDOW"
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

// offscreenX keeps the fake player out of sight: windows may sit anywhere,
// and geometry, visibility and ownership work the same off the screen.
const offscreenX = -12000

// hangMessage makes the fake player stop pumping messages, like a hung app.
const hangMessage = win32.WM_APP + 7

// TestFakePlayerWindow is not a test: run as a child process, it shows a
// plain resizable window that obeys every size, standing in for a player.
// Writing "hang" to its input makes it stop responding.
func TestFakePlayerWindow(t *testing.T) {
	if os.Getenv(helperVariable) != "1" {
		t.Skip("helper process only")
	}
	// The window belongs to the OS thread that creates it, and only that
	// thread can pump its messages.
	runtime.LockOSThread()
	name, _ := windows.UTF16PtrFromString("LingoReelFakePlayer")
	proc := syscall.NewCallback(func(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case win32.WM_DESTROY:
			win32.PostQuitMessage()
			return 0
		case hangMessage:
			select {} // never returns: the window stops responding
		}
		return win32.DefWindowProc(hwnd, message, wparam, lparam)
	})
	if _, err := win32.RegisterClass(&win32.WndClassEx{WndProc: proc, Instance: win32.ModuleHandle(), ClassName: name}); err != nil {
		t.Fatal(err)
	}
	hwnd, err := win32.CreateWindow(0, wsOverlapped, name, 0, win32.ModuleHandle())
	if err != nil {
		t.Fatal(err)
	}
	x := int32(offscreenX) // negative: passed as a sign-extended int
	procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), 60, 656, 399, swpNoZOrder|swpNoActivate)
	procShowWindow.Call(uintptr(hwnd), swShowNA)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if scanner.Text() == "hang" {
				procPostMessage.Call(uintptr(hwnd), hangMessage, 0, 0)
			}
		}
	}()
	var msg win32.Msg
	for {
		if ok, _ := win32.GetMessage(&msg); !ok {
			return
		}
		win32.DispatchMessage(&msg)
	}
}

// fakePlayer is the helper window running in a child process; the overlay
// ignores events from its own process, so the player must be another one.
type fakePlayer struct {
	cmd   *exec.Cmd
	input io.WriteCloser
}

func startFakePlayer(t *testing.T) *fakePlayer {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakePlayerWindow$")
	cmd.Env = append(os.Environ(), helperVariable+"=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return &fakePlayer{cmd: cmd, input: input}
}

func (p *fakePlayer) pid() int { return p.cmd.Process.Pid }

func (p *fakePlayer) hang(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(p.input, "hang\n"); err != nil {
		t.Fatal(err)
	}
}

// hasVisibleWindow reports whether a process shows a top-level window.
func hasVisibleWindow(pid int) bool {
	found := false
	callback := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var owner uint32
		windows.GetWindowThreadProcessId(hwnd, &owner)
		if int(owner) == pid && windows.IsWindowVisible(hwnd) {
			found = true
			return 0
		}
		return 1
	})
	windows.EnumWindows(callback, unsafe.Pointer(nil))
	return found
}

// overlayWindows finds this process's overlay windows.
func overlayWindows() []windows.HWND {
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

// scene is an open overlay and the player window it follows.
type scene struct {
	t      *testing.T
	screen *overlay.Overlay
	window windows.HWND
	target windows.HWND
}

func openOver(t *testing.T, processID int) *scene {
	t.Helper()
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: processID, VideoWidth: 320, VideoHeight: 180,
		Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { screen.Close() })
	if err := screen.Show(overlay.Content{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	var ownerPID uint32
	windows.GetWindowThreadProcessId(windows.HWND(owner), &ownerPID)
	if int(ownerPID) != processID {
		t.Fatalf("overlay owner belongs to pid %d, want the player's %d", ownerPID, processID)
	}
	s := &scene{t: t, screen: screen, window: found[0], target: windows.HWND(owner)}
	s.waitUntil("the overlay to cover the player's client area", s.covering)
	return s
}

func (s *scene) playerEmpty() bool {
	r, err := win32.ClientRectOnScreen(s.target)
	return err == nil && (r.Right <= r.Left || r.Bottom <= r.Top)
}

func (s *scene) visible() bool { return windows.IsWindowVisible(s.window) }

func (s *scene) covering() bool {
	a, errA := win32.ClientRectOnScreen(s.window)
	b, errB := win32.ClientRectOnScreen(s.target)
	return errA == nil && errB == nil && a == b && s.visible()
}

func (s *scene) hidden() bool { return !s.visible() }

func (s *scene) state() string {
	a, _ := win32.ClientRectOnScreen(s.window)
	b, _ := win32.ClientRectOnScreen(s.target)
	return fmt.Sprintf("overlay %+v visible %v, player %+v visible %v iconic %v",
		a, s.visible(), b, windows.IsWindowVisible(s.target), win32.IsIconic(s.target))
}

func (s *scene) waitUntil(what string, condition func() bool) {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			s.t.Fatalf("timed out waiting for %s; %s", what, s.state())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *scene) place(x, y, w, h int) {
	procSetWindowPos.Call(uintptr(s.target), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoZOrder|swpNoActivate)
}

func (s *scene) show(command uintptr) {
	procShowWindow.Call(uintptr(s.target), command)
}

func (s *scene) waitForEnd() {
	s.t.Helper()
	select {
	case <-s.screen.Done():
	case <-time.After(10 * time.Second):
		s.t.Fatal("overlay still open after the player window went away")
	}
	if left := overlayWindows(); len(left) != 0 {
		s.t.Fatalf("%d overlay windows left after the player ended", len(left))
	}
	if err := s.screen.Close(); err != nil {
		s.t.Fatalf("Close after the player ended: %v", err)
	}
}

// TestOverlayFollowsAPlayerWindow drives a window that obeys every change,
// so each rule is checked exactly.
func TestOverlayFollowsAPlayerWindow(t *testing.T) {
	player := startFakePlayer(t)
	s := openOver(t, player.pid())

	s.place(offscreenX+40, 80, 1000, 600)
	s.waitUntil("the overlay to follow a resize", s.covering)
	s.place(offscreenX+80, 90, 1000, 600)
	s.waitUntil("the overlay to follow a move", s.covering)

	// Shrunk to its title bar the window has no client area: nothing to
	// cover, and a later move must not bring the stale overlay back.
	s.place(offscreenX+80, 90, 1000, 1)
	s.waitUntil("the player to keep only its title bar", s.playerEmpty)
	s.waitUntil("the overlay to hide over an empty client area", s.hidden)
	s.place(offscreenX+120, 90, 1000, 1)
	time.Sleep(300 * time.Millisecond)
	if s.visible() {
		t.Fatalf("a move brought back the overlay over an empty client area; %s", s.state())
	}
	s.place(offscreenX+80, 90, 1000, 600)
	s.waitUntil("the overlay to return with the client area", s.covering)

	// A hidden window keeps its size, but there is nothing to cover.
	s.show(swHide)
	s.waitUntil("the overlay to hide with the hidden player", s.hidden)
	s.show(swShowNA)
	s.waitUntil("the overlay to return with the player", s.covering)

	s.show(swMinimize)
	s.waitUntil("the overlay to hide with the minimized player", s.hidden)
	s.show(swRestore)
	s.waitUntil("the overlay to come back after a restore", s.covering)

	// The player process ends: its window is destroyed with it.
	player.cmd.Process.Kill()
	s.waitForEnd()
}

// Creating a window owned by a hung player waits on the player's thread;
// that is how the development machine got stuck. Open must give up in
// bounded time instead.
func TestOpenOverAHungPlayerGivesUpInTime(t *testing.T) {
	player := startFakePlayer(t)
	// Wait for its window before hanging it, so the failure is the hang.
	deadline := time.Now().Add(5 * time.Second)
	for !hasVisibleWindow(player.pid()) {
		if time.Now().After(deadline) {
			t.Fatal("the fake player never showed its window")
		}
		time.Sleep(50 * time.Millisecond)
	}
	player.hang(t)
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	screen, err := overlay.Open(t.Context(), overlay.Options{ProcessID: player.pid()})
	// One UI timeout (3 s) plus slack; paying it twice is already too slow.
	if elapsed := time.Since(start); elapsed > 4500*time.Millisecond {
		t.Fatalf("Open over a hung player took %v", elapsed)
	}
	if err == nil {
		start = time.Now()
		screen.Close()
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("Close over a hung player took %v", elapsed)
		}
	} else if !errors.Is(err, overlay.ErrPlayerNotResponding) {
		t.Fatalf("Open over a hung player = %v, want ErrPlayerNotResponding", err)
	}
}

// A hung player blocks the thread of a window it owns. Show must still return
// at once (the session keeps running) and Close must give up in bounded time
// instead of hanging the app, which is what a real hang did in development.
func TestHungPlayerNeverBlocksShowOrClose(t *testing.T) {
	player := startFakePlayer(t)
	s := openOver(t, player.pid())
	player.hang(t)
	time.Sleep(300 * time.Millisecond)
	// Make the overlay touch the hung owner: a resize request goes through
	// the player's (stuck) thread, and a redraw is queued behind it.
	go s.place(offscreenX+40, 80, 900, 500)

	start := time.Now()
	for i := range 50 {
		if err := s.screen.Show(overlay.Content{Primary: fmt.Sprint("linha ", i)}); err != nil && !errors.Is(err, overlay.ErrPlayerNotResponding) {
			t.Fatalf("Show: %v", err)
		}
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("50 Show calls took %v over a hung player", elapsed)
	}

	start = time.Now()
	err := s.screen.Close()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Close took %v over a hung player", elapsed)
	}
	if err != nil && !errors.Is(err, overlay.ErrPlayerNotResponding) {
		t.Fatalf("Close: %v", err)
	}
}

// TestOverlayFollowsTheRealVLC checks the same ownership and geometry over
// the real VLC, including its fullscreen-like resize and quitting it.
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
	s := openOver(t, session.ProcessID())

	s.place(40, 40, 900, 560)
	s.waitUntil("the overlay to follow VLC's resize", func() bool {
		r, err := win32.ClientRectOnScreen(s.window)
		return err == nil && s.covering() && r.Right-r.Left > 320
	})

	s.show(swMinimize)
	s.waitUntil("the overlay to hide with the minimized VLC", s.hidden)
	s.show(swRestore)
	// VLC may restore to a size of its own choosing, even an empty one.
	s.waitUntil("the overlay to come back with VLC", func() bool {
		if s.playerEmpty() {
			return s.hidden()
		}
		return s.covering()
	})

	if err := session.Close(); err != nil {
		t.Fatalf("closing VLC: %v", err)
	}
	s.waitForEnd()
}
