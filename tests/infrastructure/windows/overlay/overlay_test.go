//go:build windows && amd64

package overlay_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/vlc"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/tests/testkit"
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

// A plugin that knows its player's windows (PotPlayer) names the main
// window and the one holding the picture: the subtitles cover only the
// picture, so the player's own list and buttons stay free.
func TestOverlayCoversTheVideoAreaThePluginNames(t *testing.T) {
	fake := launchFakePlayer(t, false, areaVariable+"=1")
	main := waitForWindowOf(t, fake.pid())
	area := waitForChildOf(t, main, fakeAreaClass)

	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID:  fake.pid(),
		Window:     func() uintptr { return uintptr(main) },
		Area:       func() uintptr { return uintptr(area) },
		VideoWidth: 320, VideoHeight: 180, Look: defaultLook(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { screen.Close() })
	if err := screen.Show(session.Screen{Primary: "That's sick!"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	// The overlay is still owned by the main window, so it stays above the
	// whole player, but it covers only the picture.
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	if windows.HWND(owner) != main {
		t.Fatalf("overlay owner %#x, want the main window %#x", owner, main)
	}
	covers := func(target windows.HWND) func() bool {
		return func() bool {
			a, errA := win32.ClientRectOnScreen(found[0])
			b, errB := win32.ClientRectOnScreen(target)
			return errA == nil && errB == nil && a == b && windows.IsWindowVisible(found[0])
		}
	}
	s := &scene{t: t, screen: screen, window: found[0], target: main}
	s.waitUntil("the overlay to cover the picture", covers(area))
	if covers(main)() {
		t.Fatal("the overlay covers the whole player window, not only the picture")
	}

	// Resizing the player resizes the picture; the overlay follows it.
	s.place(offscreenX+40, 80, 1000, 600)
	s.waitUntil("the overlay to follow the picture on a resize", covers(area))

	// A picture window that goes away leaves the whole window to cover.
	fake.send(t, "hide-area")
	s.waitUntil("the overlay to cover the whole window without a picture", covers(main))
	fake.send(t, "show-area")
	s.waitUntil("the overlay to go back to the picture", covers(area))

	fake.cmd.Process.Kill()
	s.waitForEnd()
}

// The Opener a session uses must pass the plugin's windows on, or the
// subtitles would be put over the whole player window.
func TestOpenerUsesTheWindowsThePluginNames(t *testing.T) {
	fake := launchFakePlayer(t, false, areaVariable+"=1")
	main := waitForWindowOf(t, fake.pid())
	area := waitForChildOf(t, main, fakeAreaClass)

	look := func() overlay.Look {
		return defaultLook()
	}
	running := &windowedRunning{processID: fake.pid(), window: main, area: area}
	display, err := overlay.Opener(look, nil)(t.Context(), running, media.Info{})
	if err != nil {
		t.Fatalf("Opener: %v", err)
	}
	t.Cleanup(func() { display.Close() })
	if err := display.Show(session.Screen{Primary: "That's sick!"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	s := &scene{t: t, screen: nil, window: found[0], target: main}
	s.waitUntil("the overlay to cover the picture the plugin named", func() bool {
		a, errA := win32.ClientRectOnScreen(found[0])
		b, errB := win32.ClientRectOnScreen(area)
		return errA == nil && errB == nil && a == b && windows.IsWindowVisible(found[0])
	})
}

// TestOverlayFollowsAPlayerWindow drives a window that obeys every change,
// so each rule is checked exactly.
func TestOverlayFollowsAPlayerWindow(t *testing.T) {
	testkit.Slow(t)
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
	testkit.Slow(t)
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

// What the overlay shows must change with every Show, even when the size
// stays the same and the bitmap is reused. A real paused session showed a
// subtitle only after the window was resized; this reads the screen itself.
func TestEveryShowReachesTheScreen(t *testing.T) {
	testkit.Slow(t)
	player := launchFakePlayer(t, true)
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: player.pid(), Look: defaultLook(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer screen.Close()
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	target := windows.HWND(owner)

	// A fixed pause was not enough under load: the "blank" picture was taken
	// before the player's first paint and the whole window differed later.
	// Wait until two captures in a row agree instead.
	settle := func() []byte {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		time.Sleep(300 * time.Millisecond) // let the posted drawing start
		skipIfCovered(t, target)
		previous := captureClient(t, target)
		for {
			time.Sleep(150 * time.Millisecond)
			shot := captureClient(t, target)
			if changedPixels(previous, shot) == 0 {
				return shot
			}
			if time.Now().After(deadline) {
				t.Fatal("the screen over the player never settled")
			}
			previous = shot
		}
	}
	blank := settle()

	steps := []struct {
		content session.Screen
		visible bool
	}{
		{session.Screen{Primary: "That's sick!", Translation: "Que loucura!", TranslationVisible: true}, true},
		{session.Screen{}, false},
		{session.Screen{Primary: "I'm not going there."}, true},
		{session.Screen{Primary: "I'm not going there.", Translation: "Eu não vou para lá.", TranslationVisible: false}, true},
	}
	var previous []byte
	for i, step := range steps {
		if err := screen.Show(step.content); err != nil {
			t.Fatalf("step %d: Show: %v", i, err)
		}
		shot := settle()
		changed := changedPixels(blank, shot)
		switch {
		case step.visible && changed < 100:
			t.Fatalf("step %d: only %d pixels differ from the blank player; the text did not reach the screen (%s)", i, changed, windowFacts(found[0], target))
		case !step.visible && changed > 0:
			t.Fatalf("step %d: %d pixels still differ after clearing the overlay (%s)", i, changed, windowFacts(found[0], target))
		}
		if i == 3 && changedPixels(previous, shot) != 0 {
			t.Fatalf("step %d: a hidden translation changed the picture", i)
		}
		previous = shot
	}
}

// A new look reaches the screen at once, with no new content: a change on
// the settings screen shows on the video being watched.
func TestRestyleRedrawsTheSubtitles(t *testing.T) {
	testkit.Slow(t)
	player := launchFakePlayer(t, true)
	look := defaultLook()
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: player.pid(), Look: look,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer screen.Close()
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	target := windows.HWND(owner)
	settle := func() []byte {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		time.Sleep(300 * time.Millisecond)
		skipIfCovered(t, target)
		previous := captureClient(t, target)
		for {
			time.Sleep(150 * time.Millisecond)
			shot := captureClient(t, target)
			if changedPixels(previous, shot) == 0 {
				return shot
			}
			if time.Now().After(deadline) {
				t.Fatal("the screen over the player never settled")
			}
			previous = shot
		}
	}
	if err := screen.Show(session.Screen{Primary: "That's sick!"}); err != nil {
		t.Fatal(err)
	}
	before := settle()
	yellow := look
	yellow.Primary.Color = appearance.RGB(0xFF, 0xE6, 0x00)
	yellow.Primary.FontSize *= 1.5
	if err := screen.Restyle(yellow); err != nil {
		t.Fatal(err)
	}
	after := settle()
	if changed := changedPixels(before, after); changed < 100 {
		t.Fatalf("only %d pixels changed with the new look", changed)
	}
	// Back to the first look: the first picture again.
	if err := screen.Restyle(look); err != nil {
		t.Fatal(err)
	}
	if changed := changedPixels(before, settle()); changed != 0 {
		t.Fatalf("%d pixels differ after going back to the first look", changed)
	}
}

// A notice shows in the top left corner and leaves by itself after 1.5 s,
// counted from the latest notice.
func TestNoticeShowsAndLeavesByItself(t *testing.T) {
	testkit.Slow(t)
	player := launchFakePlayer(t, true)
	screen, err := overlay.Open(t.Context(), overlay.Options{
		ProcessID: player.pid(), Look: defaultLook(),
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer screen.Close()
	found := overlayWindows()
	if len(found) != 1 {
		t.Fatalf("found %d overlay windows, want 1", len(found))
	}
	owner, _, _ := procGetWindow.Call(uintptr(found[0]), gwOwner)
	target := windows.HWND(owner)
	settle := func() []byte {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		skipIfCovered(t, target)
		previous := captureClient(t, target)
		for {
			time.Sleep(100 * time.Millisecond)
			shot := captureClient(t, target)
			if changedPixels(previous, shot) == 0 {
				return shot
			}
			if time.Now().After(deadline) {
				t.Fatal("the screen over the player never settled")
			}
			previous = shot
		}
	}
	time.Sleep(300 * time.Millisecond)
	blank := settle()

	start := time.Now()
	if err := screen.Notify("Legenda de cima (inglês): −0,150 s"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	shown := settle()
	if changed := changedPixels(blank, shown); changed < 100 {
		t.Fatalf("only %d pixels changed: the notice did not reach the screen (%s)", changed, windowFacts(found[0], target))
	}
	// A second notice before the first leaves restarts the time.
	time.Sleep(800 * time.Millisecond)
	if err := screen.Notify("Legenda de cima (inglês): −0,200 s"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	restarted := time.Now()
	time.Sleep(1000 * time.Millisecond) // 1.8 s after the first, 1.0 s after the second
	if changed := changedPixels(blank, captureClient(t, target)); changed < 100 {
		t.Fatalf("the notice left %v after the first one, though a second came %v later", time.Since(start), restarted.Sub(start))
	}
	deadline := restarted.Add(3 * time.Second)
	for changedPixels(blank, captureClient(t, target)) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the notice was still on screen 3 s after the last one (%s)", windowFacts(found[0], target))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if left := time.Since(restarted); left < 1400*time.Millisecond {
		t.Fatalf("the notice left %v after the last one, want about 1.5 s", left)
	}
}

// A hung player blocks the thread of a window it owns. Show must still return
// at once (the session keeps running) and Close must give up in bounded time
// instead of hanging the app, which is what a real hang did in development.
func TestHungPlayerNeverBlocksShowOrClose(t *testing.T) {
	testkit.Slow(t)
	player := startFakePlayer(t)
	s := openOver(t, player.pid())
	player.hang(t)
	time.Sleep(300 * time.Millisecond)
	// Make the overlay touch the hung owner: a resize request goes through
	// the player's (stuck) thread, and a redraw is queued behind it.
	go s.place(offscreenX+40, 80, 900, 500)

	start := time.Now()
	for i := range 50 {
		if err := s.screen.Show(session.Screen{Primary: fmt.Sprint("linha ", i)}); err != nil && !errors.Is(err, overlay.ErrPlayerNotResponding) {
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

	// The session ends a hung player; the overlay must then finish its
	// removal by itself: no window, no thread left, and Close succeeds.
	if err := player.cmd.Process.Kill(); err != nil {
		t.Fatalf("ending the hung player: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := s.screen.Close()
		if err == nil {
			break
		}
		if !errors.Is(err, overlay.ErrPlayerNotResponding) || time.Now().After(deadline) {
			t.Fatalf("Close after the hung player ended = %v", err)
		}
	}
	if left := overlayWindows(); len(left) != 0 {
		t.Fatalf("%d overlay windows left after the hung player ended", len(left))
	}
}

// Two overlays in one process each follow their own player: the callbacks
// they share must reach the right one.
func TestTwoOverlaysFollowTheirOwnPlayers(t *testing.T) {
	testkit.Slow(t)
	first := openOver(t, startFakePlayer(t).pid())
	second := openOverAnother(t, startFakePlayer(t).pid(), first.window)

	first.place(offscreenX+300, 200, 700, 420)
	first.waitUntil("the first overlay to follow its player", first.covering)
	if !second.covering() {
		t.Fatalf("the second overlay moved with the first player: %s", second.state())
	}
	second.place(offscreenX+900, 120, 500, 300)
	second.waitUntil("the second overlay to follow its player", second.covering)

	if err := first.screen.Close(); err != nil {
		t.Fatalf("closing the first overlay: %v", err)
	}
	second.place(offscreenX+950, 160, 520, 320)
	second.waitUntil("the second overlay to keep following after the first closed", second.covering)
}

// TestOverlayFollowsTheRealVLC checks the same ownership and geometry over
// the real VLC, including its fullscreen-like resize and quitting it.
func TestOverlayFollowsTheRealVLC(t *testing.T) {
	if os.Getenv("LINGOREEL_VLC_TEST") != "1" {
		t.Skip("set LINGOREEL_VLC_TEST=1 to run against the installed VLC")
	}
	var executable string
	for _, installation := range discovery.Detect(discovery.SystemEnvironment{}, []discovery.Hints{vlc.Plugin{}.Hints()}, nil) {
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

	// VLC fits its window to the video when the first frame arrives, which
	// can undo an early resize: repeat it until VLC keeps it.
	s.waitUntil("the overlay to follow VLC's resize", func() bool {
		player, err := win32.ClientRectOnScreen(s.target)
		if err == nil && player.Right-player.Left <= 320 {
			s.place(40, 40, 900, 560)
		}
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

	// Real fullscreen, not a resize. If VLC moved the video to
	// another window, the overlay would hide with the old one. Right after a
	// restore VLC refits its window to the video and was seen ignoring a
	// fullscreen request sent meanwhile, so the request waits for the window
	// to settle and is repeated a few times.
	s.waitUntil("VLC's window to settle after the restore", func() bool {
		before, _ := win32.ClientRectOnScreen(s.target)
		time.Sleep(500 * time.Millisecond)
		after, _ := win32.ClientRectOnScreen(s.target)
		return before == after
	})
	fullscreen := func() bool {
		player, err := win32.ClientRectOnScreen(s.target)
		return err == nil && player.Right-player.Left >= 1000
	}
	// VLC refuses the request while it has no video output (its script
	// answers an error), as after a restore to an empty window: it is asked
	// again until it takes it, for up to 10 s.
	var refused error
	for attempt, deadline := 1, time.Now().Add(10*time.Second); !fullscreen(); attempt++ {
		if time.Now().After(deadline) {
			t.Fatalf("VLC did not enter fullscreen in 10 s (%d requests, last refusal: %v); %s", attempt-1, refused, s.state())
		}
		if err := session.SetFullscreen(true); err != nil {
			refused = err
		}
		for wait := time.Now().Add(2 * time.Second); !fullscreen() && time.Now().Before(wait); {
			time.Sleep(100 * time.Millisecond)
		}
		if fullscreen() && attempt > 1 {
			t.Logf("VLC entered fullscreen on request %d", attempt)
		}
	}
	s.waitUntil("the overlay to cover VLC in fullscreen", s.covering)
	if err := session.SetFullscreen(false); err != nil {
		t.Fatalf("SetFullscreen(false): %v", err)
	}
	s.waitUntil("the overlay to follow VLC out of fullscreen", func() bool {
		if s.playerEmpty() {
			return s.hidden()
		}
		player, err := win32.ClientRectOnScreen(s.target)
		return err == nil && player.Right-player.Left < 1000 && s.covering()
	})

	if err := session.Close(); err != nil {
		t.Fatalf("closing VLC: %v", err)
	}
	s.waitForEnd()
}
