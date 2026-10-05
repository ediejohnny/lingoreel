//go:build windows

package discovery_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
)

func writeFakeExecutable(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vlc.exe")
	if err := os.WriteFile(path, []byte("MZ"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLookPathFindsAnAbsolutePathEntry(t *testing.T) {
	want := writeFakeExecutable(t, filepath.Join(t.TempDir(), "bin"))
	t.Setenv("PATH", filepath.Dir(want))

	got, found := discovery.SystemEnvironment{}.LookPath("vlc.exe")
	if !found || !strings.EqualFold(got, want) {
		t.Fatalf("LookPath = %q, %v; want %q", got, found, want)
	}
}

// A player found through the current directory or a relative PATH entry could
// be a file planted next to a video. Go refuses those results by default
// (exec.ErrDot); the check must hold even when that protection is switched off.
func TestLookPathRefusesMatchesRelativeToTheCurrentDirectory(t *testing.T) {
	for _, godebug := range []string{"", "execerrdot=0"} {
		t.Run("GODEBUG="+godebug, func(t *testing.T) {
			dir := t.TempDir()
			writeFakeExecutable(t, dir)
			writeFakeExecutable(t, filepath.Join(dir, "relativo"))
			t.Chdir(dir)
			t.Setenv("GODEBUG", godebug)

			for _, pathList := range []string{"relativo", "", "." + string(os.PathListSeparator) + "relativo"} {
				t.Setenv("PATH", pathList)
				if got, found := (discovery.SystemEnvironment{}).LookPath("vlc.exe"); found {
					t.Errorf("PATH=%q: LookPath accepted %q", pathList, got)
				}
			}
		})
	}
}
