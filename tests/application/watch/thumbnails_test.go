package watch_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// heldFilm begins every still and holds it until its context ends.
type heldFilm struct {
	began chan thumbnail.Shot
	mu    sync.Mutex
	ctxs  []context.Context
}

func (f *heldFilm) Grab(ctx context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	f.mu.Lock()
	f.ctxs = append(f.ctxs, ctx)
	f.mu.Unlock()
	f.began <- shot
	<-ctx.Done()
	return thumbnail.Picture{}, ctx.Err()
}

// A video in the app's window gets stills with the FFmpeg chosen for the
// voice, the pointer's moment first (asked through the window's remote), and
// ending the session never waits for the still being taken. A player in a
// window of its own gets none.
func TestStillsComeOnlyWithTheAppsWindowAndEndWithTheSession(t *testing.T) {
	t.Parallel()
	r, window, _ := hostedRig(t)
	r.request.Voice.FFmpegPath = `C:\tools\ffmpeg.exe`
	film := &heldFilm{began: make(chan thumbnail.Shot, 8)}
	opened := make(chan [2]string, 1)
	r.ports.OpenStills = func(_ context.Context, chosen, file string) (thumbnail.Film, error) {
		opened <- [2]string{chosen, file}
		return film, nil
	}
	s := r.start(t)
	remote, _ := window.state()
	remote.WantThumbnail(30 * time.Second) // the film lasts a minute
	select {
	case shot := <-film.began:
		if shot.At != 30*time.Second || !shot.Waited {
			t.Fatalf("the first still is %+v, want the pointer's 30 s", shot)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the pointer's still never began")
	}
	if got := <-opened; got != [2]string{`C:\tools\ffmpeg.exe`, `C:\v.mkv`} {
		t.Fatalf("stills opened with %q", got)
	}
	start := time.Now()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("ending the session took %v with a still being taken", took)
	}
	testkit.Eventually(t, "the still's context to end", func() bool {
		film.mu.Lock()
		defer film.mu.Unlock()
		return film.ctxs[0].Err() != nil
	})

	// The hosted session above opened its stills at once; this one, in 300 ms,
	// must not.
	own := newRig(t)
	ownOpened := make(chan struct{}, 1)
	own.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) {
		ownOpened <- struct{}{}
		return nil, thumbnail.ErrNoStills
	}
	session := own.start(t)
	select {
	case <-ownOpened:
		t.Fatal("stills opened for a player with its own window")
	case <-time.After(300 * time.Millisecond):
	}
	if view := session.Thumbnail(time.Second); view.Status != thumbnail.Off {
		t.Fatalf("a player with its own window shows %+v", view)
	}
}
