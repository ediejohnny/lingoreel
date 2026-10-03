// Package shortcut models the keyboard shortcuts: which key combination runs
// which command, and the rules every combination follows.
package shortcut

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// Errors of a combination that cannot be a shortcut.
var (
	ErrNoKey = errors.New("shortcut without a key")
	// ErrUnknownKey reports a key name outside Keys.
	ErrUnknownKey = errors.New("unknown key")
	// ErrTooManyKeys reports more than two modifiers: a shortcut has at
	// most three keys.
	ErrTooManyKeys = errors.New("more than three keys")
	// ErrAltShift reports Alt+Shift, which switches the keyboard layout on
	// Windows.
	ErrAltShift = errors.New("Alt+Shift switches the keyboard layout")
	// ErrReserved reports a combination Windows keeps for itself.
	ErrReserved = errors.New("combination reserved by Windows")
	// ErrAloneKey reports a key that only works alone, such as the right
	// Alt, used with modifiers.
	ErrAloneKey = errors.New("key must be pressed alone")
)

// Modifiers are the keys held with the main key.
type Modifiers uint8

const (
	Ctrl Modifiers = 1 << iota
	Alt
	Shift
)

// maxModifiers keeps shortcuts to three keys: two modifiers and a key.
const maxModifiers = 2

// modifierNames are the modifiers in the order they are written.
var modifierNames = []struct {
	modifier Modifiers
	name     string
}{{Ctrl, "Ctrl"}, {Alt, "Alt"}, {Shift, "Shift"}}

// Chord is a key combination: modifiers and one key, matched by the key's
// position on the keyboard, not its character, so Shift+, works on layouts
// where it types "<" (ABNT2).
type Chord struct {
	Modifiers Modifiers
	Key       Key
}

// Parse reads a chord written as "Ctrl+Shift+Comma": modifiers in any
// order, then the key, separated by "+".
func Parse(text string) (Chord, error) {
	parts := strings.Split(strings.TrimSpace(text), "+")
	var chord Chord
	for _, part := range parts[:len(parts)-1] {
		modifier, found := modifierNamed(part)
		if !found {
			return Chord{}, fmt.Errorf("%w: %q", ErrUnknownKey, part)
		}
		if chord.Modifiers&modifier != 0 {
			return Chord{}, fmt.Errorf("%w: %s repeated", ErrUnknownKey, part)
		}
		chord.Modifiers |= modifier
	}
	chord.Key = Key(parts[len(parts)-1])
	if err := chord.Validate(); err != nil {
		return Chord{}, err
	}
	return chord, nil
}

func modifierNamed(name string) (Modifiers, bool) {
	for _, m := range modifierNames {
		if m.name == name {
			return m.modifier, true
		}
	}
	return 0, false
}

// String writes the chord as Parse reads it, modifiers in a fixed order.
func (c Chord) String() string {
	var b strings.Builder
	for _, m := range modifierNames {
		if c.Modifiers&m.modifier != 0 {
			b.WriteString(m.name)
			b.WriteByte('+')
		}
	}
	b.WriteString(string(c.Key))
	return b.String()
}

// IsZero reports the empty chord: no shortcut.
func (c Chord) IsZero() bool {
	return c == Chord{}
}

// Has reports whether the chord holds a modifier.
func (c Chord) Has(modifier Modifiers) bool {
	return c.Modifiers&modifier != 0
}

// Validate applies the rules every shortcut follows.
func (c Chord) Validate() error {
	switch {
	case c.Key == "":
		return ErrNoKey
	case !c.Key.Known():
		return fmt.Errorf("%w: %q", ErrUnknownKey, c.Key)
	case c.Modifiers&^(Ctrl|Alt|Shift) != 0 || bits.OnesCount8(uint8(c.Modifiers)) > maxModifiers:
		return ErrTooManyKeys
	case c.Has(Alt) && c.Has(Shift):
		return ErrAltShift
	case c.Key.Alone() && c.Modifiers != 0:
		return fmt.Errorf("%w: %s", ErrAloneKey, c.Key)
	}
	if slices.Contains(reservedChords, c) {
		return fmt.Errorf("%w: %s", ErrReserved, c)
	}
	return nil
}

// reservedChords are taken by Windows before any program sees them, or
// close windows and switch programs. F12 is kept for debuggers and cannot
// be a system-wide hotkey.
var reservedChords = []Chord{
	{Alt, KeyF4}, {Alt, KeyTab}, {Alt, KeyEscape}, {Alt, KeySpace},
	{Ctrl, KeyEscape}, {0, KeyF12},
}

// MarshalText implements encoding.TextMarshaler; no shortcut is written as
// an empty text.
func (c Chord) MarshalText() ([]byte, error) {
	if c.IsZero() {
		return []byte{}, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return []byte(c.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler; an empty text is no
// shortcut.
func (c *Chord) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*c = Chord{}
		return nil
	}
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}
