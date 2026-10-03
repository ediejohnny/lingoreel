//go:build windows && amd64

package hotkeys

import (
	"fmt"
	"log/slog"
	"sync"

	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/windows/keyboard"
)

// BindingOf is the system-wide hotkey of a chord on the current keyboard
// layout.
func BindingOf(chord shortcut.Chord) (Binding, error) {
	key := keyboard.VirtualKey(chord.Key)
	if key == 0 {
		return Binding{}, fmt.Errorf("%w: no key %s on this keyboard", ErrInvalidBinding, chord.Key)
	}
	return Binding{Modifiers: keyboard.Modifiers(chord.Modifiers), Key: key}, nil
}

// syncShortcuts adapts Hotkeys to the session's sync shortcuts; keys is nil
// when none of them has a chord.
type syncShortcuts struct {
	keys      *Hotkeys
	presses   chan watch.Press
	closeOnce sync.Once
}

// StartSync listens to watch.SyncShortcuts, on the given chords, while the
// player is in the foreground: its own window, or else any window of its
// process. A shortcut without a chord is left out.
func StartSync(target watch.ShortcutTarget, chords []shortcut.Chord, logger *slog.Logger) (watch.Shortcuts, error) {
	if len(chords) != len(watch.SyncShortcuts) {
		return nil, fmt.Errorf("%w: %d chords for %d sync shortcuts", ErrInvalidBinding, len(chords), len(watch.SyncShortcuts))
	}
	var bindings []Binding
	var indices []int // the sync shortcut of each binding
	for i, chord := range chords {
		if chord.IsZero() {
			continue
		}
		binding, err := BindingOf(chord)
		if err != nil {
			return nil, err
		}
		bindings, indices = append(bindings, binding), append(indices, i)
	}
	s := &syncShortcuts{presses: make(chan watch.Press, pressBuffer)}
	if len(bindings) == 0 {
		return s, nil
	}
	keys, err := Start(Options{ProcessID: target.ProcessID, Window: target.Window, Bindings: bindings, Logger: logger})
	if err != nil {
		return nil, err
	}
	s.keys = keys
	// Ends when Close closes the key presses.
	go func() {
		defer close(s.presses)
		for press := range keys.Presses() {
			s.presses <- watch.Press{Index: indices[press.Index], Repeat: press.Repeat}
		}
	}()
	return s, nil
}

func (s *syncShortcuts) Presses() <-chan watch.Press { return s.presses }

func (s *syncShortcuts) Close() error {
	if s.keys == nil {
		s.closeOnce.Do(func() { close(s.presses) })
		return nil
	}
	return s.keys.Close()
}
