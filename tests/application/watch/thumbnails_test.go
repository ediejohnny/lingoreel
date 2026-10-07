package watch_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
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

// quickFilm takes every still at once, telling each as it begins.
type quickFilm struct{ began chan thumbnail.Shot }

func (f *quickFilm) Grab(_ context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	f.began <- shot
	return thumbnail.Picture{At: shot.At, Width: 16, Height: 9, JPEG: []byte{1}}, nil
}

// On the battery a video's stills wait for the pointer: only the moment it
// rests on gets one. Plugged in, the background goes on.
func TestOnTheBatteryOnlyThePointersStillIsTaken(t *testing.T) {
	t.Parallel()
	r, window, _ := hostedRig(t)
	battery := newPower(true)
	r.ports.Power = battery
	film := &quickFilm{began: make(chan thumbnail.Shot, 64)}
	r.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) { return film, nil }
	r.start(t)
	remote, _ := window.state()
	remote.WantThumbnail(30 * time.Second)
	select {
	case shot := <-film.began:
		if shot.At != 30*time.Second || !shot.Waited {
			t.Fatalf("on the battery the first still is %+v, want the pointer's 30 s", shot)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("on the battery the pointer's still never began")
	}
	select {
	case shot := <-film.began:
		t.Fatalf("on the battery a still nobody waits for began: %+v", shot)
	case <-time.After(300 * time.Millisecond):
	}
	battery.changes <- false
	select {
	case shot := <-film.began:
		if shot.Waited {
			t.Fatalf("plugged in, the next still is %+v, want the background's", shot)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("plugged in, the background never went on")
	}
}

// With "also on the battery" turned on while the video plays on the battery,
// the background goes on at once.
func TestTheStillsCanGoOnOnTheBattery(t *testing.T) {
	t.Parallel()
	r, window, _ := hostedRig(t)
	r.ports.Power = newPower(true)
	film := &quickFilm{began: make(chan thumbnail.Shot, 64)}
	r.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) { return film, nil }
	s := r.start(t)
	remote, _ := window.state()
	remote.WantThumbnail(30 * time.Second)
	if shot := <-film.began; !shot.Waited {
		t.Fatalf("the first still is %+v, want the pointer's", shot)
	}
	select {
	case shot := <-film.began:
		t.Fatalf("on the battery a still nobody waits for began: %+v", shot)
	case <-time.After(300 * time.Millisecond):
	}
	s.SetThumbnailSetup(watch.ThumbnailSetup{OnBattery: true})
	select {
	case shot := <-film.began:
		if shot.Waited {
			t.Fatalf("also on the battery, the next still is %+v, want the background's", shot)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("also on the battery, the background never went on")
	}
}

// gateFilm begins every still and finishes it when the test lets it.
type gateFilm struct {
	began chan thumbnail.Shot
	gate  chan struct{}
}

func (f *gateFilm) Grab(ctx context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	f.began <- shot
	select {
	case <-f.gate:
		return thumbnail.Picture{At: shot.At, Width: 16, Height: 9, JPEG: []byte{1}}, nil
	case <-ctx.Done():
		return thumbnail.Picture{}, ctx.Err()
	}
}

// pointerThenNext asks for the pointer's still, lets it finish once ready
// says so, and tells whether a still nobody waits for begins next.
func pointerThenNext(t *testing.T, remote watch.Remote, film *gateFilm, ready func()) bool {
	t.Helper()
	remote.WantThumbnail(30 * time.Second)
	if shot := <-film.began; !shot.Waited {
		t.Fatalf("the first still is %+v, want the pointer's", shot)
	}
	ready()
	film.gate <- struct{}{}
	select {
	case shot := <-film.began:
		return !shot.Waited
	case <-time.After(time.Second):
		return false
	}
}

// With "also on the battery" the background goes on on the battery, from
// the start and when the computer leaves its plug while the video plays;
// without it, stills started again on the battery (the setup changed) rest.
func TestAlsoOnTheBatteryTheBackgroundGoesOn(t *testing.T) {
	t.Parallel()
	start := func(t *testing.T, onBattery bool, setup watch.ThumbnailSetup) (*watch.Session, watch.Remote, *gateFilm, *power) {
		r, window, _ := hostedRig(t)
		battery := newPower(onBattery)
		r.ports.Power = battery
		r.request.Thumbnails = setup
		film := &gateFilm{began: make(chan thumbnail.Shot, 8), gate: make(chan struct{})}
		r.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) { return film, nil }
		s := r.start(t)
		remote, _ := window.state()
		return s, remote, film, battery
	}
	t.Run("from the start", func(t *testing.T) {
		t.Parallel()
		_, remote, film, _ := start(t, true, watch.ThumbnailSetup{OnBattery: true})
		if !pointerThenNext(t, remote, film, func() {}) {
			t.Fatal("on the battery, also on the battery, the background did not go on")
		}
	})
	t.Run("leaving the plug", func(t *testing.T) {
		t.Parallel()
		_, remote, film, battery := start(t, false, watch.ThumbnailSetup{OnBattery: true})
		unplug := func() {
			battery.changes <- true
			time.Sleep(200 * time.Millisecond)
		}
		if !pointerThenNext(t, remote, film, unplug) {
			t.Fatal("unplugged, also on the battery, the background did not go on")
		}
	})
	t.Run("started again on the battery", func(t *testing.T) {
		t.Parallel()
		s, remote, film, battery := start(t, false, watch.ThumbnailSetup{})
		battery.changes <- true
		time.Sleep(200 * time.Millisecond)
		s.SetThumbnailSetup(watch.ThumbnailSetup{InMemory: true})
		if pointerThenNext(t, remote, film, func() {}) {
			t.Fatal("started again on the battery, a still nobody waits for began")
		}
	})
}

// The stills follow their settings while the video plays: kept in memory
// only they never read the disk, back on the disk the kept ones show, turned
// off the bar shows the time alone and the still being taken ends, and once
// the session ends nothing starts them again.
func TestTheStillsFollowTheirSettings(t *testing.T) {
	t.Parallel()
	r, window, _ := hostedRig(t)
	r.request.Thumbnails = watch.ThumbnailSetup{InMemory: true}
	still := thumbnail.Picture{At: 30 * time.Second, Width: 16, Height: 9, JPEG: []byte{30}}
	r.ports.KeepStills = keptStore{kept: []thumbnail.Kept{{Asked: 30 * time.Second, Picture: still}}}
	film := &heldFilm{began: make(chan thumbnail.Shot, 64)}
	var opens atomic.Int32
	r.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) {
		opens.Add(1)
		return film, nil
	}
	s := r.start(t)
	remote, _ := window.state()
	remote.WantThumbnail(31 * time.Second)
	<-film.began
	if view := remote.Thumbnail(31 * time.Second); view.Status == thumbnail.Ready {
		t.Fatalf("kept in memory only, the disk's still showed: %+v", view)
	}

	s.SetThumbnailSetup(watch.ThumbnailSetup{})
	testkit.Eventually(t, "the kept still once the stills keep to the disk", func() bool {
		view := remote.Thumbnail(31 * time.Second)
		return view.Status == thumbnail.Ready && view.Picture.JPEG[0] == 30
	})

	s.SetThumbnailSetup(watch.ThumbnailSetup{Off: true})
	if view := remote.Thumbnail(31 * time.Second); view.Status != thumbnail.Off {
		t.Fatalf("turned off, the bar shows %+v", view)
	}
	testkit.Eventually(t, "every still being taken to end", func() bool {
		film.mu.Lock()
		defer film.mu.Unlock()
		for _, ctx := range film.ctxs {
			if ctx.Err() == nil {
				return false
			}
		}
		return true
	})

	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	before := opens.Load()
	s.SetThumbnailSetup(watch.ThumbnailSetup{})
	time.Sleep(300 * time.Millisecond)
	if opens.Load() != before || s.Thumbnail(31*time.Second).Status != thumbnail.Off {
		t.Fatal("the stills started again after the session ended")
	}
}

// keptStore holds one film's stills, as the app's folder would.
type keptStore struct{ kept []thumbnail.Kept }

func (s keptStore) Load(string) ([]thumbnail.Kept, error) { return s.kept, nil }
func (keptStore) Save(string, []thumbnail.Kept) error     { return nil }

// A video's stills kept from an earlier opening show on the seek bar at
// once, through the window's remote.
func TestKeptStillsShowWhenTheVideoOpensAgain(t *testing.T) {
	t.Parallel()
	r, window, _ := hostedRig(t)
	still := thumbnail.Picture{At: 30 * time.Second, Width: 16, Height: 9, JPEG: []byte{30}}
	r.ports.KeepStills = keptStore{kept: []thumbnail.Kept{{Asked: 30 * time.Second, Picture: still}}}
	r.ports.OpenStills = func(context.Context, string, string) (thumbnail.Film, error) {
		return &heldFilm{began: make(chan thumbnail.Shot, 8)}, nil
	}
	r.start(t)
	remote, _ := window.state()
	testkit.Eventually(t, "the kept still on the bar", func() bool {
		view := remote.Thumbnail(31 * time.Second)
		return view.Status == thumbnail.Ready && view.Sharp && view.Picture.JPEG[0] == 30
	})
}
