package hotkeys_test

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/windows/hotkeys"
	"lingoreel/internal/infrastructure/windows/win32"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindowEx = user32.NewProc("FindWindowExW")
	procPostMessage  = user32.NewProc("PostMessageW")
)

// vkF24 is a key no keyboard has: the tests can take Ctrl+Shift+F24 from
// whatever program is in front without the user noticing.
const vkF24 = 0x87

var unusedShortcut = hotkeys.Binding{Modifiers: win32.MOD_CONTROL | win32.MOD_SHIFT, Key: vkF24}

func TestBindingValidate(t *testing.T) {
	cases := []struct {
		binding hotkeys.Binding
		ok      bool
	}{
		{hotkeys.Binding{Modifiers: win32.MOD_SHIFT, Key: win32.VK_OEM_COMMA}, true},
		{hotkeys.Binding{Modifiers: win32.MOD_CONTROL | win32.MOD_ALT, Key: 'P'}, true},
		{hotkeys.Binding{Key: 'P'}, true},
		{hotkeys.Binding{Modifiers: win32.MOD_ALT | win32.MOD_SHIFT, Key: 'P'}, false},
		{hotkeys.Binding{Modifiers: win32.MOD_CONTROL | win32.MOD_ALT | win32.MOD_SHIFT, Key: 'P'}, false},
		{hotkeys.Binding{Modifiers: 0x0008, Key: 'P'}, false}, // the Windows key is not offered
		{hotkeys.Binding{Modifiers: win32.MOD_SHIFT}, false},
		{hotkeys.Binding{Key: 0xFF}, false},
	}
	for _, c := range cases {
		err := c.binding.Validate()
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, hotkeys.ErrInvalidBinding)) {
			t.Errorf("Validate(%+v) = %v, want ok %v", c.binding, err, c.ok)
		}
	}
}

func TestStartRejectsBadOptions(t *testing.T) {
	valid := []hotkeys.Binding{unusedShortcut}
	for name, options := range map[string]hotkeys.Options{
		"no process":       {Bindings: valid},
		"no shortcuts":     {ProcessID: os.Getpid()},
		"invalid shortcut": {ProcessID: os.Getpid(), Bindings: []hotkeys.Binding{{Key: 0}}},
		"repeated":         {ProcessID: os.Getpid(), Bindings: []hotkeys.Binding{unusedShortcut, unusedShortcut}},
	} {
		if keys, err := hotkeys.Start(options); err == nil {
			keys.Close()
			t.Errorf("%s: Start succeeded", name)
		}
	}
}

// foregroundProcess is the process of the window the user is working in.
func foregroundProcess(t *testing.T) int {
	t.Helper()
	var pid uint32
	if window := win32.ForegroundWindow(); window == 0 {
		t.Skip("no window in the foreground")
	} else if _, err := windows.GetWindowThreadProcessId(window, &pid); err != nil {
		t.Skip("foreground window without a process")
	}
	return int(pid)
}

// taken reports whether another window cannot register the combination,
// which is how Windows says a program holds it.
func taken(t *testing.T, binding hotkeys.Binding) bool {
	t.Helper()
	// Without a window the registration belongs to the calling thread, so
	// both calls must run on the same one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const probeID = 0x5150
	if err := win32.RegisterHotKey(0, probeID, binding.Modifiers, binding.Key); err != nil {
		return true
	}
	win32.UnregisterHotKey(0, probeID)
	return false
}

func TestShortcutsAreHeldOnlyForTheForegroundProcess(t *testing.T) {
	if taken(t, unusedShortcut) {
		t.Skip("another program holds Ctrl+Shift+F24")
	}
	// A process that is not in front: nothing may be taken from the others.
	away, err := hotkeys.Start(hotkeys.Options{ProcessID: os.Getpid(), Bindings: []hotkeys.Binding{unusedShortcut}})
	if err != nil {
		t.Fatal(err)
	}
	if taken(t, unusedShortcut) {
		away.Close()
		t.Fatal("a shortcut was registered while its player was not in the foreground")
	}
	if err := away.Close(); err != nil {
		t.Fatal(err)
	}

	pid := foregroundProcess(t)
	front, err := hotkeys.Start(hotkeys.Options{ProcessID: pid, Bindings: []hotkeys.Binding{unusedShortcut}})
	if err != nil {
		t.Fatal(err)
	}
	if foregroundProcess(t) != pid {
		front.Close()
		t.Skip("the foreground changed during the test")
	}
	if !taken(t, unusedShortcut) {
		front.Close()
		t.Fatal("the shortcut was not registered with its player in the foreground")
	}
	if err := front.Close(); err != nil {
		t.Fatal(err)
	}
	if taken(t, unusedShortcut) {
		t.Fatal("Close did not release the shortcut")
	}
	if err := front.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, open := <-front.Presses(); open {
		t.Fatal("Presses still open after Close")
	}
}

// With the app's own player window, the window decides, not the process:
// the app's other windows (the panel) share the process and must not take
// Shift+, and Ctrl+, from the user typing in them.
func TestShortcutsFollowThePlayerWindow(t *testing.T) {
	if taken(t, unusedShortcut) {
		t.Skip("another program holds Ctrl+Shift+F24")
	}
	pid := foregroundProcess(t)
	// The foreground process, but another window: nothing taken.
	elsewhere, err := hotkeys.Start(hotkeys.Options{ProcessID: pid, Window: uintptr(windows.GetDesktopWindow()), Bindings: []hotkeys.Binding{unusedShortcut}})
	if err != nil {
		t.Fatal(err)
	}
	if taken(t, unusedShortcut) {
		elsewhere.Close()
		t.Fatal("the shortcut was taken for the player's process while its window was not in front")
	}
	if err := elsewhere.Close(); err != nil {
		t.Fatal(err)
	}

	front := win32.ForegroundWindow()
	keys, err := hotkeys.Start(hotkeys.Options{Window: uintptr(front), Bindings: []hotkeys.Binding{unusedShortcut}})
	if err != nil {
		t.Fatal(err)
	}
	defer keys.Close()
	if win32.ForegroundWindow() != front {
		t.Skip("the foreground changed during the test")
	}
	if !taken(t, unusedShortcut) {
		t.Fatal("the shortcut was not registered with the player window in front")
	}
}

// messageWindow finds the hidden window that receives the shortcuts.
func messageWindow(t *testing.T) windows.HWND {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("LingoReelHotkeys")
	hwnd, _, _ := procFindWindowEx.Call(uintptr(win32.HWND_MESSAGE), 0, uintptr(unsafe.Pointer(class)), 0)
	if hwnd == 0 {
		t.Fatal("shortcuts window not found")
	}
	return windows.HWND(hwnd)
}

func press(t *testing.T, window windows.HWND, index int) {
	t.Helper()
	if r, _, err := procPostMessage.Call(uintptr(window), win32.WM_HOTKEY, uintptr(index), 0); r == 0 {
		t.Fatalf("PostMessage: %v", err)
	}
}

func next(t *testing.T, keys *hotkeys.Hotkeys) hotkeys.Press {
	t.Helper()
	select {
	case p := <-keys.Presses():
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("no press delivered")
		return hotkeys.Press{}
	}
}

// Windows repeats a held key; a press that comes while the key is still
// down is a repeat, one after its release is new. F24 is never down, so the
// release check clears it within its 50 ms poll.
func TestPressesTellRepeatsFromNewPresses(t *testing.T) {
	second := hotkeys.Binding{Modifiers: win32.MOD_CONTROL | win32.MOD_SHIFT, Key: vkF24 - 1}
	keys, err := hotkeys.Start(hotkeys.Options{ProcessID: os.Getpid(), Bindings: []hotkeys.Binding{unusedShortcut, second}})
	if err != nil {
		t.Fatal(err)
	}
	defer keys.Close()
	window := messageWindow(t)

	// Both posted before the loop runs: the second arrives while the first
	// is still "down".
	press(t, window, 1)
	press(t, window, 1)
	if got := next(t, keys); got != (hotkeys.Press{Index: 1}) {
		t.Fatalf("first press = %+v", got)
	}
	if got := next(t, keys); got != (hotkeys.Press{Index: 1, Repeat: true}) {
		t.Fatalf("press while down = %+v, want a repeat", got)
	}
	time.Sleep(200 * time.Millisecond) // past the release check
	press(t, window, 1)
	if got := next(t, keys); got != (hotkeys.Press{Index: 1}) {
		t.Fatalf("press after the release = %+v, want a new press", got)
	}
	// Unknown ids from a hostile sender are ignored.
	press(t, window, 99)
	press(t, window, -1)
	select {
	case got := <-keys.Presses():
		t.Fatalf("an unknown id was delivered as %+v", got)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestBindingOfAChord(t *testing.T) {
	binding, err := hotkeys.BindingOf(shortcut.Chord{Modifiers: shortcut.Ctrl, Key: shortcut.KeyComma})
	if err != nil || binding != (hotkeys.Binding{Modifiers: win32.MOD_CONTROL, Key: win32.VK_OEM_COMMA}) {
		t.Fatalf("BindingOf(Ctrl+Comma) = %+v, %v", binding, err)
	}
	if _, err := hotkeys.BindingOf(shortcut.Chord{Key: "Nope"}); !errors.Is(err, hotkeys.ErrInvalidBinding) {
		t.Fatalf("BindingOf an unknown key = %v", err)
	}
	// StartGlobal needs one chord per global command: the four sync steps
	// and the two passage marks.
	if _, err := hotkeys.StartGlobal(watch.ShortcutTarget{ProcessID: os.Getpid()}, nil, nil); !errors.Is(err, hotkeys.ErrInvalidBinding) {
		t.Fatalf("StartGlobal without chords = %v", err)
	}
	four := make([]shortcut.Chord, 4)
	if _, err := hotkeys.StartGlobal(watch.ShortcutTarget{ProcessID: os.Getpid()}, four, nil); !errors.Is(err, hotkeys.ErrInvalidBinding) {
		t.Fatalf("StartGlobal with the sync chords only = %v", err)
	}
}

func TestStartGlobalLeavesOutShortcutsWithoutAKey(t *testing.T) {
	// This process is not in front: nothing is taken from the user. Only the
	// last global command (the passage end) has a chord, and its presses keep
	// its own index.
	target := watch.ShortcutTarget{ProcessID: os.Getpid()}
	chords := []shortcut.Chord{{}, {}, {}, {}, {}, {Modifiers: shortcut.Ctrl | shortcut.Shift, Key: "F11"}}
	keys, err := hotkeys.StartGlobal(target, chords, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer keys.Close()
	press(t, messageWindow(t), 0)
	select {
	case got := <-keys.Presses():
		if got.Index != 5 {
			t.Fatalf("press of the only chord = %+v, want index 5", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no press delivered")
	}

	// None has a chord: nothing to register, and Close still ends the presses.
	none, err := hotkeys.StartGlobal(target, make([]shortcut.Chord, len(watch.GlobalCommands())), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(none.Close(), none.Close()); err != nil {
		t.Fatal(err)
	}
	if _, open := <-none.Presses(); open {
		t.Fatal("Presses still open after Close")
	}
}
