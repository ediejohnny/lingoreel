//go:build windows && amd64

package keyboard_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/keyboard"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// keyLParam builds a key message's lParam for a scan code.
func keyLParam(scan uintptr, extended bool) uintptr {
	lparam := 1 | scan<<16
	if extended {
		lparam |= 1 << 24
	}
	return lparam
}

func TestEveryKeyHasOnePlaceOnTheKeyboard(t *testing.T) {
	keys := shortcut.Keys()
	// 26 letters + 10 digits + 10 keypad digits + 12 F keys + 11 named
	// (Space, Escape, Backspace, Tab, Enter, Insert, Delete, Home, End,
	// PageUp, PageDown) + 4 arrows + 13 punctuation + 6 keypad keys
	// (+ - * / , Enter) + the right Alt = 93.
	if len(keys) != 93 {
		t.Fatalf("%d keys, want 93", len(keys))
	}
	for _, key := range keys {
		if keyboard.VirtualKey(key) == 0 {
			t.Errorf("%s has no virtual key on this layout", key)
		}
	}
	// Every scan code maps back to one key: no two keys share a place.
	seen := map[shortcut.Key]bool{}
	for scan := range uintptr(0x80) {
		for _, extended := range []bool{false, true} {
			key, found := keyboard.KeyOfMessage(keyLParam(scan, extended))
			if !found {
				continue
			}
			if seen[key] {
				t.Errorf("%s at two places", key)
			}
			seen[key] = true
		}
	}
	for _, key := range keys {
		if !seen[key] {
			t.Errorf("%s has no scan code", key)
		}
	}
}

func TestKnownPlaces(t *testing.T) {
	cases := []struct {
		scan     uintptr
		extended bool
		want     shortcut.Key
	}{
		{0x39, false, shortcut.KeySpace},
		{0x33, false, shortcut.KeyComma},
		{0x34, false, shortcut.KeyPeriod},
		{0x32, false, shortcut.KeyM},
		{0x0E, false, shortcut.KeyBackspace},
		{0x4B, true, shortcut.KeyArrowLeft},
		// Same scan code, not extended: the keypad's 4, whatever Num Lock.
		{0x4B, false, "Numpad4"},
		{0x38, true, shortcut.KeyAltRight},
		{0x1C, true, "NumpadEnter"},
		{0x1C, false, "Enter"},
		{0x3F, false, "F5"},
		{0x58, false, shortcut.KeyF12},
		{0x02, false, "Digit1"},
		{0x0B, false, "Digit0"},
	}
	for _, c := range cases {
		got, found := keyboard.KeyOfMessage(keyLParam(c.scan, c.extended))
		if !found || got != c.want {
			t.Errorf("scan %#x extended %v = %q %v, want %q", c.scan, c.extended, got, found, c.want)
		}
	}
	// The left Alt, the Ctrl keys and the Shift keys are modifiers, not keys.
	for _, scan := range []uintptr{0x38, 0x1D, 0x2A, 0x36} {
		if key, found := keyboard.KeyOfMessage(keyLParam(scan, false)); found {
			t.Errorf("modifier scan %#x read as %s", scan, key)
		}
	}
	if !keyboard.Repeated(keyLParam(0x39, false)|1<<30) || keyboard.Repeated(keyLParam(0x39, false)) {
		t.Fatal("Repeated reads the wrong bit")
	}
}

func TestVirtualKeysForHotkeys(t *testing.T) {
	// Keys the layout cannot move.
	cases := map[shortcut.Key]uint32{
		shortcut.KeySpace: 0x20, shortcut.KeyArrowLeft: 0x25, shortcut.KeyArrowDown: 0x28,
		"F5": 0x74, "F12": 0x7B, "Numpad3": 0x63, "NumpadAdd": 0x6B, "NumpadEnter": 0x0D,
		shortcut.KeyAltRight: 0xA5, shortcut.KeyEscape: 0x1B, shortcut.KeyBackspace: 0x08,
		// Comma and period are the same keys on US and ABNT2 layouts.
		shortcut.KeyComma: win32.VK_OEM_COMMA, shortcut.KeyPeriod: win32.VK_OEM_PERIOD,
	}
	for key, want := range cases {
		if got := keyboard.VirtualKey(key); got != want {
			t.Errorf("VirtualKey(%s) = %#x, want %#x", key, got, want)
		}
	}
	if keyboard.VirtualKey("Nope") != 0 {
		t.Fatal("an unknown key has a virtual key")
	}
	if got := keyboard.Modifiers(shortcut.Ctrl | shortcut.Shift); got != win32.MOD_CONTROL|win32.MOD_SHIFT {
		t.Fatalf("Modifiers = %#x", got)
	}
	if got := keyboard.Modifiers(shortcut.Alt); got != win32.MOD_ALT {
		t.Fatalf("Modifiers(Alt) = %#x", got)
	}
}

func TestChordOfIgnoresModifiersOnlyForTheRightAlt(t *testing.T) {
	// AltGr sends a Ctrl with the right Alt.
	if got := keyboard.ChordOf(shortcut.KeyAltRight, shortcut.Ctrl); got != (shortcut.Chord{Key: shortcut.KeyAltRight}) {
		t.Fatalf("right Alt with Ctrl = %+v", got)
	}
	if got := keyboard.ChordOf(shortcut.KeyComma, shortcut.Ctrl); got != (shortcut.Chord{Modifiers: shortcut.Ctrl, Key: shortcut.KeyComma}) {
		t.Fatalf("Ctrl+Comma = %+v", got)
	}
	if got := keyboard.Char(shortcut.KeyComma); got != "," {
		t.Fatalf("Char(Comma) = %q", got)
	}
	if got := keyboard.Char(shortcut.KeyF4); got != "" {
		t.Fatalf("Char(F4) = %q, want none", got)
	}
}
