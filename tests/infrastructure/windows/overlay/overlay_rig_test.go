//go:build windows && amd64

package overlay_test

import (
	"bufio"
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

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// The overlay tests' rig: a fake player window run as a child process, the
// scene of an overlay over it, and readings of what the screen shows.

var sampleVideo = filepath.Join("..", "..", "..", "testdata", "video", "study-sample.mkv")

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procGetWindow     = user32.NewProc("GetWindow")
	procGetWindowLong = user32.NewProc("GetWindowLongPtrW")
	// WindowFromPoint and GetAncestor find which top-level window is shown at
	// a screen point.
	procWindowFromPoint = user32.NewProc("WindowFromPoint")
	procGetAncestor     = user32.NewProc("GetAncestor")
	procShowWindow      = user32.NewProc("ShowWindow")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procPostMessage     = user32.NewProc("PostMessageW")
	procGetDC           = user32.NewProc("GetDC")
	procReleaseDC       = user32.NewProc("ReleaseDC")

	gdi32              = windows.NewLazySystemDLL("gdi32.dll")
	procGetStockObject = gdi32.NewProc("GetStockObject")
	procBitBlt         = gdi32.NewProc("BitBlt")
)

const (
	visibleVariable = "LINGOREEL_FAKE_PLAYER_VISIBLE"
	// areaVariable gives the fake player a child window holding the
	// "picture", like PotPlayer's, next to the rest of its window.
	areaVariable  = "LINGOREEL_FAKE_PLAYER_AREA"
	fakeAreaClass = "LingoReelFakeArea"
	// areaShare is how much of the player's width the picture takes.
	areaShare   = 0.6
	blackBrush  = 4
	hwndTopmost = ^uintptr(0) // (HWND)-1
	srcCopy     = 0x00CC0020
	captureBlt  = 0x40000000
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

// offscreenX keeps the fake player out of sight: windows may sit anywhere,
// and geometry, visibility and ownership work the same off the screen.
const offscreenX = -12000

// hangMessage makes the fake player stop pumping messages, like a hung app.
const hangMessage = win32.WM_APP + 7

// The fake player hides and shows its picture, like a player whose list
// takes the whole window.
const (
	hideAreaMessage = win32.WM_APP + 8
	showAreaMessage = win32.WM_APP + 9
)

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
	// area stands for the child window a player draws its picture in; it
	// only exists when the test asks for one.
	var area windows.HWND
	proc := syscall.NewCallback(func(hwnd windows.HWND, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case win32.WM_DESTROY:
			win32.PostQuitMessage()
			return 0
		case win32.WM_SIZE:
			if area != 0 {
				width, height := int32(lparam&0xFFFF), int32(lparam>>16&0xFFFF)
				procSetWindowPos.Call(uintptr(area), 0, 0, 0, uintptr(int32(float64(width)*areaShare)), uintptr(height), swpNoZOrder|swpNoActivate)
			}
		case hideAreaMessage:
			procShowWindow.Call(uintptr(area), swHide)
		case showAreaMessage:
			procShowWindow.Call(uintptr(area), swShowNA)
		case hangMessage:
			select {} // never returns: the window stops responding
		}
		return win32.DefWindowProc(hwnd, message, wparam, lparam)
	})
	black, _, _ := procGetStockObject.Call(blackBrush)
	class := win32.WndClassEx{WndProc: proc, Instance: win32.ModuleHandle(), ClassName: name, Background: windows.Handle(black)}
	if _, err := win32.RegisterClass(&class); err != nil {
		t.Fatal(err)
	}
	hwnd, err := win32.CreateWindow(0, wsOverlapped, name, 0, win32.ModuleHandle())
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(areaVariable) == "1" {
		if area, err = createFakeArea(hwnd); err != nil {
			t.Fatal(err)
		}
	}
	if os.Getenv(visibleVariable) == "1" {
		// On screen, in the corner, so its pixels can be captured; never
		// activated. It asks to stay on top, but Windows refused that on the
		// development machine, so the capture test checks that no other
		// window covers the area before trusting the pixels.
		procSetWindowPos.Call(uintptr(hwnd), hwndTopmost, 0, 0, 420, 280, swpNoActivate)
	} else {
		x := int32(offscreenX) // negative: passed as a sign-extended int
		procSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), 60, 656, 399, swpNoZOrder|swpNoActivate)
	}
	procShowWindow.Call(uintptr(hwnd), swShowNA)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			switch scanner.Text() {
			case "hang":
				procPostMessage.Call(uintptr(hwnd), hangMessage, 0, 0)
			case "hide-area":
				procPostMessage.Call(uintptr(hwnd), hideAreaMessage, 0, 0)
			case "show-area":
				procPostMessage.Call(uintptr(hwnd), showAreaMessage, 0, 0)
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

// wsChild marks a window drawn inside another one.
const wsChild = 0x40000000

// createFakeArea gives the fake player the child window a plugin would name
// as its picture: the left part of the client area, like PotPlayer's.
func createFakeArea(parent windows.HWND) (windows.HWND, error) {
	name, _ := windows.UTF16PtrFromString(fakeAreaClass)
	white, _, _ := procGetStockObject.Call(0) // WHITE_BRUSH: tells it apart
	class := win32.WndClassEx{WndProc: syscall.NewCallback(win32.DefWindowProc),
		Instance: win32.ModuleHandle(), ClassName: name, Background: windows.Handle(white)}
	if _, err := win32.RegisterClass(&class); err != nil {
		return 0, err
	}
	client, err := win32.ClientRectOnScreen(parent)
	if err != nil {
		return 0, err
	}
	bounds := win32.Rect{Right: int32(float64(client.Right-client.Left) * areaShare), Bottom: client.Bottom - client.Top}
	return win32.CreateTitledWindow(0, wsChild|win32.WS_VISIBLE, name, nil, bounds, parent, win32.ModuleHandle())
}

// fakePlayer is the helper window running in a child process; the overlay
// ignores events from its own process, so the player must be another one.
type fakePlayer struct {
	cmd   *exec.Cmd
	input io.WriteCloser
}

func startFakePlayer(t *testing.T) *fakePlayer {
	t.Helper()
	return launchFakePlayer(t, false)
}

func launchFakePlayer(t *testing.T, visible bool, extraEnv ...string) *fakePlayer {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestFakePlayerWindow$")
	cmd.Env = append(os.Environ(), helperVariable+"=1")
	cmd.Env = append(cmd.Env, extraEnv...)
	if visible {
		cmd.Env = append(cmd.Env, visibleVariable+"=1")
	}
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
	p.send(t, "hang")
}

func (p *fakePlayer) send(t *testing.T, command string) {
	t.Helper()
	if _, err := io.WriteString(p.input, command+"\n"); err != nil {
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
	if err := screen.Show(session.Screen{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true}); err != nil {
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

// windowedRunning is a player that names its windows (players.Windowed),
// as the PotPlayer plugin does; nothing else about it is used here.
type windowedRunning struct {
	processID    int
	window, area windows.HWND
}

func (r *windowedRunning) Capabilities() playback.Capabilities { return playback.Capabilities{} }
func (r *windowedRunning) Play() error                         { return nil }
func (r *windowedRunning) Pause() error                        { return nil }
func (r *windowedRunning) SeekTo(time.Duration) error          { return nil }
func (r *windowedRunning) SetSpeed(playback.Speed) error       { return nil }
func (r *windowedRunning) StepFrame(bool) error                { return nil }
func (r *windowedRunning) SelectAudioTrack(int) error          { return nil }
func (r *windowedRunning) Samples() <-chan playback.Sample     { return nil }
func (r *windowedRunning) ProcessID() int                      { return r.processID }
func (r *windowedRunning) Err() error                          { return nil }
func (r *windowedRunning) Close() error                        { return nil }
func (r *windowedRunning) Window() uintptr                     { return uintptr(r.window) }
func (r *windowedRunning) VideoArea() uintptr                  { return uintptr(r.area) }

// waitForWindowOf returns the process's visible top-level window.
func waitForWindowOf(t *testing.T, pid int) windows.HWND {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var found windows.HWND
		callback := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
			var owner uint32
			windows.GetWindowThreadProcessId(hwnd, &owner)
			if int(owner) == pid && windows.IsWindowVisible(hwnd) {
				found = hwnd
				return 0
			}
			return 1
		})
		windows.EnumWindows(callback, unsafe.Pointer(nil))
		if found != 0 {
			return found
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process %d showed no window", pid)
	return 0
}

// waitForChildOf returns the child window of a class inside parent.
func waitForChildOf(t *testing.T, parent windows.HWND, class string) windows.HWND {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var found windows.HWND
		callback := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
			name := make([]uint16, 64)
			n, _ := windows.GetClassName(hwnd, &name[0], int32(len(name)))
			if windows.UTF16ToString(name[:n]) == class {
				found = hwnd
				return 0
			}
			return 1
		})
		windows.EnumChildWindows(parent, callback, unsafe.Pointer(nil))
		if found != 0 {
			return found
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("window %#x has no %s child", parent, class)
	return 0
}

// captureClient copies what the screen shows over the player's client area.
func captureClient(t *testing.T, target windows.HWND) []byte {
	t.Helper()
	r, err := win32.ClientRectOnScreen(target)
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	dib, err := win32.NewDIB(w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer dib.Close()
	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)
	if ok, _, err := procBitBlt.Call(uintptr(dib.DC), 0, 0, uintptr(w), uintptr(h), screen, uintptr(r.Left), uintptr(r.Top), srcCopy|captureBlt); ok == 0 {
		t.Fatalf("BitBlt: %v", err)
	}
	return append([]byte(nil), dib.Pixels...)
}

// skipIfCovered skips a pixel test when another window (someone using the
// computer) covers part of the player: the screen would show that window, not
// the overlay. The overlay itself is transparent to WindowFromPoint.
func skipIfCovered(t *testing.T, player windows.HWND) {
	t.Helper()
	const gaRoot = 2
	r, err := win32.ClientRectOnScreen(player)
	if err != nil {
		t.Fatal(err)
	}
	for _, fx := range []int32{1, 2, 3} {
		for _, fy := range []int32{1, 2, 3} {
			x := r.Left + (r.Right-r.Left)*fx/4
			y := r.Top + (r.Bottom-r.Top)*fy/4
			// POINT goes by value: x in the low 32 bits, y in the high ones.
			point := uintptr(uint32(x)) | uintptr(uint32(y))<<32
			hit, _, _ := procWindowFromPoint.Call(point)
			root, _, _ := procGetAncestor.Call(hit, gaRoot)
			if windows.HWND(root) != player {
				t.Skipf("another window covers the player at (%d, %d); the pixel check needs that corner of the screen free", x, y)
			}
		}
	}
}

// windowFacts describes the overlay and the player for a failure message:
// visibility, placement, topmost flag and which one is higher on screen.
func windowFacts(overlayWindow, player windows.HWND) string {
	const gwlExStyle = -20
	const wsExTopmost = 0x00000008
	style := func(hwnd windows.HWND) uintptr {
		index := int32(gwlExStyle)
		value, _, _ := procGetWindowLong.Call(uintptr(hwnd), uintptr(index))
		return value
	}
	a, _ := win32.ClientRectOnScreen(overlayWindow)
	b, _ := win32.ClientRectOnScreen(player)
	return fmt.Sprintf("overlay visible=%v rect=%+v topmost=%v; player visible=%v rect=%+v topmost=%v; overlay above player=%v",
		windows.IsWindowVisible(overlayWindow), a, style(overlayWindow)&wsExTopmost != 0,
		windows.IsWindowVisible(player), b, style(player)&wsExTopmost != 0, isAbove(overlayWindow, player))
}

// isAbove walks the z-order down from a window looking for another one.
func isAbove(upper, lower windows.HWND) bool {
	const gwHwndNext = 2
	for hwnd := upper; hwnd != 0; {
		next, _, _ := procGetWindow.Call(uintptr(hwnd), gwHwndNext)
		hwnd = windows.HWND(next)
		if hwnd == lower {
			return true
		}
	}
	return false
}

// changedPixels counts pixels whose color differs between two captures.
func changedPixels(a, b []byte) int {
	changed := 0
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] {
			changed++
		}
	}
	return changed
}

// openOverAnother opens an overlay while another one exists and returns the
// new scene, told apart from the existing window.
func openOverAnother(t *testing.T, processID int, existing windows.HWND) *scene {
	t.Helper()
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: processID, VideoWidth: 320, VideoHeight: 180,
		Layout: appearance.DefaultLayout(), Primary: appearance.DefaultPrimaryStyle(), Translation: appearance.DefaultTranslationStyle(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { screen.Close() })
	if err := screen.Show(session.Screen{Primary: "Second."}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	for _, window := range overlayWindows() {
		if window == existing {
			continue
		}
		owner, _, _ := procGetWindow.Call(uintptr(window), gwOwner)
		s := &scene{t: t, screen: screen, window: window, target: windows.HWND(owner)}
		s.waitUntil("the second overlay to cover its player", s.covering)
		return s
	}
	t.Fatal("the second overlay window was not found")
	return nil
}
