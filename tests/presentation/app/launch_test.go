package app_test

import (
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

func TestVideoArgument(t *testing.T) {
	t.Parallel()
	const dir = `C:\Programas\LingoReel`
	cases := []struct {
		args  []string
		want  string
		found bool
	}{
		{[]string{`D:\Filmes\Movie.mkv`}, `D:\Filmes\Movie.mkv`, true},
		// Options come before the file, and quotes left by a shell go.
		{[]string{"--debug", `"D:\Filmes\Um filme.mkv"`}, `D:\Filmes\Um filme.mkv`, true},
		{[]string{`Filmes\..\Movie.mkv`}, `C:\Programas\LingoReel\Movie.mkv`, true},
		{[]string{`D:\a.mkv`, `D:\b.mkv`}, `D:\a.mkv`, true},
		{[]string{"", "  ", "-x"}, "", false},
		{nil, "", false},
	}
	for _, c := range cases {
		got, found := app.VideoArgument(c.args, dir)
		if got != c.want || found != c.found {
			t.Errorf("VideoArgument(%q) = %q, %v; want %q, %v", c.args, got, found, c.want, c.found)
		}
	}
}

// A video opened again while one plays (a double click on another file)
// is refused with a reason, and the session goes on.
func TestAVideoWaitsForTheOneThatPlays(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	_, err := r.api.OpenVideo(`D:\Filmes\Outro.mkv`)
	if err == nil || !strings.Contains(err.Error(), "Encerre") {
		t.Fatalf("OpenVideo while watching = %v", err)
	}
	if !r.api.PlaybackState().Active {
		t.Fatal("the session ended")
	}
	r.api.StopWatching()
	testkit.Eventually(t, "the end", func() bool { return !r.api.PlaybackState().Active })
	if _, err := r.api.OpenVideo(`D:\Filmes\Outro.mkv`); err != nil {
		t.Fatalf("OpenVideo after the session = %v", err)
	}
}
