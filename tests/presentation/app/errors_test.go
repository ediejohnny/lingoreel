package app_test

import (
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The errors a user can meet, each with a sentence that says what happened and
// what to do.

func TestOpeningAVideoThatCannotBeRead(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	cases := []struct {
		path, want string
	}{
		{missingVideo, "Sumiu.mkv não foi encontrado"},
		{brokenVideo, "Quebrado.mkv não parece um vídeo"},
		{`D:\Filmes\notas.txt`, "não abre arquivos .txt"},
	}
	for _, c := range cases {
		_, err := r.api.OpenVideo(c.path)
		wantMessage(t, "OpenVideo "+c.path, err, c.want)
	}
}

func TestASubtitleLanguageThatIsNotThere(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	video, err := r.api.OpenVideo(portugueseVideo)
	if err != nil {
		t.Fatal(err)
	}
	// The profile studies English with Portuguese below: only the English is missing.
	if !strings.Contains(video.LanguageNote, "Não há legenda em inglês neste vídeo") || strings.Contains(video.LanguageNote, "português") {
		t.Fatalf("language note = %q", video.LanguageNote)
	}
	if video, err = r.api.OpenVideo(movie); err != nil || video.LanguageNote != "" {
		t.Fatalf("a video with both languages: %q, %v", video.LanguageNote, err)
	}
}

func TestASessionThatDoesNotStartSaysWhy(t *testing.T) {
	cases := []struct {
		name  string
		video string
		set   func(*rig)
		want  string
	}{
		{"the player times out", movie, func(r *rig) { r.plugin.launchErr = errors.New("VLC did not answer in time") }, "Fake Player não abriu a tempo"},
		{"the player's program is gone", movie, func(r *rig) {
			r.plugin.launchErr = fmt.Errorf("starting: %w", exec.ErrNotFound)
		}, `Fake Player não foi encontrado em C:\fake\fake.exe`},
		{"the display does not open", movie, func(r *rig) { r.displayErr = errors.New("no window") }, "janela do vídeo com as legendas"},
		{"the subtitles do not read", badSubtitlesVideo, func(*rig) {}, "ler as legendas escolhidas"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			r.attach(t)
			if _, err := r.api.OpenVideo(c.video); err != nil {
				t.Fatal(err)
			}
			c.set(r)
			err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5})
			wantMessage(t, "Watch", err, c.want)
			if r.api.PlaybackState().Active {
				t.Fatal("a session runs after a failed start")
			}
		})
	}
}

// A player that crashes ends the session with a reason and an offer to
// watch again from where it was.
func TestAPlayerLostCanBeWatchedAgainFromThere(t *testing.T) {
	r := startWatching(t)
	if err := r.api.SeekTo(50); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "the position", func() bool { return math.Abs(r.api.PlaybackState().Position-50) < 0.5 })
	running := r.plugin.current()
	running.lost.Store(true)
	running.Close()
	testkit.Eventually(t, "the end", func() bool { _, ended := r.ui.last(app.EventEnded); return ended })
	data, _ := r.ui.last(app.EventEnded)
	ended := data.(app.Ended)
	if !ended.Resume || !strings.Contains(ended.Error, "parou de responder") {
		t.Fatalf("ended = %+v", ended)
	}

	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, Resume: true}); err != nil {
		t.Fatal(err)
	}
	if start := r.plugin.launched[len(r.plugin.launched)-1].Start; start < 49*time.Second || start > 51*time.Second {
		t.Fatalf("watched again from %v, want about 50 s", start)
	}
	// The place is used once: the next start begins as usual, unless asked
	// to go on where the video was left.
	r.api.StopWatching()
	testkit.Eventually(t, "the stop", func() bool { return !r.api.PlaybackState().Active })
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5}); err != nil {
		t.Fatal(err)
	}
	if start := r.plugin.launched[len(r.plugin.launched)-1].Start; start != 0 {
		t.Fatalf("a start without resume began at %v", start)
	}
}
