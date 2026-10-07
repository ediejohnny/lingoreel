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
	// Delay is how long the film plays before the first still, so the
	// player's opening is not disturbed; the pointer on the bar ends it.
	Delay time.Duration
	// Duty is the share of the time the background stills work, resting the
	// rest; the first Overview stills and those the pointer waits for do not
	// rest.
	Duty     float64
	Overview int
	// GiveUp is how many stills in a row may fail before the film's stills
	// stop.
	GiveUp int
	// OnStop hears, once, why the stills stopped before the film was covered.
	OnStop func(err error)
	Now    func() time.Time
	After  func(time.Duration) <-chan time.Time
}

// ErrGaveUp reports a film whose stills kept failing.
var ErrGaveUp = errors.New("the stills kept failing")

const (
	defaultDelay    = 10 * time.Second
	defaultDuty     = 0.25
	defaultOverview = 100
	defaultGiveUp   = 3
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
	// wanted is the moment the pointer is on; pointed, a want not yet served.
	wanted   time.Duration
	pointed  bool
	finished bool
	off      bool
}

// Start begins taking a film's stills. duration tells the film's length,
// zero while unknown.
func Start(file string, open Opener, duration func() time.Duration, options Options) *Service {
	if options.Delay <= 0 {
		options.Delay = defaultDelay
	}
	if !(options.Duty > 0 && options.Duty <= 1) {
		options.Duty = defaultDuty
	}
	options.Overview = cmpOr(options.Overview, defaultOverview)
	options.GiveUp = cmpOr(options.GiveUp, defaultGiveUp)
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
	s.wanted, s.pointed = at, true
	s.mu.Unlock()
	s.poke()
}

// Close stops taking stills, without waiting for the one being taken.
func (s *Service) Close() {
	if s != nil {
		s.cancel()
	}
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

func (s *Service) run() {
	film, err := s.open(s.ctx, s.file)
	if err != nil {
		s.stop(err)
		return
	}
	if !s.pause(s.options.Delay) {
		return
	}
	duration := s.duration()
	for duration <= 0 {
		select {
		case <-s.options.After(durationPoll):
		case <-s.ctx.Done():
			return
		}
		duration = s.duration()
	}
	plan := storyboard.NewPlan(storyboard.For(duration))
	s.mu.Lock()
	s.plan = plan
	s.mu.Unlock()
	failures, taken := 0, 0
	for {
		s.mu.Lock()
		slot, waited, ok := plan.Next(s.pointed, s.wanted)
		s.pointed = false
		s.finished = !ok
		s.mu.Unlock()
		if !ok {
			return
		}
		began := s.options.Now()
		picture, err := film.Grab(s.ctx, Shot{At: plan.Board().At(slot), Waited: waited})
		if s.ctx.Err() != nil {
			return
		}
		s.mu.Lock()
		if err != nil {
			plan.Skip(slot)
			failures++
		} else {
			plan.Done(slot, picture.At)
			s.pictures[picture.At] = picture
			failures, taken = 0, taken+1
		}
		s.mu.Unlock()
		if failures >= s.options.GiveUp {
			s.stop(errors.Join(ErrGaveUp, err))
			return
		}
		if !waited && taken > s.options.Overview {
			work := s.options.Now().Sub(began)
			if !s.pause(time.Duration(float64(work) * (1 - s.options.Duty) / s.options.Duty)) {
				return
			}
		}
	}
}

// stop turns the film's stills off for good and tells why.
func (s *Service) stop(err error) {
	s.mu.Lock()
	s.off = true
	s.mu.Unlock()
	if s.ctx.Err() == nil && s.options.OnStop != nil {
		s.options.OnStop(err)
	}
}
