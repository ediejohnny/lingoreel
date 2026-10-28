package voice_test

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// placingModel places a line's words where a test says, in the film's time,
// as sure as it says; it keeps what it was asked and whether it was closed.
type placingModel struct {
	at     []speech.Span
	sure   float64
	mu     sync.Mutex
	words  []string
	sound  int
	closed atomic.Bool
}

// soundFrom is where the sound handed for "How... are you?" begins: the
// margin before the line, which a voice starting early needs.
const soundFrom = 1900*ms - speech.LineMargin

func (m *placingModel) Align(sound []float32, words []string) ([]speech.Word, error) {
	m.mu.Lock()
	m.words, m.sound = slices.Clone(words), len(sound)
	m.mu.Unlock()
	placed := make([]speech.Word, len(m.at))
	for i, span := range m.at {
		placed[i] = speech.Word{Span: speech.Span{Start: span.Start - soundFrom, End: span.End - soundFrom}, Heard: true, Confidence: m.sure}
	}
	return placed, nil
}

func (m *placingModel) Close() { m.closed.Store(true) }

func (m *placingModel) asked() ([]string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.words, m.sound
}

// The film says "How..." at 2.0 s and "are you?" at 4.25 s; the model places
// the line's words in the pause between them, where no voice is found, so a
// fill there follows the words and not the voice.
var wordsInThePause = []speech.Span{{Start: 2500 * ms, End: 2700 * ms}, {Start: 3000 * ms, End: 3200 * ms}, {Start: 3300 * ms, End: 3500 * ms}}

func followWithWords(model *placingModel, line voice.Line) *voice.Follower {
	lines := func(from, to time.Duration) []voice.Line {
		if line.End > from && line.Start < to {
			return []voice.Line{line}
		}
		return nil
	}
	return voice.Follow(context.Background(), voice.Sources{Model: testkit.LoudModel{}, Decoder: decoder{sound: film()},
		Voice: voice.Stretch{File: "film.mkv"}, Words: model, Lines: lines}, newClock(time.Now()), voice.Settings{Sensitivity: speech.SensitivityNormal})
}

// With the line's words placed in the sound before it shows, the fill
// follows them word by word; the model is handed the line's words said and
// its sound with a margin on each side, and closes with the follower.
func TestTheFillFollowsTheWordsPlacedInTheSound(t *testing.T) {
	t.Parallel()
	model := &placingModel{at: wordsInThePause, sure: 0.9}
	follower := followWithWords(model, howAreYou)
	testkit.Eventually(t, "the line painted by its words", func() bool {
		share, ok := follower.Fill(howAreYou, 3500*ms)
		return ok && share == 1
	})
	within(t, `halfway through "How..."`, fill(t, follower, howAreYou, 2600*ms), 3, 3)
	within(t, `between "How..." and "are"`, fill(t, follower, howAreYou, 2850*ms), 6, 6)
	within(t, `halfway through "are"`, fill(t, follower, howAreYou, 3100*ms), 7.5, 7.5)

	words, sound := model.asked()
	if !slices.Equal(words, []string{"How...", "are", "you?"}) {
		t.Errorf("the model was asked %q, want the line's words said", words)
	}
	if want := int((howAreYou.End - howAreYou.Start + 2*speech.LineMargin).Seconds() * speech.SampleRate); sound != want {
		t.Errorf("the model heard %d samples, want %d: the line and its margin on each side", sound, want)
	}
	follower.Close()
	if !model.closed.Load() {
		t.Error("the words model was left open after the follower closed")
	}
}

// Words the model is not sure of (a shout, a whisper) leave the line to the
// voice found: "How..." said, and the pause held.
func TestUnsureWordsLeaveTheLineToTheVoiceFound(t *testing.T) {
	t.Parallel()
	model := &placingModel{at: wordsInThePause, sure: 0.2}
	follower := followWithWords(model, howAreYou)
	defer follower.Close()
	testkit.Eventually(t, "the words tried and the line decoded", func() bool {
		words, _ := model.asked()
		_, ok := follower.Fill(howAreYou, 3500*ms)
		return words != nil && ok
	})
	within(t, "in the pause, by the voice found", fill(t, follower, howAreYou, 3500*ms), 5.5, 6)
}

// A film's last line may show past the film's end, where no sound ever
// comes: its words are placed in the sound up to the voice track's end, and
// the fill follows them.
func TestTheLastLineIsPlacedInTheSoundUpToTheFilmsEnd(t *testing.T) {
	t.Parallel()
	model := &placingModel{at: wordsInThePause, sure: 0.9}
	last := voice.Line{Text: howAreYou.Text, Start: howAreYou.Start, End: 7 * time.Second}
	follower := followWithWords(model, last)
	defer follower.Close()
	testkit.Eventually(t, "the last line painted by its words", func() bool {
		share, ok := follower.Fill(last, 3500*ms)
		return ok && share == 1
	})
	if _, sound := model.asked(); sound != int((filmPeriod-soundFrom).Seconds()*speech.SampleRate) {
		t.Errorf("the model heard %d samples, want the sound from the margin before the line to the film's end", sound)
	}
}
