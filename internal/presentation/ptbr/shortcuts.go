package ptbr

import (
	"errors"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
)

// KeyChar is the character a key types on the user's keyboard layout, such
// as "Ç" for the key right of L on ABNT2; empty when unknown.
type KeyChar func(key shortcut.Key) string

// keyNames are the keys written by name instead of their character.
var keyNames = map[shortcut.Key]string{
	"Escape": "Esc", "Backspace": "Backspace", "Tab": "Tab", "Enter": "Enter",
	"Insert": "Insert", "Delete": "Delete", "Home": "Home", "End": "End", "PageUp": "Page Up", "PageDown": "Page Down",
	"ArrowLeft": "←", "ArrowRight": "→", "ArrowUp": "↑", "ArrowDown": "↓",
}

// numpadKeys are what the numeric keypad's keys are written as, before the
// words that tell them from the main keys.
var numpadKeys = map[shortcut.Key]string{
	"NumpadAdd": "+", "NumpadSubtract": "-", "NumpadMultiply": "*", "NumpadDivide": "/",
	"NumpadDecimal": ",", "NumpadEnter": "Enter",
}

// usChars are the characters of the punctuation keys on a US keyboard, used
// when the layout's own is unknown.
var usChars = map[shortcut.Key]string{
	"Comma": ",", "Period": ".", "Minus": "-", "Equal": "=", "Semicolon": ";", "Quote": "'",
	"Slash": "/", "Backslash": `\`, "BracketLeft": "[", "BracketRight": "]", "Backquote": "`",
	"IntlBackslash": `\`, "IntlRo": "/",
}

// KeyLabel names a key as the user sees it on the keyboard.
func KeyLabel(key shortcut.Key, char KeyChar) string {
	switch key {
	case shortcut.KeySpace:
		return T("key.space")
	case shortcut.KeyAltRight:
		return T("key.altRight")
	}
	if name, named := keyNames[key]; named {
		return name
	}
	if name, named := numpadKeys[key]; named {
		return T("key.numpad", "key", name)
	}
	if letter, found := strings.CutPrefix(string(key), "Key"); found && len(letter) == 1 {
		return letter
	}
	if digit, found := strings.CutPrefix(string(key), "Digit"); found {
		return digit
	}
	if digit, found := strings.CutPrefix(string(key), "Numpad"); found && len(digit) == 1 {
		return T("key.numpad", "key", digit)
	}
	if char != nil {
		if typed := strings.TrimSpace(char(key)); typed != "" {
			return strings.ToUpper(typed)
		}
	}
	if us, found := usChars[key]; found {
		return us
	}
	return string(key) // F1..F12
}

// ShortcutLabel writes a chord as the screens show it: "Ctrl+Shift+,";
// empty for a command without a shortcut.
func ShortcutLabel(chord shortcut.Chord, char KeyChar) string {
	if chord.IsZero() {
		return ""
	}
	var b strings.Builder
	for _, m := range []struct {
		modifier shortcut.Modifiers
		name     string
	}{{shortcut.Ctrl, "Ctrl"}, {shortcut.Alt, "Alt"}, {shortcut.Shift, "Shift"}} {
		if chord.Has(m.modifier) {
			b.WriteString(m.name + "+")
		}
	}
	b.WriteString(KeyLabel(chord.Key, char))
	return b.String()
}

// CommandName names a command on the shortcuts screen.
func CommandName(command shortcut.Command) string {
	switch command {
	case shortcut.PlayPause:
		return T("shortcut.command.playPause")
	case shortcut.SeekBack:
		return T("shortcut.command.seekBack")
	case shortcut.SeekForward:
		return T("shortcut.command.seekForward")
	case shortcut.SeekBack10:
		return T("shortcut.command.seekBack10")
	case shortcut.SeekForward10:
		return T("shortcut.command.seekForward10")
	case shortcut.SeekBack30:
		return T("shortcut.command.seekBack30")
	case shortcut.SeekForward30:
		return T("shortcut.command.seekForward30")
	case shortcut.PrecisionBack:
		return T("shortcut.command.precisionBack")
	case shortcut.PrecisionForward:
		return T("shortcut.command.precisionForward")
	case shortcut.FrameBack:
		return T("shortcut.command.frameBack")
	case shortcut.FrameForward:
		return T("shortcut.command.frameForward")
	case shortcut.SpeedDown:
		return T("shortcut.command.speedDown")
	case shortcut.SpeedUp:
		return T("shortcut.command.speedUp")
	case shortcut.SpeedNormal:
		return T("shortcut.command.speedNormal")
	case shortcut.ShowBoth:
		return T("shortcut.command.showBoth")
	case shortcut.PrimaryOnly:
		return T("shortcut.command.primaryOnly")
	case shortcut.RepeatPhrase:
		return T("shortcut.command.repeatPhrase")
	case shortcut.RepeatLast3:
		return T("shortcut.command.repeatLast3")
	case shortcut.RepeatLast5:
		return T("shortcut.command.repeatLast5")
	case shortcut.RepeatLast10:
		return T("shortcut.command.repeatLast10")
	case shortcut.PassageMarkStart:
		return T("shortcut.command.passageMarkStart")
	case shortcut.PassageMarkEnd:
		return T("shortcut.command.passageMarkEnd")
	case shortcut.PassageStop:
		return T("shortcut.command.passageStop")
	case shortcut.PassageResume:
		return T("shortcut.command.passageResume")
	case shortcut.Undo:
		return T("shortcut.command.undo")
	case shortcut.Redo:
		return T("shortcut.command.redo")
	case shortcut.VolumeUp:
		return T("shortcut.command.volumeUp")
	case shortcut.VolumeDown:
		return T("shortcut.command.volumeDown")
	case shortcut.Mute:
		return T("shortcut.command.mute")
	case shortcut.Fullscreen:
		return T("shortcut.command.fullscreen")
	case shortcut.ExitFullscreen:
		return T("shortcut.command.exitFullscreen")
	case shortcut.Translation:
		return T("shortcut.command.translation")
	case shortcut.ReplayLine:
		return T("shortcut.command.replayLine")
	case shortcut.SyncPrimaryEarlier:
		return T("shortcut.command.syncPrimaryEarlier")
	case shortcut.SyncPrimaryLater:
		return T("shortcut.command.syncPrimaryLater")
	case shortcut.SyncTranslationEarlier:
		return T("shortcut.command.syncTranslationEarlier")
	case shortcut.SyncTranslationLater:
		return T("shortcut.command.syncTranslationLater")
	}
	return string(command)
}

// CategoryName names a group of commands.
func CategoryName(category shortcut.Category) string {
	switch category {
	case shortcut.CategoryPlayback:
		return T("shortcut.category.playback")
	case shortcut.CategorySeek:
		return T("shortcut.category.seek")
	case shortcut.CategoryFrame:
		return T("shortcut.category.frame")
	case shortcut.CategorySpeed:
		return T("shortcut.category.speed")
	case shortcut.CategorySubtitle:
		return T("shortcut.category.subtitle")
	case shortcut.CategoryTranslation:
		return T("shortcut.category.translation")
	case shortcut.CategoryStudy:
		return T("shortcut.category.study")
	case shortcut.CategoryEditor:
		return T("shortcut.category.editor")
	}
	return string(category)
}

// ShortcutProblem words why a chord or a set of shortcuts was refused.
func ShortcutProblem(err error, char KeyChar) string {
	var conflict *shortcut.ConflictError
	switch {
	case errors.As(err, &conflict):
		return T("shortcut.problem.conflict", "chord", ShortcutLabel(conflict.Chord, char),
			"first", CommandName(conflict.First), "second", CommandName(conflict.Second))
	case errors.Is(err, shortcut.ErrAltShift):
		return T("shortcut.problem.altShift")
	case errors.Is(err, shortcut.ErrReserved):
		return T("shortcut.problem.reserved")
	case errors.Is(err, shortcut.ErrTooManyKeys):
		return T("shortcut.problem.tooManyKeys")
	case errors.Is(err, shortcut.ErrAloneKey):
		return T("shortcut.problem.aloneKey")
	case errors.Is(err, shortcut.ErrNoKey):
		return T("shortcut.problem.noKey")
	}
	return T("shortcut.problem.other")
}
