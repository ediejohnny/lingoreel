package voice

import (
	"context"
	"errors"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Hearing which language an audio track speaks: each candidate language's
// speech model places a few lines of that language's subtitle in the track
// (speech.HeardLanguage tells the rule).

const (
	// LanguageLines is how many lines of each language are heard.
	LanguageLines = 8
	// lineMargin is the sound taken before and after a line, since a
	// subtitle's times are only near the voice's.
	lineMargin = 300 * time.Millisecond
	// minSpokenWords leaves out lines too short to tell a language by.
	minSpokenWords = 3
	// languageSpan is the part of the film the lines come from: past the
	// opening and before the credits, where the dialogue is.
	languageFrom, languageTo = 0.1, 0.9
)

// Aligner places a line's words in 16 kHz mono sound with a language's
// speech model; a line with no letter of its alphabet returns
// speech.ErrNothingToAlign.
type Aligner interface {
	Align(sound []float32, words []string) ([]speech.Word, error)
}

// WordModel is a language's speech pack opened: it aligns until closed.
type WordModel interface {
	Aligner
	Close()
}

// SpokenLine is a subtitle line to hear: when it is shown and its words as
// said.
type SpokenLine struct {
	Start, End time.Duration
	Words      []string
}

// LanguageCandidate is a language the track may speak: its speech model and
// lines of a subtitle in it.
type LanguageCandidate struct {
	Language string
	Model    Aligner
	Lines    []SpokenLine
}

// LinesToHear picks LanguageLines lines of a subtitle spread over the
// film's dialogue, each with enough words to tell a language by.
func LinesToHear(events []subtitle.Event) []SpokenLine {
	if len(events) == 0 {
		return nil
	}
	length := events[len(events)-1].End
	from, to := time.Duration(float64(length)*languageFrom), time.Duration(float64(length)*languageTo)
	var usable []SpokenLine
	for _, e := range events {
		if e.Start < from || e.End > to {
			continue
		}
		if words := subtitle.SpokenWords(e.Text()); len(words) >= minSpokenWords {
			usable = append(usable, SpokenLine{Start: e.Start, End: e.End, Words: words})
		}
	}
	if len(usable) <= LanguageLines {
		return usable
	}
	picked := make([]SpokenLine, 0, LanguageLines)
	for i := range LanguageLines {
		picked = append(picked, usable[i*len(usable)/LanguageLines])
	}
	return picked
}

// ScoreLanguages hears each candidate's lines in the track (track's File,
// Track, Channels and Codec; its times are the lines') and scores each
// language, telling how far it went from 0 to 1. A line the model cannot
// place counts for nothing.
func ScoreLanguages(ctx context.Context, decoder Decoder, track Stretch, candidates []LanguageCandidate, progress func(done float64)) ([]speech.LanguageScore, error) {
	total := 0
	for _, c := range candidates {
		total += len(c.Lines)
	}
	heard := 0
	scores := make([]speech.LanguageScore, 0, len(candidates))
	for _, c := range candidates {
		var sureness []float64
		for _, line := range c.Lines {
			stretch := track
			stretch.From = max(line.Start-lineMargin, 0)
			stretch.Length = line.End + lineMargin - stretch.From
			sound, err := DecodeMono(ctx, decoder, stretch)
			if err != nil {
				return nil, err
			}
			words, err := c.Model.Align(sound, line.Words)
			switch {
			case errors.Is(err, speech.ErrNothingToAlign), errors.Is(err, speech.ErrTooShort):
			case err != nil:
				return nil, err
			default:
				if s, ok := speech.LineSureness(words); ok {
					sureness = append(sureness, s)
				}
			}
			heard++
			if progress != nil {
				progress(float64(heard) / float64(total))
			}
		}
		scores = append(scores, speech.ScoreLanguage(c.Language, sureness))
	}
	return scores, nil
}

// DecodeMono decodes a stretch at speech.SampleRate, its channels averaged.
func DecodeMono(ctx context.Context, decoder Decoder, stretch Stretch) ([]float32, error) {
	sound := make([]float32, 0, max(int(stretch.Length.Seconds()*speech.SampleRate), 0))
	err := decoder.Decode(ctx, stretch, func(samples []float32, channels int) {
		sound = appendMono(sound, samples, channels)
	})
	if err != nil {
		return nil, err
	}
	if len(sound) == 0 {
		return nil, ErrNoSound
	}
	return sound, nil
}

// appendMono appends interleaved samples to mono sound, each frame's
// channels averaged; a frame left incomplete at the end is dropped.
func appendMono(mono, samples []float32, channels int) []float32 {
	channels = max(channels, 1)
	for i := 0; i+channels <= len(samples); i += channels {
		sum := float32(0)
		for _, s := range samples[i : i+channels] {
			sum += s
		}
		mono = append(mono, sum/float32(channels))
	}
	return mono
}

// ErrNoSound reports a stretch that decoded to no sound at all.
var ErrNoSound = errors.New("voice: the stretch has no sound")
