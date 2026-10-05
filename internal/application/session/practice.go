package session

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// ErrNoPassage reports a passage command with no passage being studied.
var ErrNoPassage = errors.New("no passage is being studied")

// lineRepeatDelay is how long after a line ends "Repetir cada fala" goes
// back, never past the start of the next line.
const lineRepeatDelay = 300 * time.Millisecond

// PracticeEvent is what a practice tells the screen.
type PracticeEvent int

const (
	// PracticeLap: a lap of the passage begins.
	PracticeLap PracticeEvent = iota + 1
	// PracticeAtEnd: the passage ended and waits for Play.
	PracticeAtEnd
	// PracticeSuspended: the video left the passage.
	PracticeSuspended
	// PracticeFinished: the goal was reached.
	PracticeFinished
	// PracticeFailed: the player did not carry out what the practice asked
	// (a pause at the end, a lap from the start), so the video may go on
	// past the passage.
	PracticeFailed
)

// PracticeStatus describes the passage being studied.
type PracticeStatus struct {
	Active    bool
	Segment   study.Segment
	Plan      study.PracticePlan
	Lap       int
	Finished  int
	Suspended bool
	AtEnd     bool
	Look      study.Lap
}

// PracticeHooks are how a practice reaches the rest of the session; nil
// ones are left out.
type PracticeHooks struct {
	// Event tells the screen what happened, with the status after it.
	Event func(PracticeEvent, PracticeStatus)
	// Look shows or hides the lines for a lap (the ladder); it is called
	// with both lines shown when the practice ends.
	Look func(study.Lap)
}

// Practice runs "Estudar trecho" and the Modo Karaokê's "Repetir cada
// fala" over the session's single clock: it pauses or loops at the end of
// the passage, sends Play at the end back to the start, and notices a seek
// out of the passage. It is safe for concurrent use; the runner calls Tick.
type Practice struct {
	controls  *Controls
	subtitles *Subtitles
	hooks     PracticeHooks
	now       func() time.Time

	mu       sync.Mutex
	segment  *study.Segment
	plan     study.PracticePlan
	lap      int
	finished int
	suspend  bool
	atEnd    bool
	resumeAt time.Time
	look     study.Lap
	repeat   int
	repeated map[string]int

	// ownSeek marks the seeks the practice makes itself, so they are not
	// taken for the user's.
	ownSeek atomic.Bool
}

// NewPractice ties a practice to the session's controls and subtitles.
func NewPractice(controls *Controls, subtitles *Subtitles, hooks PracticeHooks, now func() time.Time) *Practice {
	if now == nil {
		now = time.Now
	}
	p := &Practice{controls: controls, subtitles: subtitles, hooks: hooks, now: now,
		look: study.Lap{ShowPrimary: true, ShowTranslation: true}, repeated: map[string]int{}}
	controls.beforePlay = p.beforePlay
	controls.afterSeek = p.afterSeek
	return p
}

// SetLineRepeats sets how many times "Repetir cada fala" plays each line
// again; 0 turns it off.
func (p *Practice) SetLineRepeats(times int) error {
	if times < 0 || times > study.MaxRepeats {
		return fmt.Errorf("%w: %d repeats", study.ErrInvalidKaraoke, times)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.repeat = times
	clear(p.repeated)
	return nil
}

// LinesChanged starts the line repeats over when a line takes another
// subtitle file: the count goes by line ID, and every file numbers its
// lines from the start.
func (p *Practice) LinesChanged() {
	p.mu.Lock()
	defer p.mu.Unlock()
	clear(p.repeated)
}

// LineRepeats is how many times each line plays again.
func (p *Practice) LineRepeats() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.repeat
}

// Start studies a passage with a plan: it goes to the start and plays the
// first lap. A passage being studied is replaced.
func (p *Practice) Start(segment study.Segment, plan study.PracticePlan) error {
	if err := segment.Validate(p.controls.clock.State().Duration); err != nil {
		return err
	}
	if err := plan.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	p.segment, p.plan, p.finished = &segment, plan, 0
	p.mu.Unlock()
	return p.startLap(1)
}

// Stop leaves the passage; both lines show again.
func (p *Practice) Stop() {
	p.mu.Lock()
	active := p.segment != nil
	p.segment, p.suspend, p.atEnd, p.resumeAt = nil, false, false, time.Time{}
	p.look = study.Lap{ShowPrimary: true, ShowTranslation: true}
	p.mu.Unlock()
	if active && p.hooks.Look != nil {
		p.hooks.Look(study.Lap{ShowPrimary: true, ShowTranslation: true})
	}
}

// Resume goes back into a passage the video left, restarting its lap.
func (p *Practice) Resume() error {
	p.mu.Lock()
	if p.segment == nil {
		p.mu.Unlock()
		return ErrNoPassage
	}
	lap := max(p.lap, 1)
	p.mu.Unlock()
	return p.startLap(lap)
}

// Status describes the passage being studied.
func (p *Practice) Status() PracticeStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status()
}

func (p *Practice) status() PracticeStatus {
	if p.segment == nil {
		return PracticeStatus{Look: p.look}
	}
	return PracticeStatus{
		Active: true, Segment: *p.segment, Plan: p.plan, Lap: p.lap, Finished: p.finished,
		Suspended: p.suspend, AtEnd: p.atEnd, Look: p.look,
	}
}

// startLap applies lap n (its speed and lines), goes to the start of the
// passage and plays.
func (p *Practice) startLap(n int) error {
	p.mu.Lock()
	if p.segment == nil {
		p.mu.Unlock()
		return ErrNoPassage
	}
	segment := *p.segment
	p.lap, p.suspend, p.atEnd, p.resumeAt = n, false, false, time.Time{}
	p.look = p.plan.Lap(n)
	look, status := p.look, p.status()
	p.mu.Unlock()

	if p.hooks.Look != nil {
		p.hooks.Look(look)
	}
	var errs []error
	if look.Speed > 0 && p.controls.Capabilities().Speed {
		errs = append(errs, p.controls.SetSpeed(look.Speed))
	}
	errs = append(errs, p.seek(segment.Start))
	if p.controls.clock.State().Status != playback.StatusPlaying {
		errs = append(errs, p.controls.Play())
	}
	p.tell(PracticeLap, status)
	return errors.Join(errs...)
}

func (p *Practice) seek(position time.Duration) error {
	p.ownSeek.Store(true)
	defer p.ownSeek.Store(false)
	return p.controls.SeekTo(position)
}

func (p *Practice) tell(event PracticeEvent, status PracticeStatus) {
	if p.hooks.Event != nil {
		p.hooks.Event(event, status)
	}
}

// failed tells the screen when an order the practice gave the player on its
// own (no user command waits for the answer) went wrong. Called without mu.
func (p *Practice) failed(err error) {
	if err != nil {
		p.tell(PracticeFailed, p.Status())
	}
}

// beforePlay runs inside Controls.Play: at the end of the passage, Play
// starts the next lap from the start instead of playing on.
func (p *Practice) beforePlay() {
	p.mu.Lock()
	atEnd := p.segment != nil && (p.atEnd || !p.resumeAt.IsZero()) && !p.suspend
	next := p.lap + 1
	if atEnd {
		p.atEnd, p.resumeAt = false, time.Time{}
	}
	p.mu.Unlock()
	if atEnd {
		_ = p.startLap(next) // its errors reach the user through Play's own
	}
}

// afterSeek runs after every seek of the controls. A seek of the user
// starts the line repeats over and, out of the passage, suspends it; back
// inside, the passage goes on.
func (p *Practice) afterSeek(position time.Duration) {
	if p.ownSeek.Load() {
		return
	}
	p.mu.Lock()
	clear(p.repeated)
	if p.segment == nil {
		p.mu.Unlock()
		return
	}
	inside := p.segment.Contains(position)
	changed := p.suspend == inside
	p.suspend, p.atEnd, p.resumeAt = !inside, false, time.Time{}
	status := p.status()
	p.mu.Unlock()
	if changed && !inside {
		p.tell(PracticeSuspended, status)
	}
}

// Tick runs on every turn of the runner with the clock's state: it ends
// laps, waits out the gap between laps and repeats lines.
func (p *Practice) Tick(state playback.Sample) {
	p.mu.Lock()
	if p.segment != nil && !p.resumeAt.IsZero() && !p.suspend {
		due := !p.now().Before(p.resumeAt)
		next := p.lap + 1
		p.mu.Unlock()
		if due {
			p.failed(p.startLap(next))
		}
		return
	}
	if state.Status != playback.StatusPlaying {
		p.mu.Unlock()
		return
	}
	switch {
	case p.segment != nil:
		p.tickSegment(state) // unlocks
	case p.repeat > 0:
		p.tickLines(state) // unlocks
	default:
		p.mu.Unlock()
	}
}

// tickSegment runs with mu held and releases it.
func (p *Practice) tickSegment(state playback.Sample) {
	if p.suspend || state.Position < p.segment.End {
		p.mu.Unlock()
		return
	}
	p.finished++
	status := p.status()
	switch {
	case p.plan.Done(p.finished):
		p.mu.Unlock()
		p.failed(p.controls.Pause())
		p.Stop()
		p.tell(PracticeFinished, status)
	case p.plan.Mode == study.EndLoop && p.plan.Gap > 0:
		// The runner wakes up when the gap is over (WakeIn) and Tick starts
		// the next lap.
		p.resumeAt = p.now().Add(p.plan.Gap)
		p.mu.Unlock()
		p.failed(p.controls.Pause())
	case p.plan.Mode == study.EndLoop:
		next := p.lap + 1
		p.mu.Unlock()
		p.failed(p.startLap(next))
	default:
		p.atEnd = true
		status.AtEnd = true
		p.mu.Unlock()
		p.failed(p.controls.Pause())
		p.tell(PracticeAtEnd, status)
	}
}

// tickLines runs with mu held and releases it: a line that just ended
// plays again from its preview, up to the repeats asked for.
func (p *Practice) tickLines(state playback.Sample) {
	karaoke := p.subtitles.Karaoke()
	track := p.subtitles.Track(LinePrimary)
	if !karaoke.Enabled || track == nil || track.Timeline == nil {
		p.mu.Unlock()
		return
	}
	at := track.Offset.TrackTime(state.Position)
	line, found := track.Timeline.LastStartedBy(at)
	if !found || at < line.End {
		p.mu.Unlock()
		return
	}
	due := line.End + lineRepeatDelay
	if next, ok := track.Timeline.NextStartAfter(line.Start); ok && next.Start < due {
		due = next.Start
	}
	if at < due || at > due+lineRepeatWindow || p.repeated[line.ID] >= p.repeat {
		p.mu.Unlock()
		return
	}
	p.repeated[line.ID]++
	p.mu.Unlock()
	if target, ok := study.ReplayTarget(track.Timeline, track.Offset, track.Offset.PlaybackTime(line.Start), karaoke.Rules); ok {
		p.failed(p.seek(target))
	}
}

// lineRepeatWindow is how late after its moment a repeat still happens; a
// position further on was reached by a jump, not by playing.
const lineRepeatWindow = time.Second

// WakeIn is how long the runner may sleep before the practice needs it, in
// wall time; zero when it does not.
func (p *Practice) WakeIn(state playback.Sample) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.segment != nil && !p.resumeAt.IsZero() && !p.suspend {
		return max(p.resumeAt.Sub(p.now()), time.Millisecond)
	}
	if state.Status != playback.StatusPlaying {
		return 0
	}
	speed := float64(usableSpeed(state.Speed))
	if p.segment != nil && !p.suspend && state.Position < p.segment.End {
		return max(time.Duration(float64(p.segment.End-state.Position)/speed), time.Millisecond)
	}
	if p.segment == nil && p.repeat > 0 {
		if track := p.subtitles.Track(LinePrimary); track != nil && track.Timeline != nil {
			at := track.Offset.TrackTime(state.Position)
			if line, found := track.Timeline.LastStartedBy(at); found && at < line.End+lineRepeatDelay {
				return max(time.Duration(float64(line.End+lineRepeatDelay-at)/speed), time.Millisecond)
			}
		}
	}
	return 0
}

// ReplayLine is "Treinar de novo": back to the start of the preview of the
// line being previewed, spoken or last spoken, and playing from there, to
// speak along. While a passage is studied, the line is one of the passage:
// at its end, the last one spoken in it.
func (p *Practice) ReplayLine() error {
	track := p.subtitles.Track(LinePrimary)
	if track == nil || track.Timeline == nil {
		return ErrNotSupported
	}
	position, rules := p.controls.Position(), p.subtitles.Karaoke().Rules
	p.mu.Lock()
	var passage *study.Segment
	if p.segment != nil && !p.suspend {
		segment := *p.segment
		passage = &segment
	}
	p.mu.Unlock()
	var target time.Duration
	if passage != nil {
		target = study.ReplayTargetIn(track.Timeline, track.Offset, position, rules, *passage)
	} else {
		var ok bool
		if target, ok = study.ReplayTarget(track.Timeline, track.Offset, position, rules); !ok {
			return nil // no line yet: nothing to train again
		}
	}
	// A jump of the user's, not of the practice: a passage paused at its end
	// leaves the end, so Play goes on from the line instead of starting the
	// passage over, and the line repeats count again.
	if err := p.controls.SeekTo(target); err != nil {
		return err
	}
	if p.controls.clock.State().Status == playback.StatusPlaying {
		return nil
	}
	return p.controls.Play()
}
