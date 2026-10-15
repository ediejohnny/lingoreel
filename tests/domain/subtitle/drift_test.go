package subtitle_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// voiceLag is how much later than its line a voice starts.
const voiceLag = 120 * time.Millisecond

// spokenFilm is the voice of a whole film as the model hears it: the lines
// of the file, said voiceLag after they show and, from cut on, slip later.
func spokenFilm(file []speech.Span, length, cut, slip time.Duration) speech.Track {
	random := rand.New(rand.NewPCG(4, 4))
	frames := make([]speech.Frame, length/speech.FrameLength)
	for i := range frames {
		frames[i].Speech = float32(0.05 + 0.2*random.Float64())
	}
	for _, line := range file {
		moved := voiceLag
		if line.Start >= cut {
			moved += slip
		}
		for at := line.Start + moved + 50*time.Millisecond; at < line.End+moved-50*time.Millisecond; at += speech.FrameLength {
			if i := int(at / speech.FrameLength); i < len(frames) {
				frames[i].Speech = 0.9
			}
		}
	}
	return speech.Track{Frames: frames}
}

// window is the voice heard around a playhead: what a follower keeps.
func window(film speech.Track, position time.Duration) (speech.Track, time.Duration) {
	from := max(position-100*time.Second, 0)
	to := min(position+20*time.Second, time.Duration(len(film.Frames))*speech.FrameLength)
	first, last := int(from/speech.FrameLength), int(to/speech.FrameLength)
	return speech.Track{Start: from, Frames: film.Frames[first:last]}, to
}

func shifted(lines []speech.Span, by time.Duration) []speech.Span {
	out := make([]speech.Span, len(lines))
	for i, l := range lines {
		out[i] = speech.Span{Start: l.Start + by, End: l.End + by}
	}
	return out
}

// playback plays a film at 1× from one position to another, reading a
// window every 5 s, and moves the lines as the tracker says; manual is the
// user's own shift.
type playback struct {
	t      *testing.T
	file   []speech.Span
	film   speech.Track
	drift  subtitle.DriftTracker
	start  time.Time
	wall   time.Duration // the wall clock goes on through seeks
	manual time.Duration
	steps  []time.Duration // positions where the lines moved
}

func (p *playback) play(from, to time.Duration) {
	for position := from; position < to; position += 5 * time.Second {
		now := p.start.Add(p.wall)
		p.wall += 5 * time.Second
		heard, end := window(p.film, position)
		reading := subtitle.ReadDrift(heard, shifted(p.file, p.manual+p.drift.Auto()))
		p.drift.Heard(reading, position, end, now)
		if _, moved := p.drift.Step(now); moved {
			p.steps = append(p.steps, position)
		}
	}
}

func newPlayback(t *testing.T, cut, slip time.Duration) *playback {
	const film = 40 * time.Minute
	file := dialogue(film, 7)
	return &playback{t: t, file: file, film: spokenFilm(file, film, cut, slip), start: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)}
}

func near(got, want, tolerance time.Duration) bool {
	return got >= want-tolerance && got <= want+tolerance
}

// A version with a scene cut at 20:00 shows every later line 800 ms early:
// the lines move to the voice soon after, in small steps, and stay; going
// back before the cut puts them back.
func TestALineThatSlipsMidFilmFollowsTheVoice(t *testing.T) {
	t.Parallel()
	const cut, slip = 20 * time.Minute, 800 * time.Millisecond
	p := newPlayback(t, cut, slip)
	p.play(0, 26*time.Minute)

	if len(p.steps) == 0 {
		t.Fatal("the lines never moved")
	}
	if p.steps[0] < cut-20*time.Second {
		t.Errorf("first move at %v, before the cut at %v", p.steps[0], cut)
	}
	if last := p.steps[len(p.steps)-1]; last > cut+4*time.Minute {
		t.Errorf("still moving at %v", last)
	}
	if got := p.drift.Auto(); !near(got, slip, 60*time.Millisecond) {
		t.Errorf("auto = %v, want %v ± 60 ms", got, slip)
	}
	if len(p.steps) > 6 {
		t.Errorf("%d moves for %v: the steps are too small or it wavers", len(p.steps), slip)
	}
	anchors := p.drift.Anchors()
	if len(anchors) != 1 || anchors[0].From < cut-2*time.Minute || anchors[0].From > cut+2*time.Minute {
		t.Fatalf("anchors %v, want one near %v", anchors, cut)
	}

	if got := p.drift.Seek(10 * time.Minute); got != 0 {
		t.Errorf("before the cut the lines take %v, want 0", got)
	}
	if got := p.drift.Seek(30 * time.Minute); !near(got, slip, 60*time.Millisecond) {
		t.Errorf("after the cut the lines take %v, want %v", got, slip)
	}
}

// A part of the film not heard yet teaches the tracker even after a seek
// back: watching the start, jumping far ahead and coming back to where the
// line slips still brings it to the voice.
func TestASeekBackToVoiceNotHeardStillCorrects(t *testing.T) {
	t.Parallel()
	const cut, slip = 20 * time.Minute, 800 * time.Millisecond
	p := newPlayback(t, cut, slip)
	p.play(0, 12*time.Minute)
	p.drift.Seek(38 * time.Minute)
	p.play(38*time.Minute, 39*time.Minute)
	p.drift.Seek(cut)
	p.play(cut, cut+8*time.Minute)
	if got := p.drift.Auto(); !near(got, slip, 60*time.Millisecond) {
		t.Errorf("auto = %v after coming back to the slip, want %v ± 60 ms", got, slip)
	}
}

// A file that never slips is never moved, however long it plays.
func TestALineOnTheVoiceStaysPut(t *testing.T) {
	t.Parallel()
	p := newPlayback(t, time.Hour, 0)
	p.play(0, 40*time.Minute)
	if len(p.steps) != 0 || p.drift.Auto() != 0 {
		t.Errorf("moves at %v, auto %v", p.steps, p.drift.Auto())
	}
}

// Voice heard again (a passage studied over and over, a seek back) is no
// new evidence: the same window, read many times, moves nothing.
func TestTheSameVoiceHeardAgainTellsNothing(t *testing.T) {
	t.Parallel()
	const cut = 20 * time.Minute
	p := newPlayback(t, cut, 800*time.Millisecond)
	p.play(0, cut-2*time.Minute)
	heard, end := window(p.film, cut+3*time.Minute)
	reading := subtitle.ReadDrift(heard, p.file)
	if !reading.Trusted() {
		t.Fatalf("the window after the cut is not trusted: %+v", reading)
	}
	for i := range 20 {
		now := p.start.Add(cut + time.Duration(i)*5*time.Second)
		p.drift.Heard(reading, cut+3*time.Minute, end, now)
		if _, moved := p.drift.Step(now); moved && i > 0 {
			t.Fatalf("moved after hearing the same window %d times", i+1)
		}
	}
	if p.drift.Auto() != 0 {
		t.Errorf("auto = %v from one window", p.drift.Auto())
	}
}

// The user's own correction wins: moving the line by hand is the new base,
// and moving it against a step just taken stops the tracker.
func TestTheUsersCorrectionWins(t *testing.T) {
	t.Parallel()
	p := newPlayback(t, time.Hour, 0)
	p.play(0, 10*time.Minute)
	p.manual = 300 * time.Millisecond
	p.drift.Nudged(p.manual, p.start.Add(10*time.Minute))
	p.play(10*time.Minute, 30*time.Minute)
	if len(p.steps) != 0 {
		t.Errorf("the tracker moved the line back at %v", p.steps)
	}

	q := newPlayback(t, 20*time.Minute, 800*time.Millisecond)
	q.play(0, 22*time.Minute)
	if len(q.steps) == 0 {
		t.Fatal("no step to overrule")
	}
	last := q.start.Add(q.steps[len(q.steps)-1])
	q.drift.Nudged(-100*time.Millisecond, last.Add(10*time.Second))
	if !q.drift.Stopped() {
		t.Fatal("a move against a step just taken did not stop the tracker")
	}
	before := q.drift.Auto()
	q.play(22*time.Minute, 30*time.Minute)
	if q.drift.Auto() != before {
		t.Errorf("a stopped tracker moved from %v to %v", before, q.drift.Auto())
	}
}

// A window that cannot tell is not trusted: few lines, or voice all over
// it (a song, a crowd).
func TestAWindowThatCannotTellIsNotTrusted(t *testing.T) {
	t.Parallel()
	p := newPlayback(t, time.Hour, 0)
	heard, _ := window(p.film, 10*time.Minute)
	if r := subtitle.ReadDrift(heard, p.file); !r.Trusted() {
		t.Fatalf("a plain window of dialogue is not trusted: %+v", r)
	}
	if r := subtitle.ReadDrift(heard, p.file[:30]); r.Trusted() {
		t.Errorf("a window with no lines in it is trusted: %+v", r)
	}
	loud := speech.Track{Start: heard.Start, Frames: make([]speech.Frame, len(heard.Frames))}
	for i := range loud.Frames {
		loud.Frames[i].Speech = 0.95
	}
	if r := subtitle.ReadDrift(loud, p.file); r.Trusted() {
		t.Errorf("a window of voice everywhere is trusted: %+v", r)
	}
}
