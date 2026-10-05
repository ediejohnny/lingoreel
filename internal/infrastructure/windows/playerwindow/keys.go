//go:build windows && amd64

package playerwindow

// The keys on the video: each goes to its command by the profile's keys,
// and the session runs it, as the panel's keys do.

import (
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/keyboard"
)

// keyDown runs the command of a key going down, by the profile's
// shortcuts, and reports whether the window took the key. For a window of
// its own the sync shortcuts never get here (their system-wide hotkeys take
// them first); on a stage there are none, and every key comes here.
func (w *Window) keyDown(lparam uintptr) bool {
	key, known := keyboard.KeyOfMessage(lparam)
	if !known {
		return false
	}
	// Esc closes the speed list or a menu first, whatever its shortcut is.
	if key == shortcut.KeyEscape && (w.speedMenu || w.openMenu != controlbar.MenuNone) {
		w.speedMenu, w.openMenu = false, controlbar.MenuNone
		w.pointer(w.cursorInZone())
		return true
	}
	chord := keyboard.ChordOf(key, keyboard.HeldModifiers())
	command, found := w.shortcuts().Find(chord)
	if !found {
		// Shift+F10 opens the menu, as in every Windows program, unless the
		// profile gave it to a command. Its release, F10's, is always taken
		// (keyUp), so Windows would never open the menu by itself.
		if chord == (shortcut.Chord{Modifiers: shortcut.Shift, Key: shortcut.KeyF10}) {
			w.openContextMenu(keyboardPoint)
			return true
		}
		return false
	}
	repeat := keyboard.Repeated(lparam)
	if repeat && !command.Repeats() {
		return true // a held key: once is enough
	}
	return w.run(command, key, repeat)
}

// keyUp sends the translation key's release; the others need none. F10's
// release is always taken: Windows turns it into the key that opens the
// window's menu, which this window has none of, and the next key pressed
// would be swallowed by that menu mode (Ctrl+F10 marks a passage, and its
// press may reach the window or the system-wide hotkey).
func (w *Window) keyUp(lparam uintptr) bool {
	key, known := keyboard.KeyOfMessage(lparam)
	if !known {
		return false
	}
	if key == shortcut.KeyF10 {
		return true
	}
	if key != w.revealKey {
		return false
	}
	w.releaseReveal()
	return true
}

// releaseReveal lets go of the translation key: on its release, or when
// the window loses the keyboard with the key down.
func (w *Window) releaseReveal() {
	if w.revealKey == "" {
		return
	}
	w.revealKey = ""
	w.do(func(r watch.Remote) error { r.Reveal(false); return nil })
}

// run carries out a command and reports whether it applied (leaving the
// fullscreen when not in it does not). The window keeps the fullscreen and
// the translation key, whose release it hears; the session runs the rest.
func (w *Window) run(command shortcut.Command, key shortcut.Key, repeat bool) bool {
	switch command {
	case shortcut.Translation:
		w.revealKey = key
		w.do(func(r watch.Remote) error { r.Reveal(true); return nil })
	case shortcut.Fullscreen:
		w.setFullscreen(!w.fullscreen)
	case shortcut.ExitFullscreen:
		if !w.fullscreen {
			return false
		}
		w.setFullscreen(false)
	default:
		w.do(func(r watch.Remote) error { return r.Run(command, repeat) })
	}
	return true
}
