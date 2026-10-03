package app

import (
	"errors"

	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/presentation/ptbr"
)

// While a video plays, the app's panel takes the profile's shortcuts too,
// outside its text fields: the same commands the player window runs.

var (
	errUnknownCommand = errors.New("Atalho desconhecido.")
	errWindowOnly     = errors.New("Esse atalho funciona na janela do filme.")
)

// Shortcuts are the keys of the commands: the video's, or else the active
// profile's.
func (a *API) Shortcuts() []ShortcutSetting {
	if current := a.current(); current != nil && current.session != nil {
		return shortcutViews(current.session.Shortcuts(), a.deps.KeyChar)
	}
	a.mu.Lock()
	keys := a.profile().Shortcuts.Resolved()
	a.mu.Unlock()
	return shortcutViews(keys, a.deps.KeyChar)
}

// RunShortcut carries out a shortcut's command on the video; repeat tells
// the key is held, which only the commands that repeat take, as in the
// player window. The translation key goes through Reveal.
func (a *API) RunShortcut(command string, repeat bool) error {
	which := shortcut.Command(command)
	switch {
	case !which.Known():
		return errUnknownCommand
	case which == shortcut.Fullscreen || which == shortcut.ExitFullscreen || which == shortcut.Translation:
		return errWindowOnly
	case repeat && !which.Repeats():
		return nil // a held key: once is enough
	}
	return a.control(func(w *watching) error { return w.session.Run(which, repeat) })
}

// Reveal is the translation key going down (pressed) or up on the panel.
func (a *API) Reveal(pressed bool) error {
	return a.control(func(w *watching) error {
		w.session.Reveal(pressed)
		return nil
	})
}

// SetStudyMode changes the study mode of the video being watched; the
// profile keeps its own.
func (a *API) SetStudyMode(mode string) error {
	which := study.Mode(mode)
	if !which.Valid() {
		return errUnknownMode
	}
	return a.control(func(w *watching) error { return w.session.SetMode(which) })
}

// shortcutViews lists every command with its keys, in display order.
func shortcutViews(keys shortcut.Map, char ptbr.KeyChar) []ShortcutSetting {
	views := []ShortcutSetting{}
	for _, command := range shortcut.Commands() {
		chord := keys.Of(command)
		views = append(views, ShortcutSetting{
			Command: string(command), Name: ptbr.CommandName(command), Category: ptbr.CategoryName(command.Category()),
			Chord: chord.String(), Label: ptbr.ShortcutLabel(chord, char),
		})
	}
	return views
}
