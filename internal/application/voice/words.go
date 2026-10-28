package voice

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Placing a line's words in its sound. While the voice track is decoded
// ahead, its sound is kept as it comes, and each line coming has its words
// placed in it by its language's speech pack (Sources.Words) before it
// shows. The fill then follows the words (study.PlanWords), which a quick
// speaker, a neighbour's voice, a noise or music under the line do not
// mislead as they do the stretches of voice found; a shout, whose letters
// the model is not sure of, is still where its words were placed.

const (
	// recordingSpan is how much decoded sound is kept for placing words:
	// the window ahead and the lines just played. trimSlack lets it grow
	// past that a little, so the old sound is let go in one copy.
	recordingSpan = 2 * time.Minute
	trimSlack     = 30 * time.Second
	// wordsMargin is the sound placed with a line on each side: its voice
	// may begin before the line shows and end after it goes.
	wordsMargin = speech.LineMargin
	// wordsDuty is the share of one processor placing words takes, more
	// than decoding's: a film's dialogue takes about a third of its time.
	wordsDuty = 0.5
)

// recording is the voice track's sound as decoded, at speech.SampleRate in
// mono, from a moment of the film on, and where that sound ends once a
// decode reached it. It is safe for concurrent use.
type recording struct {
	mu      sync.Mutex
	start   time.Duration
	samples []float32
	ending  time.Duration
	ended   bool
}

// finish keeps where the voice track's sound ends.
func (r *recording) finish(at time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ending, r.ended = at, true
}

// add keeps sound decoded from a moment on: after what is kept, or over its
// end when a window is decoded from a little before it; anywhere else it is
// kept anew. Past recordingSpan the oldest sound goes.
func (r *recording) add(at time.Duration, mono []float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if kept := samplesIn(at - r.start); len(r.samples) == 0 || kept < 0 || kept > len(r.samples) {
		r.start, r.samples = at, r.samples[:0]
	} else {
		r.samples = r.samples[:kept]
	}
	r.samples = append(r.samples, mono...)
	if extra := len(r.samples) - samplesIn(recordingSpan); extra > samplesIn(trimSlack) {
		r.samples = append([]float32(nil), r.samples[extra:]...)
		r.start += time.Duration(extra) * time.Second / speech.SampleRate
	}
}

// clip is a copy of the sound from one moment to another, when it is all
// kept; past the voice track's end, the sound up to it.
func (r *recording) clip(from, to time.Duration) ([]float32, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		to = min(to, r.ending)
	}
	first, last := samplesIn(from-r.start), samplesIn(to-r.start)
	if first < 0 || last > len(r.samples) || last <= first {
		return nil, false
	}
	return append([]float32(nil), r.samples[first:last]...), true
}

// end is the moment the sound kept reaches.
func (r *recording) end() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.endLocked()
}

// holds tells whether the sound up to a moment is all kept: it is decoded
// that far, or to the voice track's end, past which there is none.
func (r *recording) holds(to time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	end := r.endLocked()
	return to <= end || r.ended && end >= r.ending
}

// endLocked is end, with r.mu held.
func (r *recording) endLocked() time.Duration {
	return r.start + time.Duration(len(r.samples))*time.Second/speech.SampleRate
}

// samplesIn is how many samples a stretch of time holds, rounded down;
// negative for a negative stretch.
func samplesIn(d time.Duration) int {
	return int(d * speech.SampleRate / time.Second)
}

// placeWords places the words of the lines coming in the sound decoded, in
// their order, until ctx ends. Each line is tried once; what it was placed
// at stays for the film (a seek does not change where a word is said).
func (f *Follower) placeWords(ctx context.Context) {
	paced := pace{duty: wordsDuty}
	tried := map[Line]bool{}
	for {
		position := f.clock.PositionAt(time.Now())
		for _, line := range f.sources.Lines(position, f.sound.end()) {
			if tried[line] || !f.sound.holds(line.End+wordsMargin) {
				continue
			}
			tried[line] = true
			began := time.Now()
			if words, ok := f.placeLine(line); ok {
				f.mu.Lock()
				f.placed[line] = words
				f.mu.Unlock()
			}
			if !paced.worked(ctx, time.Since(began)) {
				return
			}
		}
		for line := range tried {
			if line.End < position-recordingSpan {
				delete(tried, line)
			}
		}
		if !wait(ctx, aheadPoll) {
			return
		}
	}
}

// placeLine places a line's words in the sound kept around it, with the
// times shifted to the film's; false when the sound is not kept, nothing in
// the line is said, or the model heard none of its words.
func (f *Follower) placeLine(line Line) ([]speech.Word, bool) {
	said := subtitle.SpokenWords(line.Text)
	if len(said) == 0 {
		return nil, false
	}
	from := max(line.Start-wordsMargin, 0)
	sound, ok := f.sound.clip(from, line.End+wordsMargin)
	if !ok {
		return nil, false
	}
	words, err := f.sources.Words.Align(sound, said)
	if err != nil || !slices.ContainsFunc(words, func(w speech.Word) bool { return w.Heard }) {
		return nil, false
	}
	for i := range words {
		words[i].Start += from
		words[i].End += from
	}
	return words, true
}
