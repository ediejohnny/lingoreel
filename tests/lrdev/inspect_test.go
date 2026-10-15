package lrdev_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/lrdev"
)

var fixture = filepath.Join("..", "testdata", "matroska", "subtitles.mkv")

func TestInspectDescribesTracksAndSuggestion(t *testing.T) {
	var out bytes.Buffer
	if err := lrdev.Run(t.Context(), []string{"inspect", fixture}, &out); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Duration:",
		"#1   subtitle en     S_TEXT/UTF8",
		`"Português (Brasil)"`,
		"#3   subtitle es",
		"[forced]",
		"Top line:     #1 (en)",
		"Bottom line:  #2 (pt-BR)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, " lines (") {
		t.Fatalf("subtitles were printed without --subtitles:\n%s", text)
	}
}

func TestInspectWithSubtitlesPrintsFirstLines(t *testing.T) {
	var out bytes.Buffer
	if err := lrdev.Run(t.Context(), []string{"inspect", fixture, "--subtitles"}, &out); err != nil {
		t.Fatalf("inspect --subtitles: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Subtitle #1: 3 lines (0 left out)",
		"0:00:01 → 0:00:03.500  I'm gonna have to think about it.",
		"Subtitle #2: 3 lines (0 left out)",
		"Eu não vou para lá. / Não mesmo.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lacks %q:\n%s", want, text)
		}
	}
}

func TestInspectReportsBadInput(t *testing.T) {
	if err := lrdev.Run(t.Context(), []string{"inspect"}, &bytes.Buffer{}); !errors.Is(err, lrdev.ErrMissingArgument) {
		t.Fatalf("missing path error = %v, want ErrMissingArgument", err)
	}
	if err := lrdev.Run(t.Context(), []string{"inspect", "movie.txt"}, &bytes.Buffer{}); !errors.Is(err, analysis.ErrUnsupportedFile) {
		t.Fatalf("txt error = %v, want ErrUnsupportedFile", err)
	}
	missing := filepath.Join(t.TempDir(), "missing.mkv")
	if err := lrdev.Run(t.Context(), []string{"inspect", missing}, &bytes.Buffer{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want os.ErrNotExist", err)
	}
}
