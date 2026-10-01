//go:build windows && amd64

package hotkeys

import (
	"log/slog"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/infrastructure/windows/win32"
)

// SyncBinding is the default key of a sync shortcut: Shift for the line on
// top, Ctrl for the one below; comma moves it earlier and period later,
// matching "<" and ">" on the same keys.
func SyncBinding(shortcut watch.SyncShortcut) Binding {
	binding := Binding{Modifiers: win32.MOD_SHIFT, Key: win32.VK_OEM_COMMA}
	if shortcut.Line == session.LineTranslation {
		binding.Modifiers = win32.MOD_CONTROL
	}
	if shortcut.Later {
		binding.Key = win32.VK_OEM_PERIOD
	}
	return binding
}

// syncShortcuts adapts Hotkeys to the session's sync shortcuts.
type syncShortcuts struct {
	keys    *Hotkeys
	presses chan watch.Press
}

// StartSync listens to watch.SyncShortcuts while the player is in the
// foreground: its own window, or else any window of its process.
func StartSync(target watch.ShortcutTarget, logger *slog.Logger) (watch.Shortcuts, error) {
	bindings := make([]Binding, len(watch.SyncShortcuts))
	for i, shortcut := range watch.SyncShortcuts {
		bindings[i] = SyncBinding(shortcut)
	}
	keys, err := Start(Options{ProcessID: target.ProcessID, Window: target.Window, Bindings: bindings, Logger: logger})
	if err != nil {
		return nil, err
	}
	s := &syncShortcuts{keys: keys, presses: make(chan watch.Press, pressBuffer)}
	// Ends when Close closes the key presses.
	go func() {
		defer close(s.presses)
		for press := range keys.Presses() {
			s.presses <- watch.Press{Index: press.Index, Repeat: press.Repeat}
		}
	}()
	return s, nil
}

func (s *syncShortcuts) Presses() <-chan watch.Press { return s.presses }

func (s *syncShortcuts) Close() error { return s.keys.Close() }
