package thumbnail

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/storyboard"
)

// Status tells what the bar shows for a moment.
type Status int

const (
	// Off: no still for this film (yet unknown, none to take, or given up):
	// the time alone.
	Off Status = iota
	// Pending: the still is coming; an empty box over the time.
	Pending
	// Ready: a still stands for the moment.
	Ready
)

// View is what the bar shows for a moment under the pointer.
type View struct {
	Status  Status
	Picture Picture
	// Sharp tells the moment's own still; otherwise it is the nearest one
	// taken so far, shown blurred.
	Sharp bool
}

// Options tune a Service; zero fields take the defaults.
type Options struct {
	// Delay is how long the film plays before it is looked at and its first
	// still taken, so the player's opening is not disturbed; the pointer on
	// the bar ends it.
	Delay time.Duration
	// Overview is how many stills come at full speed before the background
	// is paced: the first ones already cover the film.
	Overview int
	// Hover is how long the pointer counts as at the bar after it last asked
	// for a moment: meanwhile no still is paced.
	Hover time.Duration
	// GiveUp is how many stills in a row may fail before the film's stills
	// stop; a still the computer was too busy for (ErrBusy) is no failure.
	GiveUp int
	// BusyRest is how long the stills wait after the computer was too busy
	// for one, or for looking at the film, before asking again; the pointer
	// on the bar ends the wait.
	BusyRest time.Duration
	// Store keeps the film's stills between openings; nil keeps them in
	// memory only. They are saved once SaveEvery new stills, and as many as
	// those already kept, are there, and when the stills end; a problem
	// keeping them goes to Warn.
	Store     Store
	SaveEvery int
	Warn      func(err error)
	// OnStop hears, once, why the stills stopped before the film was covered.
	OnStop func(err error)
	Now    func() time.Time
	After  func(time.Duration) <-chan time.Time
}

// ErrGaveUp reports a film whose stills kept failing.
var ErrGaveUp = errors.New("the stills kept failing")

const (
	defaultDelay     = 10 * time.Second
	defaultOverview  = 100
	defaultHover     = 2 * time.Second
	defaultGiveUp    = 3
	defaultBusyRest  = 30 * time.Second
	defaultSaveEvery = 25
	// durationPoll is how often a film with no duration yet is asked again.
	durationPoll = 250 * time.Millisecond
)

// Service takes a film's stills while it plays, one at a time, and hands the
// bar the still that stands for the moment under the pointer. Its methods
// never wait for a still; a nil Service shows no stills.
type Service struct {
	file     string
	open     Opener
	duration func() time.Duration
	options  Options
	ctx      context.Context
	cancel   context.CancelFunc
	wake     chan struct{}

	mu sync.Mutex
	// plan is nil until the film is opened and its duration known.
	plan     *storyboard.Plan
	pictures map[time.Duration]Picture
	// kept are the stills in the order taken, as the store keeps them;
	// loaded, those the store gave until the plan places them; unsaved, how
	// many were taken since the last save.
	kept    []Kept
	loaded  []Kept
	unsaved int
	// unkept: the disk had no room, the stills stay in memory; discarded:
	// from then on they are taken anew at each opening, never kept.
	unkept, discarded bool
	// wanted is the moment the pointer is on, asked last at lastWant;
	// pointed, a want not yet served.
	wanted   time.Duration
	lastWant time.Time
	pointed  bool
	// resting takes only the stills the pointer waits for.
	resting  bool
	finished bool
	off      bool
}

// Start begins taking a film's stills. duration tells the film's length,
// zero while unknown.
func Start(file string, open Opener, duration func() time.Duration, options Options) *Service {
	if options.Delay <= 0 {
		options.Delay = defaultDelay
	}
	if options.Hover <= 0 {
		options.Hover = defaultHover
	}
	if options.BusyRest <= 0 {
		options.BusyRest = defaultBusyRest
	}
	options.Overview = cmpOr(options.Overview, defaultOverview)
	options.GiveUp = cmpOr(options.GiveUp, defaultGiveUp)
	options.SaveEvery = cmpOr(options.SaveEvery, defaultSaveEvery)
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.After == nil {
		options.After = time.After
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{file: file, open: open, duration: duration, options: options, ctx: ctx, cancel: cancel,
		wake: make(chan struct{}, 1), pictures: map[time.Duration]Picture{}}
	go s.run()
	return s
}

func cmpOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// Thumbnail is what the bar shows for a moment.
func (s *Service) Thumbnail(at time.Duration) View {
	if s == nil {
		return View{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.off || s.plan == nil {
		return View{}
	}
	if still, own, ok := s.plan.Answer(at); ok {
		return View{Status: Ready, Picture: s.pictures[still], Sharp: own}
	}
	if s.finished {
		return View{}
	}
	return View{Status: Pending}
}

// Want tells the moment the pointer is on: its still is taken before any
// other, and the film's first still is not delayed any more.
func (s *Service) Want(at time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.wanted, s.pointed, s.lastWant = at, true, s.options.Now()
	s.mu.Unlock()
	s.poke()
}

// Rest takes, while resting, only the stills the pointer waits for: the
// background waits (on a battery, by default).
func (s *Service) Rest(resting bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	woken := s.resting && !resting
	s.resting = resting
	s.mu.Unlock()
	if woken {
		s.poke()
	}
}

// Close stops taking stills, without waiting for the one being taken; those
// taken since the last save are kept.
func (s *Service) Close() {
	if s != nil {
		s.cancel()
	}
}

// Discard stops taking stills, as Close does, and keeps none of those not
// kept yet: the stills are to be taken anew at each opening, the disk never
// written.
func (s *Service) Discard() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.discarded = true
	s.mu.Unlock()
	s.cancel()
}

func (s *Service) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// pause waits d, or less when the pointer asks for a still; false once the
// service is closed.
func (s *Service) pause(d time.Duration) bool {
	select {
	case <-s.options.After(d):
	case <-s.wake:
	case <-s.ctx.Done():
		return false
	}
	return true
}

// idle waits for the pointer or the end of a rest; false once the service
// is closed.
func (s *Service) idle() bool {
	select {
	case <-s.wake:
	case <-s.ctx.Done():
		return false
	}
	return true
}

func (s *Service) run() {
	// What was taken is kept, also when the service is closed: on this
	// goroutine, so Close never waits for it.
	defer s.save()
	// Kept stills show as soon as the film's length is known; a film kept
	// whole needs no FFmpeg at all.
	if s.load() && (!s.ensurePlan() || s.complete()) {
		return
	}
	// Nothing runs while the player opens the film, the look at it included.
	if !s.pause(s.options.Delay) {
		return
	}
	film, err := s.openWhenFree()
	if err != nil {
		s.stop(err)
		return
	}
	if !s.ensurePlan() {
		return
	}
	plan := s.plan
	failures, taken := 0, 0
	for {
		s.mu.Lock()
		slot, waited, ok := plan.Next(s.pointed, s.wanted)
		s.pointed = false
		s.finished = !ok
		resting := s.resting && !waited
		hovering := s.options.Now().Sub(s.lastWant) < s.options.Hover
		s.mu.Unlock()
		if !ok {
			return
		}
		if resting {
			if !s.idle() {
				return
			}
			continue
		}
		asked := plan.Board().At(slot)
		shot := Shot{At: asked, Waited: waited, Paced: !waited && !hovering && taken >= s.options.Overview}
		picture, err := film.Grab(s.ctx, shot)
		if s.ctx.Err() != nil {
			return
		}
		busy := errors.Is(err, ErrBusy)
		s.mu.Lock()
		switch {
		case busy:
			// The slot stays open, to be taken after the rest.
		case err != nil:
			plan.Skip(slot)
			failures++
		default:
			plan.Done(slot, picture.At)
			s.pictures[picture.At] = picture
			s.kept = append(s.kept, Kept{Asked: asked, Picture: picture})
			s.unsaved++
			failures, taken = 0, taken+1
		}
		// Each save at least doubles what is kept: a long film is written
		// some 2 times its size in all, not once per batch.
		due := s.unsaved >= s.options.SaveEvery && s.unsaved >= len(s.kept)-s.unsaved
		s.mu.Unlock()
		switch {
		case busy:
			if !s.pause(s.options.BusyRest) {
				return
			}
		case errors.Is(err, ErrNoStills):
			s.stop(err)
			return
		case failures >= s.options.GiveUp:
			s.stop(errors.Join(ErrGaveUp, err))
			return
		case due:
			s.save()
		}
	}
}

// openWhenFree looks at the film, again after each rest while the computer
// is too busy to; an error once the service is closed.
func (s *Service) openWhenFree() (Film, error) {
	for {
		film, err := s.open(s.ctx, s.file)
		if !errors.Is(err, ErrBusy) {
			return film, err
		}
		if !s.pause(s.options.BusyRest) {
			return nil, s.ctx.Err()
		}
	}
}

// load takes the film's kept stills from the store, and tells whether there
// were any.
func (s *Service) load() bool {
	if s.options.Store == nil {
		return false
	}
	kept, err := s.options.Store.Load(s.file)
	if err != nil {
		s.warn(err)
		return false
	}
	s.loaded = kept
	return len(kept) > 0
}

// ensurePlan lays the film's grid once its length is known, with the kept
// stills on it; false once the service is closed.
func (s *Service) ensurePlan() bool {
	if s.plan != nil {
		return true
	}
	duration := s.duration()
	for duration <= 0 {
		select {
		case <-s.options.After(durationPoll):
		case <-s.ctx.Done():
			return false
		}
		duration = s.duration()
	}
	plan := storyboard.NewPlan(storyboard.For(duration))
	board := plan.Board()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, kept := range s.loaded {
		// A still asked for off this grid (another length) is left out.
		if slot := board.Slot(kept.Asked); board.At(slot) == kept.Asked {
			plan.Done(slot, kept.Picture.At)
			s.pictures[kept.Picture.At] = kept.Picture
			s.kept = append(s.kept, kept)
		}
	}
	s.loaded = nil
	s.plan = plan
	return true
}

// complete tells whether every still of the film is there.
func (s *Service) complete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, open := s.plan.Next(false, 0)
	s.finished = !open
	return !open
}

// save hands the store the film's stills when some are new.
func (s *Service) save() {
	if s.options.Store == nil {
		return
	}
	s.mu.Lock()
	if s.unsaved == 0 || s.unkept || s.discarded {
		s.mu.Unlock()
		return
	}
	stills := append([]Kept(nil), s.kept...)
	s.unsaved = 0
	s.mu.Unlock()
	if err := s.options.Store.Save(s.file, stills); err != nil {
		if errors.Is(err, ErrNoRoom) {
			s.mu.Lock()
			s.unkept = true // asked again it would only fail again
			s.mu.Unlock()
		}
		s.warn(err)
	}
}

func (s *Service) warn(err error) {
	if s.options.Warn != nil {
		s.options.Warn(err)
	}
}

// stop ends the film's stills for good and tells why: those already there
// still show, and a film with none shows the time alone.
func (s *Service) stop(err error) {
	s.mu.Lock()
	s.finished = true
	s.off = len(s.pictures) == 0
	s.mu.Unlock()
	if s.ctx.Err() == nil && s.options.OnStop != nil {
		s.options.OnStop(err)
	}
}
