package voice_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// stereoDecoder hands a second of stereo sound for any stretch, its left
// channel 1 and its right 3, and keeps the stretches asked for.
type stereoDecoder struct {
	mu    sync.Mutex
	asked []voice.Stretch
	fail  error
}

func (d *stereoDecoder) Decode(_ context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	d.mu.Lock()
	d.asked = append(d.asked, stretch)
	d.mu.Unlock()
	if d.fail != nil {
		return d.fail
	}
	piece := make([]float32, 0, 3200)
	for range 1600 {
		piece = append(piece, 1, 3)
	}
	hear(piece, 2)
	return nil
}

// sureModel is a language's model as sure of each line as told by the
// line's first word; it checks the channels came to it averaged.
type sureModel struct {
	sureness map[string]float64
	sound    []float32
}

func (m *sureModel) Align(sound []float32, words []string) ([]speech.Word, error) {
	m.sound = sound
	if words[0] == "1,2,3" {
		return nil, speech.ErrNothingToAlign
	}
	result := make([]speech.Word, len(words))
	for i := range result {
		result[i] = speech.Word{Heard: true, Confidence: m.sureness[words[0]]}
	}
	return result, nil
}

func TestTheLinesHeardAreSpreadOverTheDialogue(t *testing.T) {
	t.Parallel()
	var events []subtitle.Event
	for i := range 100 {
		text := fmt.Sprintf("line number %d", i)
		switch i % 10 {
		case 3:
			text = "Too short"
		case 7:
			text = "♪ la la la ♪"
		}
		start := time.Duration(i) * time.Minute
		events = append(events, subtitle.Event{Start: start, End: start + 2*time.Second, OriginalText: text})
	}
	lines := voice.LinesToHear(events)
	if len(lines) != voice.LanguageLines {
		t.Fatalf("%d lines, want %d", len(lines), voice.LanguageLines)
	}
	length := events[len(events)-1].End
	for i, line := range lines {
		if len(line.Words) < 3 || slices.Contains(line.Words, "♪") {
			t.Errorf("line %d has words %q, want three or more, none sung", i, line.Words)
		}
		if line.Start < length/10 || line.End > length*9/10 {
			t.Errorf("line %d at %v, want between the opening and the credits", i, line.Start)
		}
		if i > 0 && line.Start-lines[i-1].Start < 5*time.Minute {
			t.Errorf("lines %d and %d are %v apart, want them spread", i-1, i, line.Start-lines[i-1].Start)
		}
	}
	// With fewer usable lines than asked for, every one is heard.
	var short []subtitle.Event
	for i := range 10 {
		text := "Too short"
		if i == 3 || i == 5 {
			text = "long enough to hear"
		}
		start := time.Duration(i) * time.Minute
		short = append(short, subtitle.Event{Start: start, End: start + 2*time.Second, OriginalText: text})
	}
	if few := voice.LinesToHear(short); len(few) != 2 {
		t.Errorf("from ten lines, two long enough: %d heard, want those two", len(few))
	}
}

func TestEachLanguageIsScoredByItsOwnLinesInTheTrack(t *testing.T) {
	t.Parallel()
	decoder := &stereoDecoder{}
	track := voice.Stretch{File: "film.mkv", Track: 1, Channels: 2, Codec: "A_AC3"}
	line := func(start time.Duration, first string) voice.SpokenLine {
		return voice.SpokenLine{Start: start, End: start + 2*time.Second, Words: []string{first, "and", "more"}}
	}
	english := &sureModel{sureness: map[string]float64{"a": 0.9, "b": 0.8, "c": 0.95}}
	portuguese := &sureModel{sureness: map[string]float64{"x": 0.3, "y": 0.2}}
	candidates := []voice.LanguageCandidate{
		{Language: "en", Model: english, Lines: []voice.SpokenLine{line(0, "a"), line(time.Minute, "b"), line(2*time.Minute, "c")}},
		{Language: "pt", Model: portuguese, Lines: []voice.SpokenLine{line(time.Minute, "x"), line(3*time.Minute, "y"), line(4*time.Minute, "1,2,3")}},
	}
	var progress []float64
	scores, err := voice.ScoreLanguages(t.Context(), decoder, track, candidates, func(done float64) { progress = append(progress, done) })
	if err != nil {
		t.Fatal(err)
	}
	want := []speech.LanguageScore{{Language: "en", Score: 0.9, Lines: 3}, {Language: "pt", Score: 0.25, Lines: 2}}
	if len(scores) != 2 || scores[0] != want[0] || scores[1].Lines != 2 || scores[1].Score < 0.249 || scores[1].Score > 0.251 {
		t.Fatalf("scores = %+v, want %+v (a line the model cannot place counts for nothing)", scores, want)
	}
	if len(progress) != 6 || progress[5] != 1 {
		t.Errorf("progress told %v, want six steps to 1", progress)
	}
	// Each line is decoded from the track itself, with a margin around it.
	first := decoder.asked[0]
	if first.File != "film.mkv" || first.Track != 1 || first.Codec != "A_AC3" || first.From != 0 || first.Length != 2300*time.Millisecond {
		t.Errorf("the first line was decoded as %+v, want the track from 0 for 2.3 s", first)
	}
	if second := decoder.asked[1]; second.From != time.Minute-300*time.Millisecond || second.Length != 2600*time.Millisecond {
		t.Errorf("the second line was decoded as %+v, want 300 ms each side", second)
	}
	if len(english.sound) == 0 || english.sound[0] != 2 {
		t.Errorf("the model was given %v..., want the two channels averaged (2)", english.sound[:min(len(english.sound), 2)])
	}
}

func TestADecodingThatFailsStopsTheHearing(t *testing.T) {
	t.Parallel()
	decoder := &stereoDecoder{fail: errors.New("no such track")}
	candidates := []voice.LanguageCandidate{{Language: "en", Model: &sureModel{}, Lines: []voice.SpokenLine{
		{Start: time.Second, End: 3 * time.Second, Words: strings.Fields("a b c")},
	}}}
	if _, err := voice.ScoreLanguages(t.Context(), decoder, voice.Stretch{}, candidates, nil); err == nil {
		t.Error("a failed decoding was not told")
	}
}
