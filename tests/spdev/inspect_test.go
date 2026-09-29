package spdev_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/spdev"
)

var fixture = filepath.Join("..", "testdata", "matroska", "subtitles.mkv")

func TestInspectDescribesTracksAndSuggestion(t *testing.T) {
	var out bytes.Buffer
	if err := spdev.Run([]string{"inspect", fixture}, &out); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Duração:",
		"#1   legenda  en     S_TEXT/UTF8",
		`"Português (Brasil)"`,
		"#3   legenda  es",
		"[forçada]",
		"Legenda:   #1 (en)",
		"Tradução:  #2 (pt-BR)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "falas") {
		t.Fatalf("subtitles were printed without --legendas:\n%s", text)
	}
}

func TestInspectWithSubtitlesPrintsFirstLines(t *testing.T) {
	var out bytes.Buffer
	if err := spdev.Run([]string{"inspect", fixture, "--legendas"}, &out); err != nil {
		t.Fatalf("inspect --legendas: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Legenda #1: 3 falas (0 descartadas)",
		"0:00:01 → 0:00:03.500  I'm gonna have to think about it.",
		"Legenda #2: 3 falas (0 descartadas)",
		"Eu não vou para lá. / Não mesmo.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lacks %q:\n%s", want, text)
		}
	}
}

func TestInspectReportsBadInput(t *testing.T) {
	if err := spdev.Run([]string{"inspect"}, &bytes.Buffer{}); !errors.Is(err, spdev.ErrMissingArgument) {
		t.Fatalf("missing path error = %v, want ErrMissingArgument", err)
	}
	if err := spdev.Run([]string{"inspect", "filme.mp4"}, &bytes.Buffer{}); !errors.Is(err, analysis.ErrUnsupportedFile) {
		t.Fatalf("mp4 error = %v, want ErrUnsupportedFile", err)
	}
	missing := filepath.Join(t.TempDir(), "nao-existe.mkv")
	if err := spdev.Run([]string{"inspect", missing}, &bytes.Buffer{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want os.ErrNotExist", err)
	}
}
