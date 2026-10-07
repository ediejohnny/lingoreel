package filmkey_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/filmkey"
)

// A film keeps its name in the app's folder when it is renamed or moved,
// and a different film gets another: one that differs only in its last
// bytes, or only in its length, too. A small file is named as well; a
// folder or a missing file is not.
func TestAFilmIsNamedByItsContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	key := func(path string) string {
		t.Helper()
		got, err := filmkey.Of(path)
		if err != nil || len(got) != 32 {
			t.Fatalf("%s: key %q (%v)", path, got, err)
		}
		return got
	}
	film := bytes.Repeat([]byte("frame "), 100_000) // 600 KB
	original := key(put("Film.mkv", film))
	if moved := key(put(filepath.Join("elsewhere", "Renamed.mkv"), film)); moved != original {
		t.Fatalf("the same film renamed and moved got another key: %s, %s", original, moved)
	}
	changedEnd := bytes.Clone(film)
	changedEnd[len(changedEnd)-1] = 'X'
	longer := append(bytes.Clone(film), 0)
	for name, data := range map[string][]byte{"end.mkv": changedEnd, "longer.mkv": longer} {
		if other := key(put(name, data)); other == original {
			t.Errorf("%s got the same key as another film", name)
		}
	}
	if zeros, more := key(put("zeros.mkv", make([]byte, 200_000))), key(put("more zeros.mkv", make([]byte, 200_001))); zeros == more {
		t.Error("two films alike at both ends but of different lengths got the same key")
	}
	if small := key(put("small.mkv", []byte("tiny"))); small == original {
		t.Error("a small file got the film's key")
	}
	for _, path := range []string{dir, filepath.Join(dir, "missing.mkv")} {
		if _, err := filmkey.Of(path); err == nil {
			t.Errorf("%s got a key", path)
		}
	}
}
