package voice

import (
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// RecentSound is how much of the sound heard Memory keeps: enough to
// train the lines just spoken again.
const RecentSound = 2 * time.Minute

// joinTolerance is how far a piece may land from where the last one ended
// and still follow it: the clock the pieces are placed by wavers a little.
const joinTolerance = 50 * time.Millisecond

// Memory keeps the frames of the sound heard, by the film's clock, so a line
// trained again is painted from its voice found ahead. It is safe for
// concurrent use.
type Memory struct {
	mu    sync.Mutex
	runs  []speech.Track // in the order heard; each one unbroken
	limit int            // frames kept
	kept  int
}

// NewMemory keeps up to a length of sound.
func NewMemory(length time.Duration) *Memory {
	return &Memory{limit: int(length / speech.FrameLength)}
}

// Add records frames heard from a position of the film. Frames that do not
// follow the last ones (a seek, a pause) start a new run.
func (m *Memory) Add(at time.Duration, frames []speech.Frame) {
	if len(frames) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if n := len(m.runs); n > 0 {
		last := &m.runs[n-1]
		end := last.Start + time.Duration(len(last.Frames))*speech.FrameLength
		if (at - end).Abs() <= joinTolerance {
			last.Frames = append(last.Frames, frames...)
			m.kept += len(frames)
			m.forget()
			return
		}
	}
	m.runs = append(m.runs, speech.Track{Start: at, Frames: append([]speech.Frame(nil), frames...)})
	m.kept += len(frames)
	m.forget()
}

// forget drops the oldest frames past the limit.
func (m *Memory) forget() {
	for m.kept > m.limit {
		oldest := &m.runs[0]
		if drop := m.kept - m.limit; drop < len(oldest.Frames) {
			oldest.Frames = oldest.Frames[drop:]
			oldest.Start += time.Duration(drop) * speech.FrameLength
			m.kept -= drop
			return
		}
		m.kept -= len(oldest.Frames)
		m.runs = m.runs[1:]
	}
}

// Track returns the frames heard for a stretch of the film and the
// speech.LineMargin around it, as far as they were heard; false when no
// run holds the stretch whole.
func (m *Memory) Track(from, to time.Duration) (speech.Track, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.runs) - 1; i >= 0; i-- { // the latest hearing wins
		run := m.runs[i]
		end := run.Start + time.Duration(len(run.Frames))*speech.FrameLength
		if run.Start <= from && to <= end {
			first := max(int((from-speech.LineMargin-run.Start)/speech.FrameLength), 0)
			last := min(int((to+speech.LineMargin-run.Start+speech.FrameLength-1)/speech.FrameLength), len(run.Frames))
			return speech.Track{
				Start:  run.Start + time.Duration(first)*speech.FrameLength,
				Frames: append([]speech.Frame(nil), run.Frames[first:last]...),
			}, true
		}
	}
	return speech.Track{}, false
}
