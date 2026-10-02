package shortcut

import (
	"errors"
	"fmt"
	"slices"
)

// Command is something a shortcut runs.
type Command string

// The commands with a shortcut. More arrive with the study shortcuts.
const (
	PlayPause              Command = "play-pause"
	SeekBack               Command = "seek-back"
	SeekForward            Command = "seek-forward"
	VolumeUp               Command = "volume-up"
	VolumeDown             Command = "volume-down"
	Mute                   Command = "mute"
	Fullscreen             Command = "fullscreen"
	ExitFullscreen         Command = "exit-fullscreen"
	Translation            Command = "translation"
	ReplayLine             Command = "replay-line"
	SyncPrimaryEarlier     Command = "sync-primary-earlier"
	SyncPrimaryLater       Command = "sync-primary-later"
	SyncTranslationEarlier Command = "sync-translation-earlier"
	SyncTranslationLater   Command = "sync-translation-later"
)

// Category groups the commands on the settings screen.
type Category string

const (
	CategoryPlayback    Category = "playback"
	CategorySubtitle    Category = "subtitle"
	CategoryTranslation Category = "translation"
	CategoryStudy       Category = "study"
)

// entry is a command with its category and default chord.
type entry struct {
	command  Command
	category Category
	chord    Chord
}

// commands are the commands in display order.
var commands = []entry{
	{PlayPause, CategoryPlayback, Chord{Key: KeySpace}},
	{SeekBack, CategoryPlayback, Chord{Key: KeyArrowLeft}},
	{SeekForward, CategoryPlayback, Chord{Key: KeyArrowRight}},
	{VolumeUp, CategoryPlayback, Chord{Key: KeyArrowUp}},
	{VolumeDown, CategoryPlayback, Chord{Key: KeyArrowDown}},
	{Mute, CategoryPlayback, Chord{Key: KeyM}},
	{Fullscreen, CategoryPlayback, Chord{Key: KeyF}},
	{ExitFullscreen, CategoryPlayback, Chord{Key: KeyEscape}},
	{SyncPrimaryEarlier, CategorySubtitle, Chord{Shift, KeyComma}},
	{SyncPrimaryLater, CategorySubtitle, Chord{Shift, KeyPeriod}},
	{SyncTranslationEarlier, CategorySubtitle, Chord{Ctrl, KeyComma}},
	{SyncTranslationLater, CategorySubtitle, Chord{Ctrl, KeyPeriod}},
	{Translation, CategoryTranslation, Chord{Key: KeyAltRight}},
	{ReplayLine, CategoryStudy, Chord{Key: KeyBackspace}},
}

// Commands lists every command in display order.
func Commands() []Command {
	list := make([]Command, len(commands))
	for i, c := range commands {
		list[i] = c.command
	}
	return list
}

// Known reports whether the command exists.
func (c Command) Known() bool {
	return slices.ContainsFunc(commands, func(e entry) bool { return e.command == c })
}

// Category is the command's group; empty for an unknown command.
func (c Command) Category() Category {
	for _, e := range commands {
		if e.command == c {
			return e.category
		}
	}
	return ""
}

// Repeats tells whether holding the shortcut repeats the command, as the
// keyboard repeats a held key: moving through the video and the volume do,
// switching something on and off does not.
func (c Command) Repeats() bool {
	switch c {
	case SeekBack, SeekForward, VolumeUp, VolumeDown:
		return true
	}
	return false
}

// ErrUnknownCommand reports a command outside Commands.
var ErrUnknownCommand = errors.New("unknown command")

// ConflictError reports two commands on the same chord.
type ConflictError struct {
	First, Second Command
	Chord         Chord
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s and %s share the shortcut %s", e.First, e.Second, e.Chord)
}

// Map gives each command its chord. Commands left out keep their default,
// so a profile saved before a command existed still gets it.
type Map map[Command]Chord

// Defaults are the shortcuts of a new profile.
func Defaults() Map {
	m := make(Map, len(commands))
	for _, c := range commands {
		m[c.command] = c.chord
	}
	return m
}

// Resolved is every command's chord: the map's own, or the default.
func (m Map) Resolved() Map {
	resolved := Defaults()
	for command, chord := range m {
		if command.Known() {
			resolved[command] = chord
		}
	}
	return resolved
}

// Of is a command's chord.
func (m Map) Of(command Command) Chord {
	if chord, found := m[command]; found {
		return chord
	}
	return Defaults()[command]
}

// Find is the command a chord runs.
func (m Map) Find(chord Chord) (Command, bool) {
	for _, command := range Commands() {
		if m.Of(command) == chord {
			return command, true
		}
	}
	return "", false
}

// Validate checks every chord and that no two commands share one, defaults
// included. A conflict is reported as a *ConflictError naming the commands
// in display order.
func (m Map) Validate() error {
	for command, chord := range m {
		if !command.Known() {
			return fmt.Errorf("%w: %q", ErrUnknownCommand, command)
		}
		if err := chord.Validate(); err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
	}
	owner := map[Chord]Command{}
	for _, command := range Commands() {
		chord := m.Of(command)
		if first, taken := owner[chord]; taken {
			return &ConflictError{First: first, Second: command, Chord: chord}
		}
		owner[chord] = command
	}
	return nil
}
