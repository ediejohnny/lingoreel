package thumbnail_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The stills of a film playing: taken one at a time from coarse to fine, the
// moment under the pointer first, never making anyone wait.

// fakeFilm takes stills on demand: with a gate, each still waits for a
// token (or its context), and started tells each one as it begins.
type fakeFilm struct {
	gate    chan struct{}
	started chan thumbnail.Shot
	fail    bool

	mu    sync.Mutex
	shots []thumbnail.Shot
	ctxs  []context.Context
}

func newFakeFilm(gated bool) *fakeFilm {
	f := &fakeFilm{started: make(chan thumbnail.Shot, 64)}
	if gated {
		f.gate = make(chan struct{})
	}
	return f
}

func (f *fakeFilm) Grab(ctx context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	f.mu.Lock()
	f.shots, f.ctxs = append(f.shots, shot), append(f.ctxs, ctx)
	f.mu.Unlock()
	f.started <- shot
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return thumbnail.Picture{}, ctx.Err()
		}
	}
	if f.fail {
		return thumbnail.Picture{}, errors.New("the still failed")
	}
	return thumbnail.Picture{At: shot.At, Width: 16, Height: 9, JPEG: []byte{byte(shot.At / time.Second)}}, nil
}

func (f *fakeFilm) taken() []thumbnail.Shot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.shots)
}

func opener(film thumbnail.Film) thumbnail.Opener {
	return func(context.Context, string) (thumbnail.Film, error) { return film, nil }
}

func lasting(d time.Duration) func() time.Duration { return func() time.Duration { return d } }

// next waits for the next still to begin.
func next(t *testing.T, film *fakeFilm) thumbnail.Shot {
	t.Helper()
	select {
	case shot := <-film.started:
		return shot
	case <-time.After(5 * time.Second):
		t.Fatal("no still began")
		return thumbnail.Shot{}
	}
}

// A 100 s film has a still each 10 s, taken coarse to fine (0, 80, 40, 20,
// 60, 10, 30, 50, 70, 90); the pointer, resting at 52 s while the first is
// taken, gets 50 s next, as a still it waits for.
func TestTheStillsComeCoarseFirstAndThePointerGoesFirst(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	s := thumbnail.Start("film.mkv", opener(film), lasting(100*time.Second), thumbnail.Options{Delay: time.Millisecond, Duty: 1})
	t.Cleanup(s.Close)
	if first := next(t, film); first.At != 0 || first.Waited {
		t.Fatalf("first still %+v, want 0 s from the background", first)
	}
	s.Want(52 * time.Second)
	film.gate <- struct{}{}
	if pointed := next(t, film); pointed.At != 50*time.Second || !pointed.Waited {
		t.Fatalf("after the pointer rested at 52 s the still was %+v, want 50 s, waited", pointed)
	}
	close(film.gate)
	testkit.Eventually(t, "every still taken", func() bool { return len(film.taken()) == 10 })
	var order []time.Duration
	for _, shot := range film.taken() {
		order = append(order, shot.At/time.Second)
	}
	if want := []time.Duration{0, 50, 80, 40, 20, 60, 10, 30, 70, 90}; !slices.Equal(order, want) {
		t.Fatalf("stills taken at %v s, want %v", order, want)
	}
}

// Before the film is known the bar shows the time alone; a still coming
// shows an empty box; a still taken stands for its moment (sharp) and for
// moments near it (blurred). A film with no stills stays at the time alone,
// and says why.
func TestAThumbnailIsOffThenPendingThenReady(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	opened := make(chan struct{})
	open := func(context.Context, string) (thumbnail.Film, error) { <-opened; return film, nil }
	// 20 min: a still each 10 s, standing for moments up to 12 s away.
	s := thumbnail.Start("film.mkv", open, lasting(20*time.Minute), thumbnail.Options{Delay: time.Hour, Duty: 1})
	t.Cleanup(s.Close)
	if view := s.Thumbnail(time.Minute); view.Status != thumbnail.Off {
		t.Fatalf("before the film was opened: %+v, want the time alone", view)
	}
	close(opened)
	s.Want(time.Minute) // the pointer ends the hour's delay
	if shot := next(t, film); shot.At != time.Minute || !shot.Waited {
		t.Fatalf("the still the pointer asked for is %+v", shot)
	}
	if view := s.Thumbnail(time.Minute); view.Status != thumbnail.Pending {
		t.Fatalf("while the still is taken: %+v, want an empty box", view)
	}
	film.gate <- struct{}{}
	next(t, film) // the background goes on with the next one
	for _, c := range []struct {
		at           time.Duration
		status       thumbnail.Status
		sharp        bool
		pictureAtSec byte
	}{
		{time.Minute + 2*time.Second, thumbnail.Ready, true, 60},
		{time.Minute + 11*time.Second, thumbnail.Ready, false, 60},
		{time.Minute + 30*time.Second, thumbnail.Pending, false, 0},
	} {
		view := s.Thumbnail(c.at)
		if view.Status != c.status || view.Sharp != c.sharp || (c.status == thumbnail.Ready && view.Picture.JPEG[0] != c.pictureAtSec) {
			t.Errorf("at %v: %+v, want status %v, sharp %v", c.at, view, c.status, c.sharp)
		}
	}

	stopped := make(chan error, 1)
	none := thumbnail.Start("sound.mkv", func(context.Context, string) (thumbnail.Film, error) {
		return nil, thumbnail.ErrNoStills
	}, lasting(time.Minute), thumbnail.Options{OnStop: func(err error) { stopped <- err }})
	t.Cleanup(none.Close)
	select {
	case err := <-stopped:
		if !errors.Is(err, thumbnail.ErrNoStills) {
			t.Fatalf("told %v, want ErrNoStills", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a film with no stills never said so")
	}
	if view := none.Thumbnail(time.Second); view.Status != thumbnail.Off {
		t.Fatalf("a film with no stills shows %+v", view)
	}
}

// Closing returns at once, the still being taken has its context ended, and
// no other still begins; a nil service shows the time alone.
func TestClosingNeverWaitsForAStill(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Minute), thumbnail.Options{Delay: time.Millisecond, Duty: 1})
	next(t, film)
	start := time.Now()
	s.Close()
	if took := time.Since(start); took > 50*time.Millisecond {
		t.Fatalf("Close took %v", took)
	}
	testkit.Eventually(t, "the still's context to end", func() bool {
		film.mu.Lock()
		defer film.mu.Unlock()
		return film.ctxs[0].Err() != nil
	})
	time.Sleep(50 * time.Millisecond)
	if taken := len(film.taken()); taken != 1 {
		t.Fatalf("%d stills begun after Close", taken-1)
	}
	var none *thumbnail.Service
	none.Want(time.Second)
	none.Close()
	if view := none.Thumbnail(time.Second); view.Status != thumbnail.Off {
		t.Fatalf("a nil service shows %+v", view)
	}
}

// Three stills failing in a row stop the film's stills, which then show the
// time alone, and say why.
func TestTheStillsGiveUpOnAFilmThatKeepsFailing(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(false)
	film.fail = true
	stopped := make(chan error, 1)
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Hour), thumbnail.Options{
		Delay: time.Millisecond, Duty: 1, OnStop: func(err error) { stopped <- err },
	})
	t.Cleanup(s.Close)
	select {
	case err := <-stopped:
		if !errors.Is(err, thumbnail.ErrGaveUp) {
			t.Fatalf("told %v, want ErrGaveUp", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stills never gave up")
	}
	time.Sleep(50 * time.Millisecond)
	if taken := len(film.taken()); taken != 3 {
		t.Fatalf("%d stills tried, want 3", taken)
	}
	if view := s.Thumbnail(0); view.Status != thumbnail.Off {
		t.Fatalf("after giving up: %+v", view)
	}
}

// Past the overview's stills, the background rests three times its work at
// a quarter of the time; the stills the pointer waits for do not rest.
func TestTheBackgroundRestsInProportionToItsWork(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	var mu sync.Mutex
	now := time.Unix(0, 0)
	var rests []time.Duration
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Hour), thumbnail.Options{
		Delay: time.Hour, Duty: 0.25, Overview: 1,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			now = now.Add(100 * time.Millisecond) // each still "takes" 100 ms
			return now
		},
		After: func(d time.Duration) <-chan time.Time {
			if d >= time.Second {
				return time.After(d) // the hour before the first still
			}
			mu.Lock()
			rests = append(rests, d)
			mu.Unlock()
			ready := make(chan time.Time, 1)
			ready <- time.Time{}
			return ready
		},
	})
	t.Cleanup(s.Close)
	s.Want(0) // ends the hour at once
	if first := next(t, film); !first.Waited {
		t.Fatalf("the first still %+v, want the pointer's", first)
	}
	film.gate <- struct{}{} // the pointer's: no rest
	next(t, film)
	s.Want(30 * time.Minute)
	film.gate <- struct{}{} // past the overview's one still: rests 300 ms
	if pointed := next(t, film); !pointed.Waited {
		t.Fatalf("the third still %+v, want the pointer's", pointed)
	}
	film.gate <- struct{}{} // the pointer's: no rest
	next(t, film)
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(rests, []time.Duration{300 * time.Millisecond}) {
		t.Fatalf("rests %v, want one of 300 ms", rests)
	}
}
