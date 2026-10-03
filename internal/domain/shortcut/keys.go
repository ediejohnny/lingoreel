package shortcut

import (
	"maps"
	"slices"
)

// Key names a key by its position on the keyboard, with the names the web
// uses for KeyboardEvent.code ("KeyM", "Comma", "ArrowLeft"), so the app
// window can capture a shortcut and the player window can match it.
type Key string

// Keys used by the default shortcuts and the rules.
const (
	KeySpace      Key = "Space"
	KeyEscape     Key = "Escape"
	KeyBackspace  Key = "Backspace"
	KeyTab        Key = "Tab"
	KeyArrowLeft  Key = "ArrowLeft"
	KeyArrowRight Key = "ArrowRight"
	KeyArrowUp    Key = "ArrowUp"
	KeyArrowDown  Key = "ArrowDown"
	KeyComma      Key = "Comma"
	KeyPeriod     Key = "Period"
	KeyF          Key = "KeyF"
	KeyM          Key = "KeyM"
	KeyY          Key = "KeyY"
	KeyZ          Key = "KeyZ"
	KeyDigit0     Key = "Digit0"
	KeyDigit3     Key = "Digit3"
	KeyDigit5     Key = "Digit5"
	KeyF4         Key = "F4"
	KeyF9         Key = "F9"
	KeyF12        Key = "F12"
	// KeyAltRight is the right Alt (AltGr on some layouts), a key of its
	// own for the translation, used alone.
	KeyAltRight Key = "AltRight"
)

// keys lists every key a shortcut may use.
var keys = func() map[Key]bool {
	known := map[Key]bool{}
	add := func(names ...Key) {
		for _, name := range names {
			known[name] = true
		}
	}
	for c := 'A'; c <= 'Z'; c++ {
		add(Key("Key" + string(c)))
	}
	for c := '0'; c <= '9'; c++ {
		add(Key("Digit"+string(c)), Key("Numpad"+string(c)))
	}
	for _, f := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"} {
		add(Key("F" + f))
	}
	add(KeySpace, KeyEscape, KeyBackspace, KeyTab, "Enter", "Insert", "Delete", "Home", "End", "PageUp", "PageDown",
		KeyArrowLeft, KeyArrowRight, KeyArrowUp, KeyArrowDown,
		KeyComma, KeyPeriod, "Minus", "Equal", "Semicolon", "Quote", "Slash", "Backslash",
		"BracketLeft", "BracketRight", "Backquote", "IntlBackslash", "IntlRo",
		"NumpadAdd", "NumpadSubtract", "NumpadMultiply", "NumpadDivide", "NumpadDecimal", "NumpadEnter",
		KeyAltRight)
	return known
}()

// Keys lists every key a shortcut may use, sorted by name.
func Keys() []Key {
	return slices.Sorted(maps.Keys(keys))
}

// Known reports whether a shortcut may use the key.
func (k Key) Known() bool {
	return keys[k]
}

// Alone reports whether the key only works without modifiers: the right
// Alt is a modifier itself (AltGr adds Ctrl to it on some layouts).
func (k Key) Alone() bool {
	return k == KeyAltRight
}
