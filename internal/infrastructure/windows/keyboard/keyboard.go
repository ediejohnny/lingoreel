//go:build windows && amd64

// Package keyboard turns the shortcut model's keys, named by their position
// on the keyboard, into what Windows reports and registers: scan codes for
// key messages and virtual keys for system-wide hotkeys.
package keyboard

import (
	"strconv"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// position is a key's scan code (set 1), extended for the keys Windows
// reports with the 0xE0 prefix.
type position struct {
	scan     uint32
	extended bool
}

// Virtual keys that do not depend on the keyboard layout.
const (
	vkNumpad0   = 0x60
	vkMultiply  = 0x6A
	vkAdd       = 0x6B
	vkSubtract  = 0x6D
	vkDecimal   = 0x6E
	vkDivide    = 0x6F
	vkF1        = 0x70
	vkReturn    = 0x0D
	vkRightMenu = 0xA5
)

// positions are the scan codes of every key the model knows.
var positions = func() map[shortcut.Key]position {
	p := map[shortcut.Key]position{
		"Escape": {0x01, false}, "Minus": {0x0C, false}, "Equal": {0x0D, false}, "Backspace": {0x0E, false},
		"Tab": {0x0F, false}, "BracketLeft": {0x1A, false}, "BracketRight": {0x1B, false}, "Enter": {0x1C, false},
		"Semicolon": {0x27, false}, "Quote": {0x28, false}, "Backquote": {0x29, false}, "Backslash": {0x2B, false},
		"Comma": {0x33, false}, "Period": {0x34, false}, "Slash": {0x35, false}, "NumpadMultiply": {0x37, false},
		"Space": {0x39, false}, "NumpadSubtract": {0x4A, false}, "NumpadAdd": {0x4E, false},
		"NumpadDecimal": {0x53, false}, "IntlBackslash": {0x56, false}, "F11": {0x57, false}, "F12": {0x58, false},
		"IntlRo":      {0x73, false},
		"NumpadEnter": {0x1C, true}, "NumpadDivide": {0x35, true}, "AltRight": {0x38, true},
		"Home": {0x47, true}, "ArrowUp": {0x48, true}, "PageUp": {0x49, true}, "ArrowLeft": {0x4B, true},
		"ArrowRight": {0x4D, true}, "End": {0x4F, true}, "ArrowDown": {0x50, true}, "PageDown": {0x51, true},
		"Insert": {0x52, true}, "Delete": {0x53, true},
	}
	rows := []struct {
		first uint32
		keys  string
	}{{0x10, "QWERTYUIOP"}, {0x1E, "ASDFGHJKL"}, {0x2C, "ZXCVBNM"}}
	for _, row := range rows {
		for i, letter := range row.keys {
			p[shortcut.Key("Key"+string(letter))] = position{row.first + uint32(i), false}
		}
	}
	// Digit1..Digit9 then Digit0, on 0x02..0x0B.
	for i, digit := range "1234567890" {
		p[shortcut.Key("Digit"+string(digit))] = position{0x02 + uint32(i), false}
	}
	// F1..F10 on 0x3B..0x44.
	for i := range 10 {
		p[shortcut.Key("F"+strconv.Itoa(i+1))] = position{0x3B + uint32(i), false}
	}
	// The keypad digits sit in rows: 7 8 9 / 4 5 6 / 1 2 3 / 0.
	for digit, scan := range map[string]uint32{"7": 0x47, "8": 0x48, "9": 0x49, "4": 0x4B, "5": 0x4C, "6": 0x4D, "1": 0x4F, "2": 0x50, "3": 0x51, "0": 0x52} {
		p[shortcut.Key("Numpad"+digit)] = position{scan, false}
	}
	return p
}()

// keysAt is positions turned around.
var keysAt = func() map[position]shortcut.Key {
	keys := make(map[position]shortcut.Key, len(positions))
	for key, at := range positions {
		keys[at] = key
	}
	return keys
}()

// KeyAt is the key at a scan code, as a key message reports it in its
// lParam (bits 16 to 23, and bit 24 for extended keys).
func KeyAt(scan uint32, extended bool) (shortcut.Key, bool) {
	key, found := keysAt[position{scan, extended}]
	return key, found
}

// KeyOfMessage is the key of a WM_KEYDOWN, WM_KEYUP or their WM_SYS forms.
func KeyOfMessage(lparam uintptr) (shortcut.Key, bool) {
	return KeyAt(uint32(lparam>>16)&0xFF, lparam&(1<<24) != 0)
}

// Repeated tells a key message comes from a held key (bit 30: the key was
// already down).
func Repeated(lparam uintptr) bool {
	return lparam&(1<<30) != 0
}

// VirtualKey is the virtual key RegisterHotKey needs for a key on the
// current keyboard layout; zero when the layout has none.
func VirtualKey(key shortcut.Key) uint32 {
	at, found := positions[key]
	if !found {
		return 0
	}
	if vk, fixed := fixedVirtualKey(key); fixed {
		return vk
	}
	return win32.VirtualKeyOfScan(at.scan, at.extended)
}

// fixedVirtualKey covers the keys the layout cannot change and that scan
// codes alone do not tell apart (the keypad digits share their scan codes
// with the arrows).
func fixedVirtualKey(key shortcut.Key) (uint32, bool) {
	switch key {
	case "NumpadMultiply":
		return vkMultiply, true
	case "NumpadAdd":
		return vkAdd, true
	case "NumpadSubtract":
		return vkSubtract, true
	case "NumpadDecimal":
		return vkDecimal, true
	case "NumpadDivide":
		return vkDivide, true
	case "NumpadEnter":
		return vkReturn, true
	case "AltRight":
		return vkRightMenu, true
	}
	if digit, found := strings.CutPrefix(string(key), "Numpad"); found && len(digit) == 1 {
		return vkNumpad0 + uint32(digit[0]-'0'), true
	}
	if number, found := strings.CutPrefix(string(key), "F"); found {
		if n, err := strconv.Atoi(number); err == nil && n >= 1 && n <= 12 {
			return vkF1 + uint32(n-1), true
		}
	}
	return 0, false
}

// Char is the character a key types on the current keyboard layout, such
// as "ç" for the key right of L on ABNT2; empty when it types none.
func Char(key shortcut.Key) string {
	vk := VirtualKey(key)
	if vk == 0 {
		return ""
	}
	if char := win32.CharOfVirtualKey(vk); char != 0 && char >= ' ' {
		return string(char)
	}
	return ""
}

// Modifiers converts a chord's modifiers into RegisterHotKey's.
func Modifiers(m shortcut.Modifiers) uint32 {
	var mod uint32
	if m&shortcut.Ctrl != 0 {
		mod |= win32.MOD_CONTROL
	}
	if m&shortcut.Alt != 0 {
		mod |= win32.MOD_ALT
	}
	if m&shortcut.Shift != 0 {
		mod |= win32.MOD_SHIFT
	}
	return mod
}

// Virtual keys of the modifiers, for the state of a key message.
const (
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
)

// HeldModifiers are the modifiers down when the key message being handled
// was sent.
func HeldModifiers() shortcut.Modifiers {
	var m shortcut.Modifiers
	if win32.KeyHeld(vkControl) {
		m |= shortcut.Ctrl
	}
	if win32.KeyHeld(vkMenu) {
		m |= shortcut.Alt
	}
	if win32.KeyHeld(vkShift) {
		m |= shortcut.Shift
	}
	return m
}

// ChordOf is the chord a key message makes with the modifiers held. A key
// that only works alone ignores them: AltGr sends a Ctrl with the right Alt.
func ChordOf(key shortcut.Key, held shortcut.Modifiers) shortcut.Chord {
	if key.Alone() {
		return shortcut.Chord{Key: key}
	}
	return shortcut.Chord{Modifiers: held, Key: key}
}
