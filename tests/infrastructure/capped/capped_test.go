package capped_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/capped"
)

// A file up to the limit is read whole; one past it, or a folder, is refused
// with its own error.
func TestAFileIsReadOnlyUpToItsLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.srt")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := capped.ReadFile(path, 5); err != nil || string(data) != "12345" {
		t.Errorf("at the limit: %q, %v; want the whole file", data, err)
	}
	if _, err := capped.ReadFile(path, 4); !errors.Is(err, capped.ErrTooLarge) {
		t.Errorf("past the limit: %v, want ErrTooLarge", err)
	}
	if _, err := capped.ReadFile(dir, 100); !errors.Is(err, capped.ErrNotFile) {
		t.Errorf("a folder: %v, want ErrNotFile", err)
	}
	if _, err := capped.ReadFile(filepath.Join(dir, "missing.srt"), 100); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing file: %v, want os.ErrNotExist", err)
	}
}
