package install_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"lingoreel/internal/infrastructure/player/install"
)

type recorder struct {
	program string
	args    []string
	err     error
}

func (r *recorder) run(_ context.Context, program string, args ...string) error {
	r.program, r.args = program, args
	return r.err
}

func found(string) (string, error) { return `C:\winget.exe`, nil }

func missing(string) (string, error) { return "", errors.New("not found") }

func TestInstallRunsWingetExactlyAndSilently(t *testing.T) {
	r := &recorder{}
	winget := install.Winget{LookPath: found, Run: r.run}
	if err := winget.Install(t.Context(), "VideoLAN.VLC"); err != nil {
		t.Fatal(err)
	}
	want := []string{"install", "--id", "VideoLAN.VLC", "--exact", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	if r.program != `C:\winget.exe` || !slices.Equal(r.args, want) {
		t.Fatalf("ran %s %q, want %q", r.program, r.args, want)
	}
}

func TestInstallRefusesWhatIsNotAPackageID(t *testing.T) {
	for _, id := range []string{"", "--source", "VideoLAN VLC", "a&b", "x;rm", `..\x`} {
		r := &recorder{}
		err := install.Winget{LookPath: found, Run: r.run}.Install(t.Context(), id)
		if !errors.Is(err, install.ErrInvalidPackage) || r.program != "" {
			t.Errorf("Install(%q) = %v and ran %q; want ErrInvalidPackage and nothing run", id, err, r.program)
		}
	}
}

func TestInstallWithoutWinget(t *testing.T) {
	r := &recorder{}
	winget := install.Winget{LookPath: missing, Run: r.run}
	if winget.Available() {
		t.Fatal("Available without winget")
	}
	if err := winget.Install(t.Context(), "VideoLAN.VLC"); !errors.Is(err, install.ErrNoWinget) || r.program != "" {
		t.Fatalf("Install without winget = %v, ran %q", err, r.program)
	}
	if !(install.Winget{LookPath: found}).Available() {
		t.Fatal("not Available with winget")
	}
}

func TestInstallReportsAFailedInstallation(t *testing.T) {
	boom := errors.New("exit status 1")
	err := install.Winget{LookPath: found, Run: (&recorder{err: boom}).run}.Install(t.Context(), "VideoLAN.VLC")
	if !errors.Is(err, boom) {
		t.Fatalf("Install = %v, want the runner's error wrapped", err)
	}
}
