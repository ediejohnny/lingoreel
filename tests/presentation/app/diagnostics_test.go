package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

func factOf(d app.Diagnostics, label string) (string, bool) {
	for _, fact := range d.Items {
		if fact.Label == label {
			return fact.Value, true
		}
	}
	return "", false
}

func TestDiagnosticsWithoutASession(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.monitors = []app.Monitor{
		{Width: 1440, Height: 900, DPI: 96, Primary: true},
		{Width: 2560, Height: 1440, X: 1440, Y: -200, DPI: 144},
	}
	r.attach(t)
	d := r.api.Diagnostics()
	want := map[string]string{
		"LingoReel":           "0.13.0-test",
		"Windows":             "10.0.19045",
		"Monitor 1":           "1440×900 em (0, 0), escala 100% (principal)",
		"Monitor 2":           "2560×1440 em (1440, -200), escala 150%",
		"Sessão":              "nenhum vídeo tocando",
		"Pasta dos registros": `C:\Users\x\AppData\Roaming\LingoReel\logs`,
	}
	for label, value := range want {
		if got, found := factOf(d, label); got != value || !found {
			t.Errorf("%s = %q (%v), want %q", label, got, found, value)
		}
	}
	// The text to copy is the same facts, one per line.
	if lines := strings.Split(d.Text, "\n"); len(lines) != len(d.Items) || lines[0] != "LingoReel: 0.13.0-test" {
		t.Fatalf("text = %q", d.Text)
	}

	r.monitorsErr = errors.New("no monitors")
	if got, _ := factOf(r.api.Diagnostics(), "Monitor 1"); got != "não foi possível listar" {
		t.Fatalf("monitors that cannot be listed = %q", got)
	}
}

func TestDiagnosticsOfASession(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	onTheLine(t, r)
	if err := r.api.Nudge("translation", false); err != nil {
		t.Fatal(err)
	}
	d := r.api.Diagnostics()
	state := r.api.PlaybackState()
	want := map[string]string{
		"Vídeo":                "Movie.mkv",
		"Player":               `fake (C:\fake\fake.exe)`,
		"Conexão com o player": "conectado",
		"Estado":               "pausado a 1,00×, posição 0:01,500 de 1:30:00",
		"Quadros por segundo":  "24,000",
		// One step earlier for the translation: 50 ms, the profile's step.
		state.Lines.Primary:     "offset +0,000 s, fala #1",
		state.Lines.Translation: "offset −0,050 s, fala #1",
		"Camada das legendas":   "ativa",
	}
	for label, value := range want {
		if got, found := factOf(d, label); got != value || !found {
			t.Errorf("%s = %q (%v), want %q", label, got, found, value)
		}
	}
	if strings.Contains(d.Text, "Hi.") {
		t.Fatalf("the diagnostics hold a line's text:\n%s", d.Text)
	}
	r.plugin.current().reconnecting.Store(true)
	if got, _ := factOf(r.api.Diagnostics(), "Conexão com o player"); got != "reconectando" {
		t.Fatalf("connection while reconnecting = %q", got)
	}
}
