package thumbnail_test

import (
	"context"
	"errors"
	"fmt"
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
	fail    error

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
	if f.fail != nil {
		return thumbnail.Picture{}, f.fail
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
	s := thumbnail.Start("film.mkv", opener(film), lasting(100*time.Second), thumbnail.Options{Delay: time.Millisecond})
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
	s := thumbnail.Start("film.mkv", open, lasting(20*time.Minute), thumbnail.Options{Delay: time.Hour})
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
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Minute), thumbnail.Options{Delay: time.Millisecond})
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
// time alone, and say why; a still that finds the film gives none (it seeks
// too slowly) stops them at once.
func TestTheStillsGiveUpOnAFilmThatKeepsFailing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		fail  error
		want  error
		tries int
	}{
		{errors.New("the still failed"), thumbnail.ErrGaveUp, 3},
		{fmt.Errorf("%w: the film seeks too slowly", thumbnail.ErrNoStills), thumbnail.ErrNoStills, 1},
	} {
		film := newFakeFilm(false)
		film.fail = c.fail
		stopped := make(chan error, 1)
		s := thumbnail.Start("film.mkv", opener(film), lasting(time.Hour), thumbnail.Options{
			Delay: time.Millisecond, OnStop: func(err error) { stopped <- err },
		})
		t.Cleanup(s.Close)
		select {
		case err := <-stopped:
			if !errors.Is(err, c.want) {
				t.Fatalf("told %v, want %v", err, c.want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("the stills failing with %q never stopped", c.fail)
		}
		time.Sleep(50 * time.Millisecond)
		if taken := len(film.taken()); taken != c.tries {
			t.Fatalf("failing with %q: %d stills tried, want %d", c.fail, taken, c.tries)
		}
		if view := s.Thumbnail(0); view.Status != thumbnail.Off {
			t.Fatalf("after giving up: %+v", view)
		}
	}
}

// clock is a time a test moves on by hand.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) move(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// The overview's stills and those taken while the pointer is at the bar come
// at full speed; past the overview, with nobody at the bar, the background
// is paced. The still the pointer rests on is the one it waits for.
func TestTheBackgroundIsPacedOnlyWhenNobodyIsAtTheBar(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	time0 := &clock{now: time.Unix(1000, 0)}
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Hour), thumbnail.Options{
		Delay: time.Millisecond, Overview: 2, Hover: time.Second, Now: time0.Now,
	})
	t.Cleanup(s.Close)
	step := func(what string, waited, paced bool) {
		t.Helper()
		if shot := next(t, film); shot.Waited != waited || shot.Paced != paced {
			t.Fatalf("%s: %+v, want waited %v, paced %v", what, shot, waited, paced)
		}
	}
	step("the overview's first", false, false)
	film.gate <- struct{}{}
	step("the overview's second", false, false)
	film.gate <- struct{}{}
	step("past the overview, nobody at the bar", false, true)
	s.Want(30 * time.Minute)
	film.gate <- struct{}{}
	step("the pointer's", true, false)
	film.gate <- struct{}{}
	step("the pointer still at the bar", false, false)
	time0.move(2 * time.Second)
	film.gate <- struct{}{}
	step("the pointer gone two seconds ago", false, true)
}

// Resting (on a battery) the background waits: only the still the pointer
// rests on is taken, once; the background goes on when the rest ends.
func TestRestingTakesOnlyThePointersStill(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	s := thumbnail.Start("film.mkv", opener(film), lasting(time.Hour), thumbnail.Options{Delay: time.Hour})
	t.Cleanup(s.Close)
	s.Rest(true)
	s.Want(50 * time.Second) // ends the hour
	if shot := next(t, film); shot.At != 50*time.Second || !shot.Waited {
		t.Fatalf("resting, the still taken is %+v, want the pointer's 50 s", shot)
	}
	film.gate <- struct{}{}
	s.Want(52 * time.Second) // the same still: nothing more to take
	select {
	case shot := <-film.started:
		t.Fatalf("resting, a still began that nobody waits for: %+v", shot)
	case <-time.After(200 * time.Millisecond):
	}
	s.Want(20 * time.Minute)
	if shot := next(t, film); shot.At != 20*time.Minute || !shot.Waited {
		t.Fatalf("resting, the pointer's next still is %+v", shot)
	}
	film.gate <- struct{}{}
	select {
	case shot := <-film.started:
		t.Fatalf("resting, a still began after the pointer's: %+v", shot)
	case <-time.After(200 * time.Millisecond):
	}
	s.Rest(false) // the background, waiting, goes on
	if shot := next(t, film); shot.Waited {
		t.Fatalf("after the rest the background did not go on: %+v", shot)
	}
}

// fakeStore keeps a film's stills in memory and tells each save.
type fakeStore struct {
	mu    sync.Mutex
	kept  []thumbnail.Kept
	saves [][]thumbnail.Kept
	fail  error
}

func (s *fakeStore) Load(string) ([]thumbnail.Kept, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.kept), nil
}

func (s *fakeStore) Save(_ string, stills []thumbnail.Kept) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, slices.Clone(stills))
	return s.fail
}

func (s *fakeStore) saved() [][]thumbnail.Kept {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.saves)
}

func keptAt(seconds ...int) []thumbnail.Kept {
	var out []thumbnail.Kept
	for _, second := range seconds {
		at := time.Duration(second) * time.Second
		out = append(out, thumbnail.Kept{Asked: at, Picture: thumbnail.Picture{At: at, Width: 16, Height: 9, JPEG: []byte{byte(second)}}})
	}
	return out
}

// A film's kept stills show as soon as it opens, long before the first
// still would be taken: a film kept whole never opens FFmpeg, and one kept
// in part takes only the stills it lacks. A still kept off this film's grid
// (another length) is left out.
func TestKeptStillsShowAtOnce(t *testing.T) {
	t.Parallel()
	opens := make(chan struct{}, 4)
	opening := func(film thumbnail.Film) thumbnail.Opener {
		return func(context.Context, string) (thumbnail.Film, error) { opens <- struct{}{}; return film, nil }
	}
	whole := &fakeStore{kept: keptAt(0, 10, 20, 30, 40)}
	s := thumbnail.Start("film.mkv", opening(newFakeFilm(true)), lasting(50*time.Second), thumbnail.Options{Delay: time.Hour, Store: whole})
	t.Cleanup(s.Close)
	testkit.Eventually(t, "the kept still to show", func() bool {
		view := s.Thumbnail(21 * time.Second)
		return view.Status == thumbnail.Ready && view.Sharp && view.Picture.JPEG[0] == 20
	})
	s.Want(45 * time.Second)
	select {
	case <-opens:
		t.Fatal("a film kept whole opened FFmpeg")
	case <-time.After(150 * time.Millisecond):
	}
	s.Close()
	time.Sleep(50 * time.Millisecond)
	if saves := whole.saved(); len(saves) != 0 {
		t.Fatalf("a film with nothing new was saved again %d times", len(saves))
	}

	film := newFakeFilm(false)
	part := &fakeStore{kept: append(keptAt(0, 40), keptAt(15)...)}
	p := thumbnail.Start("film.mkv", opening(film), lasting(50*time.Second), thumbnail.Options{Delay: time.Hour, Store: part})
	t.Cleanup(p.Close)
	testkit.Eventually(t, "the kept still to show", func() bool { return p.Thumbnail(40*time.Second).Status == thumbnail.Ready })
	if view := p.Thumbnail(15 * time.Second); view.Status == thumbnail.Ready && view.Sharp {
		t.Fatalf("a still off the grid shows: %+v", view)
	}
	p.Want(20 * time.Second) // ends the hour
	testkit.Eventually(t, "the lacking stills", func() bool { return len(film.taken()) == 3 })
	var taken []time.Duration
	for _, shot := range film.taken() {
		taken = append(taken, shot.At/time.Second)
	}
	slices.Sort(taken)
	if !slices.Equal(taken, []time.Duration{10, 20, 30}) {
		t.Fatalf("stills taken at %v s, want only the lacking 10, 20, 30", taken)
	}
}

// The stills are kept as they come, each save at least doubling what is
// kept, and when the service ends (without Close waiting): the kept ones
// from the store first, in the order taken; nothing new, no save. A store
// that fails is told about.
func TestStillsAreKeptAsTheyCome(t *testing.T) {
	t.Parallel()
	film := newFakeFilm(true)
	store := &fakeStore{kept: keptAt(0)}
	warned := make(chan error, 4)
	s := thumbnail.Start("film.mkv", opener(film), lasting(100*time.Second), thumbnail.Options{
		Delay: time.Millisecond, Store: store, SaveEvery: 2, Warn: func(err error) { warned <- err },
	})
	for range 3 {
		next(t, film)
		film.gate <- struct{}{}
	}
	next(t, film) // the fourth is being taken
	testkit.Eventually(t, "a save after two new stills", func() bool { return len(store.saved()) == 1 })
	s.Close()
	testkit.Eventually(t, "the save at the end", func() bool { return len(store.saved()) == 2 })
	saves := store.saved()
	var asked []time.Duration
	for _, kept := range saves[1] {
		asked = append(asked, kept.Asked/time.Second)
	}
	if len(saves[0]) != 3 || !slices.Equal(asked, []time.Duration{0, 80, 40, 20}) {
		t.Fatalf("saved %d then %v, want 3 (the kept one and two new) then 0, 80, 40, 20", len(saves[0]), asked)
	}
	time.Sleep(50 * time.Millisecond)
	if len(store.saved()) != 2 {
		t.Fatal("saved again with nothing new")
	}

	doubling := &fakeStore{}
	d := thumbnail.Start("film.mkv", opener(newFakeFilm(false)), lasting(100*time.Second), thumbnail.Options{
		Delay: time.Millisecond, Store: doubling, SaveEvery: 1,
	})
	t.Cleanup(d.Close)
	testkit.Eventually(t, "the save of the whole film", func() bool {
		saves := doubling.saved()
		return len(saves) > 0 && len(saves[len(saves)-1]) == 10
	})
	var sizes []int
	for _, save := range doubling.saved() {
		sizes = append(sizes, len(save))
	}
	if !slices.Equal(sizes, []int{1, 2, 4, 8, 10}) {
		t.Fatalf("saves of %v stills, want each at least doubling: 1, 2, 4, 8, then the 10 at the end", sizes)
	}

	failing := &fakeStore{fail: errors.New("disk full")}
	f := thumbnail.Start("film.mkv", opener(newFakeFilm(false)), lasting(20*time.Second), thumbnail.Options{
		Delay: time.Millisecond, Store: failing, Warn: func(err error) { warned <- err },
	})
	t.Cleanup(f.Close)
	select {
	case err := <-warned:
		if err.Error() != "disk full" {
			t.Fatalf("warned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a store that failed was never told about")
	}
}

// A film that gives no stills now still shows those it kept before; the
// moments they do not reach show the time alone.
func TestKeptStillsOutliveAFilmThatGivesNone(t *testing.T) {
	t.Parallel()
	stopped := make(chan error, 1)
	s := thumbnail.Start("film.mkv", func(context.Context, string) (thumbnail.Film, error) {
		return nil, thumbnail.ErrNoStills
	}, lasting(20*time.Minute), thumbnail.Options{Store: &fakeStore{kept: keptAt(60)}, OnStop: func(err error) { stopped <- err }})
	t.Cleanup(s.Close)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("never told the film gives no stills")
	}
	if view := s.Thumbnail(61 * time.Second); view.Status != thumbnail.Ready {
		t.Fatalf("the kept still: %+v", view)
	}
	if view := s.Thumbnail(10 * time.Minute); view.Status != thumbnail.Off {
		t.Fatalf("a moment no kept still reaches: %+v, want the time alone", view)
	}
}

// A disk with no room keeps no more of the film's stills this session (it
// would only fail again), says so once, and the stills go on in memory.
func TestAFullDiskStopsKeepingTheStills(t *testing.T) {
	t.Parallel()
	full := &fakeStore{fail: fmt.Errorf("%w: 10 MB free", thumbnail.ErrNoRoom)}
	warned := make(chan error, 8)
	film := newFakeFilm(false)
	s := thumbnail.Start("film.mkv", opener(film), lasting(100*time.Second), thumbnail.Options{
		Delay: time.Millisecond, Store: full, SaveEvery: 1, Warn: func(err error) { warned <- err },
	})
	t.Cleanup(s.Close)
	testkit.Eventually(t, "every still taken", func() bool { return len(film.taken()) == 10 })
	time.Sleep(50 * time.Millisecond)
	if saves := full.saved(); len(saves) != 1 || len(warned) != 1 {
		t.Fatalf("%d saves and %d warnings on a full disk, want one of each", len(saves), len(warned))
	}
	if view := s.Thumbnail(90 * time.Second); view.Status != thumbnail.Ready || !view.Sharp {
		t.Fatalf("the stills in memory: %+v", view)
	}
}
