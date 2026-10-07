package spdev_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/spdev"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The stills' options are checked before any FFmpeg runs, on a context
// already ended.
func TestStillsValidatesOptionsFirst(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		args []string
		want error
	}{
		{[]string{"stills"}, spdev.ErrMissingArgument},
		{[]string{"stills", "film.mkv", "--affinity", "zz"}, spdev.ErrInvalidOption},
		{[]string{"stills", "film.mkv", "--affinity", "0"}, spdev.ErrInvalidOption},
		{[]string{"stills", "film.mkv", "--parallel", "9"}, spdev.ErrInvalidOption},
		{[]string{"stills", "film.mkv", "--count", "0"}, spdev.ErrInvalidOption},
	} {
		if err := spdev.Run(ctx, c.args, &bytes.Buffer{}); !errors.Is(err, c.want) {
			t.Errorf("%q: %v, want %v", c.args, err, c.want)
		}
	}
}

// Measuring a film's stills prints what each cost and a summary, with the
// film's key and never its name.
func TestStillsMeasuresAFilmWithoutNamingIt(t *testing.T) {
	testkit.Slow(t)
	tool := os.Getenv("LINGOREEL_TEST_FFMPEG")
	if tool == "" {
		t.Skip("no FFmpeg for the tests (LINGOREEL_TEST_FFMPEG)")
	}
	film := filepath.Join(t.TempDir(), "Secret Film Name.mkv")
	if out, err := exec.Command(tool, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=30",
		"-c:v", "mpeg4", film).CombinedOutput(); err != nil {
		t.Fatalf("making the film: %v: %s", err, out)
	}
	var out bytes.Buffer
	if err := spdev.Run(context.Background(), []string{"stills", film, "--ffmpeg", tool, "--count", "3", "--paced"}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	text := out.String()
	switch {
	case strings.Contains(text, "Secret"):
		t.Fatalf("the film's name was printed:\n%s", text)
	case strings.Count(text, " reads") != 3 || strings.Count(text, "capped") != 3:
		t.Fatalf("not three paced stills, each held to its share:\n%s", text)
	case !strings.Contains(text, "Took:") || !strings.Contains(text, "0 failed") || !strings.Contains(text, "Index:    true"):
		t.Fatalf("no summary of a film with its index:\n%s", text)
	}
}
