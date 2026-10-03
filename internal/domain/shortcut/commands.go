package shortcut

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Command is something a shortcut runs.
type Command string

// The commands with a shortcut.
const (
	PlayPause              Command = "play-pause"
	VolumeUp               Command = "volume-up"
	VolumeDown             Command = "volume-down"
	Mute                   Command = "mute"
	Fullscreen             Command = "fullscreen"
	ExitFullscreen         Command = "exit-fullscreen"
	SeekBack               Command = "seek-back"
	SeekForward            Command = "seek-forward"
	SeekBack10             Command = "seek-back-10"
	SeekForward10          Command = "seek-forward-10"
	SeekBack30             Command = "seek-back-30"
	SeekForward30          Command = "seek-forward-30"
	PrecisionBack          Command = "precision-back"
	PrecisionForward       Command = "precision-forward"
	FrameBack              Command = "frame-back"
	FrameForward           Command = "frame-forward"
	SpeedDown              Command = "speed-down"
	SpeedUp                Command = "speed-up"
	SpeedNormal            Command = "speed-normal"
	SyncPrimaryEarlier     Command = "sync-primary-earlier"
	SyncPrimaryLater       Command = "sync-primary-later"
	SyncTranslationEarlier Command = "sync-translation-earlier"
	SyncTranslationLater   Command = "sync-translation-later"
	Translation            Command = "translation"
	ShowBoth               Command = "show-both"
	PrimaryOnly            Command = "primary-only"
	ReplayLine             Command = "replay-line"
	RepeatPhrase           Command = "repeat-phrase"
	RepeatLast3            Command = "repeat-last-3"
	RepeatLast5            Command = "repeat-last-5"
	RepeatLast10           Command = "repeat-last-10"
	// PassageMarkStart marks where a passage begins; PassageMarkEnd marks
	// where it ends, saves it and starts studying it; PassageStop leaves it.
	PassageMarkStart Command = "passage-mark-start"
	PassageMarkEnd   Command = "passage-mark-end"
	PassageStop      Command = "passage-stop"
	Undo             Command = "undo"
	Redo             Command = "redo"
)

// Category groups the commands on the settings screen.
type Category string

const (
	CategoryPlayback    Category = "playback"
	CategorySeek        Category = "seek"
	CategoryFrame       Category = "frame"
	CategorySpeed       Category = "speed"
	CategorySubtitle    Category = "subtitle"
	CategoryTranslation Category = "translation"
	CategoryStudy       Category = "study"
	CategoryEditor      Category = "editor"
)

// entry is a command with its category and default chord; a zero chord
// leaves the command without a shortcut until the user gives it one.
type entry struct {
	command  Command
	category Category
	chord    Chord
}

// commands are the commands in display order.
var commands = []entry{
	{PlayPause, CategoryPlayback, Chord{Key: KeySpace}},
	{VolumeUp, CategoryPlayback, Chord{Key: KeyArrowUp}},
	{VolumeDown, CategoryPlayback, Chord{Key: KeyArrowDown}},
	{Mute, CategoryPlayback, Chord{Key: KeyM}},
	{Fullscreen, CategoryPlayback, Chord{Key: KeyF}},
	{ExitFullscreen, CategoryPlayback, Chord{Key: KeyEscape}},
	{SeekBack, CategorySeek, Chord{Key: KeyArrowLeft}},
	{SeekForward, CategorySeek, Chord{Key: KeyArrowRight}},
	{SeekBack10, CategorySeek, Chord{Shift, KeyArrowLeft}},
	{SeekForward10, CategorySeek, Chord{Shift, KeyArrowRight}},
	{SeekBack30, CategorySeek, Chord{Ctrl, KeyArrowLeft}},
	{SeekForward30, CategorySeek, Chord{Ctrl, KeyArrowRight}},
	{PrecisionBack, CategorySeek, Chord{Alt, KeyArrowLeft}},
	{PrecisionForward, CategorySeek, Chord{Alt, KeyArrowRight}},
	{FrameBack, CategoryFrame, Chord{Key: KeyComma}},
	{FrameForward, CategoryFrame, Chord{Key: KeyPeriod}},
	{SpeedDown, CategorySpeed, Chord{Ctrl, KeyArrowDown}},
	{SpeedUp, CategorySpeed, Chord{Ctrl, KeyArrowUp}},
	{SpeedNormal, CategorySpeed, Chord{Ctrl, KeyDigit0}},
	{SyncPrimaryEarlier, CategorySubtitle, Chord{Shift, KeyComma}},
	{SyncPrimaryLater, CategorySubtitle, Chord{Shift, KeyPeriod}},
	{SyncTranslationEarlier, CategorySubtitle, Chord{Ctrl, KeyComma}},
	{SyncTranslationLater, CategorySubtitle, Chord{Ctrl, KeyPeriod}},
	{Translation, CategoryTranslation, Chord{Key: KeyAltRight}},
	{ShowBoth, CategoryTranslation, Chord{}},
	{PrimaryOnly, CategoryTranslation, Chord{}},
	{ReplayLine, CategoryStudy, Chord{Key: KeyBackspace}},
	{RepeatPhrase, CategoryStudy, Chord{Key: KeyF9}},
	{RepeatLast3, CategoryStudy, Chord{Key: KeyDigit3}},
	{RepeatLast5, CategoryStudy, Chord{Key: KeyDigit5}},
	{RepeatLast10, CategoryStudy, Chord{Key: KeyDigit0}},
	{PassageMarkStart, CategoryStudy, Chord{Ctrl, KeyF9}},
	{PassageMarkEnd, CategoryStudy, Chord{Ctrl, KeyF10}},
	{PassageStop, CategoryStudy, Chord{}},
	{Undo, CategoryEditor, Chord{Ctrl, KeyZ}},
	{Redo, CategoryEditor, Chord{Ctrl, KeyY}},
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
// keyboard repeats a held key: moving through the video, the volume and the
// sync steps do (a held sync key takes the large step); switching something
// on and off does not.
func (c Command) Repeats() bool {
	if _, jumps := c.Jump(); jumps {
		return true
	}
	switch c {
	case PrecisionBack, PrecisionForward, FrameBack, FrameForward, VolumeUp, VolumeDown,
		SyncPrimaryEarlier, SyncPrimaryLater, SyncTranslationEarlier, SyncTranslationLater:
		return true
	}
	return false
}

// Jump is how far a seek command moves, backward when negative.
func (c Command) Jump() (time.Duration, bool) {
	switch c {
	case SeekBack:
		return -5 * time.Second, true
	case SeekForward:
		return 5 * time.Second, true
	case SeekBack10:
		return -10 * time.Second, true
	case SeekForward10:
		return 10 * time.Second, true
	case SeekBack30:
		return -30 * time.Second, true
	case SeekForward30:
		return 30 * time.Second, true
	}
	return 0, false
}

// Rewind is how much a "repeat the last seconds" command plays again.
func (c Command) Rewind() (time.Duration, bool) {
	switch c {
	case RepeatLast3:
		return 3 * time.Second, true
	case RepeatLast5:
		return 5 * time.Second, true
	case RepeatLast10:
		return 10 * time.Second, true
	}
	return 0, false
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

// Map gives each command its chord; a zero chord is a command without a
// shortcut. Commands left out keep their default, so a profile saved before
// a command existed still gets it.
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

// Of is a command's chord, zero when it has no shortcut.
func (m Map) Of(command Command) Chord {
	if chord, found := m[command]; found {
		return chord
	}
	return Defaults()[command]
}

// Find is the command a chord runs; no chord runs a command without a
// shortcut.
func (m Map) Find(chord Chord) (Command, bool) {
	if chord.IsZero() {
		return "", false
	}
	for _, command := range Commands() {
		if m.Of(command) == chord {
			return command, true
		}
	}
	return "", false
}

// Validate checks every chord and that no two commands share one, defaults
// included; commands without a shortcut share nothing. A conflict is
// reported as a *ConflictError naming the commands in display order.
func (m Map) Validate() error {
	for command, chord := range m {
		if !command.Known() {
			return fmt.Errorf("%w: %q", ErrUnknownCommand, command)
		}
		if chord.IsZero() {
			continue
		}
		if err := chord.Validate(); err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
	}
	owner := map[Chord]Command{}
	for _, command := range Commands() {
		chord := m.Of(command)
		if chord.IsZero() {
			continue
		}
		if first, taken := owner[chord]; taken {
			return &ConflictError{First: first, Second: command, Chord: chord}
		}
		owner[chord] = command
	}
	return nil
}
