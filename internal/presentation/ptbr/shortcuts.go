package ptbr

import (
	"errors"
	"strings"

	"lingoreel/internal/domain/shortcut"
)

// KeyChar is the character a key types on the user's keyboard layout, such
// as "Ç" for the key right of L on ABNT2; empty when unknown.
type KeyChar func(key shortcut.Key) string

// keyNames are the keys written by name instead of their character.
var keyNames = map[shortcut.Key]string{
	"Space": "Espaço", "Escape": "Esc", "Backspace": "Backspace", "Tab": "Tab", "Enter": "Enter",
	"Insert": "Insert", "Delete": "Delete", "Home": "Home", "End": "End", "PageUp": "Page Up", "PageDown": "Page Down",
	"ArrowLeft": "←", "ArrowRight": "→", "ArrowUp": "↑", "ArrowDown": "↓",
	"NumpadAdd": "+ (teclado numérico)", "NumpadSubtract": "- (teclado numérico)",
	"NumpadMultiply": "* (teclado numérico)", "NumpadDivide": "/ (teclado numérico)",
	"NumpadDecimal": ", (teclado numérico)", "NumpadEnter": "Enter (teclado numérico)",
	"AltRight": "Alt direito",
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
	if name, named := keyNames[key]; named {
		return name
	}
	if letter, found := strings.CutPrefix(string(key), "Key"); found && len(letter) == 1 {
		return letter
	}
	if digit, found := strings.CutPrefix(string(key), "Digit"); found {
		return digit
	}
	if digit, found := strings.CutPrefix(string(key), "Numpad"); found && len(digit) == 1 {
		return digit + " (teclado numérico)"
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
		return "Reproduzir / pausar"
	case shortcut.SeekBack:
		return "Voltar 5 s"
	case shortcut.SeekForward:
		return "Avançar 5 s"
	case shortcut.SeekBack10:
		return "Voltar 10 s"
	case shortcut.SeekForward10:
		return "Avançar 10 s"
	case shortcut.SeekBack30:
		return "Voltar 30 s"
	case shortcut.SeekForward30:
		return "Avançar 30 s"
	case shortcut.PrecisionBack:
		return "Voltar um pouco (precisão)"
	case shortcut.PrecisionForward:
		return "Avançar um pouco (precisão)"
	case shortcut.FrameBack:
		return "Quadro anterior"
	case shortcut.FrameForward:
		return "Próximo quadro"
	case shortcut.SpeedDown:
		return "Mais devagar"
	case shortcut.SpeedUp:
		return "Mais rápido"
	case shortcut.SpeedNormal:
		return "Velocidade normal (1,00×)"
	case shortcut.ShowBoth:
		return "Mostrar as duas legendas"
	case shortcut.PrimaryOnly:
		return "Só a legenda de cima"
	case shortcut.RepeatPhrase:
		return "Repetir a frase"
	case shortcut.RepeatLast3:
		return "Repetir os últimos 3 s"
	case shortcut.RepeatLast5:
		return "Repetir os últimos 5 s"
	case shortcut.RepeatLast10:
		return "Repetir os últimos 10 s"
	case shortcut.PassageMarkStart:
		return "Marcar o início do trecho"
	case shortcut.PassageMarkEnd:
		return "Marcar o fim e salvar o trecho"
	case shortcut.PassageStop:
		return "Sair do trecho"
	case shortcut.PassageResume:
		return "Voltar ao trecho"
	case shortcut.Undo:
		return "Desfazer a correção"
	case shortcut.Redo:
		return "Refazer a correção"
	case shortcut.VolumeUp:
		return "Aumentar o volume"
	case shortcut.VolumeDown:
		return "Diminuir o volume"
	case shortcut.Mute:
		return "Sem som / com som"
	case shortcut.Fullscreen:
		return "Tela cheia"
	case shortcut.ExitFullscreen:
		return "Sair da tela cheia"
	case shortcut.Translation:
		return "Tecla da tradução"
	case shortcut.ReplayLine:
		return "Treinar de novo (Modo Karaokê)"
	case shortcut.SyncPrimaryEarlier:
		return "Legenda de cima mais cedo"
	case shortcut.SyncPrimaryLater:
		return "Legenda de cima mais tarde"
	case shortcut.SyncTranslationEarlier:
		return "Legenda de baixo mais cedo"
	case shortcut.SyncTranslationLater:
		return "Legenda de baixo mais tarde"
	}
	return string(command)
}

// CategoryName names a group of commands.
func CategoryName(category shortcut.Category) string {
	switch category {
	case shortcut.CategoryPlayback:
		return "Reprodução"
	case shortcut.CategorySeek:
		return "Avançar e voltar"
	case shortcut.CategoryFrame:
		return "Quadro a quadro"
	case shortcut.CategorySpeed:
		return "Velocidade"
	case shortcut.CategorySubtitle:
		return "Sincronização das legendas"
	case shortcut.CategoryTranslation:
		return "Tradução"
	case shortcut.CategoryStudy:
		return "Estudo"
	case shortcut.CategoryEditor:
		return "Correção das legendas"
	}
	return string(category)
}

// ShortcutProblem words why a chord or a set of shortcuts was refused.
func ShortcutProblem(err error, char KeyChar) string {
	var conflict *shortcut.ConflictError
	switch {
	case errors.As(err, &conflict):
		return "“" + ShortcutLabel(conflict.Chord, char) + "” já é o atalho de “" + CommandName(conflict.First) +
			"”. Escolha outra combinação para “" + CommandName(conflict.Second) + "”."
	case errors.Is(err, shortcut.ErrAltShift):
		return "Alt+Shift troca o idioma do teclado no Windows. Escolha outra combinação."
	case errors.Is(err, shortcut.ErrReserved):
		return "Essa combinação é do próprio Windows. Escolha outra."
	case errors.Is(err, shortcut.ErrTooManyKeys):
		return "Use no máximo três teclas: até dois modificadores (Ctrl, Alt, Shift) e uma tecla."
	case errors.Is(err, shortcut.ErrAloneKey):
		return "O Alt direito funciona sozinho, sem Ctrl, Alt ou Shift."
	case errors.Is(err, shortcut.ErrNoKey):
		return "Falta a tecla principal: segure os modificadores e aperte uma tecla."
	}
	return "Essa tecla não pode ser usada como atalho."
}
