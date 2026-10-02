package ptbr_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/presentation/ptbr"
)

func TestShortcutLabels(t *testing.T) {
	cases := map[string]string{
		"Space":            "Espaço",
		"Shift+Comma":      "Shift+,",
		"Ctrl+Period":      "Ctrl+.",
		"Ctrl+Shift+KeyP":  "Ctrl+Shift+P",
		"Alt+Digit3":       "Alt+3",
		"AltRight":         "Alt direito",
		"ArrowLeft":        "←",
		"Escape":           "Esc",
		"F5":               "F5",
		"Numpad7":          "7 (teclado numérico)",
		"Ctrl+NumpadEnter": "Ctrl+Enter (teclado numérico)",
		"Backquote":        "`",
	}
	for text, want := range cases {
		chord, err := shortcut.Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := ptbr.ShortcutLabel(chord, nil); got != want {
			t.Errorf("ShortcutLabel(%s) = %q, want %q", text, got, want)
		}
	}
	// A layout's character replaces the US one, upper case; blank is ignored.
	char := func(key shortcut.Key) string {
		return map[shortcut.Key]string{"Quote": "~", "Slash": " "}[key]
	}
	if got := ptbr.KeyLabel("Quote", char); got != "~" {
		t.Errorf("Quote = %q", got)
	}
	if got := ptbr.KeyLabel("Slash", char); got != "/" {
		t.Errorf("Slash with a blank character = %q", got)
	}
}

func TestEveryCommandAndCategoryHasAName(t *testing.T) {
	seen := map[string]bool{}
	for _, command := range shortcut.Commands() {
		name := ptbr.CommandName(command)
		if name == "" || name == string(command) || seen[name] {
			t.Errorf("%s is named %q", command, name)
		}
		seen[name] = true
		if category := ptbr.CategoryName(command.Category()); category == string(command.Category()) {
			t.Errorf("category of %s has no name", command)
		}
	}
}

func TestShortcutProblems(t *testing.T) {
	conflict := shortcut.Map{shortcut.Mute: {Key: shortcut.KeySpace}}.Validate()
	cases := []struct {
		err  error
		want string
	}{
		{conflict, "“Espaço” já é o atalho de “Reproduzir / pausar”. Escolha outra combinação para “Sem som / com som”."},
		{fmt.Errorf("x: %w", shortcut.ErrAltShift), "Alt+Shift troca o idioma"},
		{shortcut.ErrReserved, "do próprio Windows"},
		{shortcut.ErrTooManyKeys, "no máximo três teclas"},
		{shortcut.ErrAloneKey, "Alt direito funciona sozinho"},
		{shortcut.ErrNoKey, "Falta a tecla principal"},
		{shortcut.ErrUnknownKey, "não pode ser usada"},
		{errors.New("other"), "não pode ser usada"},
	}
	for _, c := range cases {
		if got := ptbr.ShortcutProblem(c.err, nil); !strings.Contains(got, c.want) {
			t.Errorf("ShortcutProblem(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
