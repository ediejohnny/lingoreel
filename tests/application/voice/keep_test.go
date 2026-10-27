package voice_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

const (
	keptFilm = 40 * time.Minute
	keptCut  = 20 * time.Minute
	keptSlip = 800 * time.Millisecond
)

// keptFile is a film's subtitle file: a line every few seconds, each said
// 120 ms after it shows, and from keptCut on said keptSlip later (a version
// with a scene added); heard is that voice as the follower keeps it.
func keptFile(t *testing.T, slip time.Duration) (*subtitle.Timeline, *voice.Memory) {
	t.Helper()
	random := rand.New(rand.NewPCG(7, 3))
	var events []subtitle.Event
	frames := make([]speech.Frame, keptFilm/speech.FrameLength)
	for i := range frames {
		frames[i].Speech = float32(0.05 + 0.2*random.Float64())
	}
	for at := 20 * time.Second; at < keptFilm-10*time.Second; {
		said := time.Second + time.Duration(random.Int64N(int64(2*time.Second)))
		events = append(events, subtitle.Event{Start: at, End: at + said, OriginalText: "a line"})
		lag := 120 * time.Millisecond
		if at >= keptCut {
			lag += slip
		}
		for v := at + lag + 50*time.Millisecond; v < at+said+lag-50*time.Millisecond; v += speech.FrameLength {
			if i := int(v / speech.FrameLength); i < len(frames) {
				frames[i].Speech = 0.9
			}
		}
		at += said + 500*time.Millisecond + time.Duration(random.Int64N(int64(5500*time.Millisecond)))
	}
	timeline, err := subtitle.NewTimeline(events)
	if err != nil {
		t.Fatal(err)
	}
	heard := voice.NewMemory(keptFilm + time.Minute)
	heard.Add(0, frames)
	return timeline, heard
}

// keeping plays a film at 1× through a LineKeeper, a turn every 5 s.
type keeping struct {
	keeper voice.LineKeeper
	line   voice.KeptLine
	heard  *voice.Memory
	start  time.Time
	wall   time.Duration
	seeks  uint64
	shift  time.Duration
	moves  int
}

func newKeeping(t *testing.T) *keeping {
	timeline, heard := keptFile(t, keptSlip)
	return &keeping{line: voice.KeptLine{Number: 3, Timeline: timeline}, heard: heard, start: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)}
}

func (k *keeping) turn(position time.Duration) bool {
	now := k.start.Add(k.wall)
	k.wall += voice.KeepEvery
	shift, changed := k.keeper.Turn(now, position, k.seeks, k.line, k.heard.Around)
	k.shift = shift
	if changed {
		k.moves++
	}
	return changed
}

func (k *keeping) play(from, to time.Duration) {
	for position := from; position < to; position += voice.KeepEvery {
		k.turn(position)
	}
}

// near tells a shift within what the drift tracker settles for: it ends a
// correction once less than 125 ms is left against the voice it learned at
// the film's start (how close it gets is subtitle's own test).
func near(got, want time.Duration) bool {
	return got >= want-125*time.Millisecond && got <= want+125*time.Millisecond
}

// A film that slips mid-way brings the study line to the voice after the
// slip, and a seek takes the shift settled for that part of the film.
func TestTheStudyLineIsKeptOnTheVoice(t *testing.T) {
	t.Parallel()
	k := newKeeping(t)
	k.play(0, keptCut-time.Minute)
	if k.shift != 0 || k.moves != 0 {
		t.Fatalf("before the slip the line moved %d times, to %v", k.moves, k.shift)
	}
	k.play(keptCut-time.Minute, keptCut+6*time.Minute)
	if !near(k.shift, keptSlip) {
		t.Fatalf("after the slip the line is shown %v later, want %v", k.shift, keptSlip)
	}

	k.seeks++
	if !k.turn(10*time.Minute) || k.shift != 0 {
		t.Errorf("a seek back before the slip shows the line %v later", k.shift)
	}
	k.seeks++
	if k.turn(30 * time.Minute); !near(k.shift, keptSlip) {
		t.Errorf("a seek past the slip shows the line %v later, want %v", k.shift, keptSlip)
	}
}

// Another file on the line starts over: the shift found for the one before
// is not the new one's.
func TestAnotherFileOnTheLineStartsOver(t *testing.T) {
	t.Parallel()
	k := newKeeping(t)
	k.play(0, keptCut+6*time.Minute)
	if k.shift == 0 {
		t.Fatal("no shift to take away")
	}
	k.line.Number = 4
	if !k.turn(keptCut+6*time.Minute) || k.shift != 0 {
		t.Errorf("another file kept the shift %v", k.shift)
	}
}

// The user's word wins: right after the voice's first move, a step of the
// line's own offset against it, or putting the line back to its file's
// timing, leaves the line where it is for the session.
func TestTheUserOverrulesTheVoice(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		overrule  string
		own, then subtitle.Offset
	}{
		{"step against", 0, subtitle.NewOffset(-100 * time.Millisecond)},
		{"back to the file", subtitle.NewOffset(300 * time.Millisecond), 0},
	} {
		k := newKeeping(t)
		k.line.Offset = c.own
		position := time.Duration(0)
		for ; k.moves == 0 && position < keptCut+6*time.Minute; position += voice.KeepEvery {
			k.turn(position)
		}
		if k.moves == 0 {
			t.Fatalf("%s: no move to overrule", c.overrule)
		}
		k.line.Offset = c.then
		before, moves := k.shift, k.moves
		k.play(position, keptCut+8*time.Minute)
		if k.shift != before || k.moves != moves {
			t.Errorf("%s: the line moved on from %v to %v", c.overrule, before, k.shift)
		}
	}
}

// Turned off, the keeper tells there was a shift to take away, once.
func TestForgettingTellsWhetherTheLineWasMoved(t *testing.T) {
	t.Parallel()
	k := newKeeping(t)
	if k.keeper.Forget() {
		t.Fatal("a keeper that never moved the line had a shift to take away")
	}
	k.play(0, keptCut+6*time.Minute)
	if !k.keeper.Forget() || k.keeper.Forget() {
		t.Fatal("forgetting a moved line did not tell it once")
	}
}

// The voice around a moment comes from the latest hearing that holds it,
// cut to the reach asked; a moment never heard gives nothing.
func TestTheVoiceAroundAMomentIsTheLatestHeard(t *testing.T) {
	t.Parallel()
	memory := voice.NewMemory(10 * time.Minute)
	frames := func(n int, p float32) []speech.Frame {
		out := make([]speech.Frame, n)
		for i := range out {
			out[i].Speech = p
		}
		return out
	}
	second := int(time.Second / speech.FrameLength)
	memory.Add(0, frames(60*second, 0.1))
	memory.Add(30*time.Second, frames(60*second, 0.9)) // heard again after a seek back
	track, found := memory.Around(40*time.Second, 5*time.Second, 2*time.Second)
	if !found || track.Start != 35*time.Second || len(track.Frames) != 7*second || track.Frames[0].Speech != 0.9 {
		t.Fatalf("Around(40 s) = from %v, %d frames, first %v (found %v)", track.Start, len(track.Frames), track.Frames[0].Speech, found)
	}
	if track, _ := memory.Around(31*time.Second, 5*time.Second, 0); track.Start != 30*time.Second {
		t.Errorf("Around(31 s) starts at %v, before the hearing that holds it", track.Start)
	}
	if _, found := memory.Around(5*time.Minute, time.Second, time.Second); found {
		t.Error("a moment never heard gave voice")
	}
}
