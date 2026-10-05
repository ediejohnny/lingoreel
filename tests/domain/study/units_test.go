package study_test

import (
	"math"
	"testing"
	"unicode"

	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// weights lists each word's time, in syllables, rounded to hundredths.
func weights(units study.Units) []float64 {
	out := make([]float64, len(units.Words))
	for i, word := range units.Words {
		out[i] = math.Round(word.Weight*100) / 100
	}
	return out
}

// Lines from films and cartoons: what is said takes time, what describes a
// sound or names a speaker does not, and a held sound takes longer.
func TestSpeechUnitsWeighWhatIsSaid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		text    string
		weights []float64
		breaks  []study.Break
		spoken  bool
		lyrics  bool
	}{
		{"a stretched shout", "Ia-ba-da-ba-duuuuu!!", []float64{6.05}, []study.Break{study.BreakStop}, true, false},
		{"a sound's description first", "[DOOR SLAMS] Get out!", []float64{0, 0, 1.3, 1.3}, []study.Break{0, 0, 0, study.BreakStop}, true, false},
		{"only a description", "(laughs)", []float64{0}, []study.Break{0}, false, false},
		{"only music", "♪ ♪", []float64{0, 0}, []study.Break{0, 0}, false, true},
		{"a song", "♪ Row, row ♪", []float64{0, 1.3, 1.3, 0}, []study.Break{0, study.BreakComma, 0, 0}, true, true},
		{"a speaker's name", "JOHN: Hi.", []float64{0, 1.3}, []study.Break{0, study.BreakStop}, true, false},
		{"a shout read as a name", "WAIT: what?", []float64{0, 1.3}, []study.Break{0, study.BreakStop}, true, false},
		{"no vowel", "Hmm. Shhh!", []float64{1.3, 1.55}, []study.Break{study.BreakStop, study.BreakStop}, true, false},
		{"hyphenated syllables", "Mm-hmm.", []float64{2.3}, []study.Break{study.BreakStop}, true, false},
		{"a silent e", "make time", []float64{1.3, 1.3}, []study.Break{0, 0}, true, false},
		{"capitals hold the same", "NOOOOO!", []float64{2.05}, []study.Break{study.BreakStop}, true, false},
		{"a pause standing alone", "How ... are you?", []float64{1.3, 0, 1.3, 1.3}, []study.Break{study.BreakStop, 0, 0, study.BreakStop}, true, false},
		{"two speakers", "- Hi.\n- Bye.", []float64{0, 1.3, 0, 1.3}, []study.Break{0, study.BreakSpeaker, 0, study.BreakStop}, true, false},
	}
	for _, c := range cases {
		units := study.SpeechUnits(c.text)
		got := weights(units)
		if len(got) != len(c.weights) {
			t.Errorf("%s: %d words %v, want %v", c.name, len(got), got, c.weights)
			continue
		}
		for i := range got {
			if got[i] != c.weights[i] || units.Words[i].Break != c.breaks[i] {
				t.Errorf("%s: word %d weighs %v with break %v, want %v with %v", c.name, i, got[i], units.Words[i].Break, c.weights[i], c.breaks[i])
			}
		}
		if units.Spoken() != c.spoken || units.Lyrics != c.lyrics {
			t.Errorf("%s: spoken %v, lyrics %v", c.name, units.Spoken(), units.Lyrics)
		}
	}
}

// The words take exactly the characters the fill paints (spaces left out),
// whatever the text, so the painted share always matches the spoken one.
func FuzzSpeechUnits(f *testing.F) {
	for _, seed := range []string{"[DOOR SLAMS] Get out!", "- Hi.\n- Bye.", "♪ Row ♪", "Ia-ba-da-ba-duuu!!", "JOHN:", "((", "))", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		visible := 0
		for _, r := range text {
			if !unicode.IsSpace(r) {
				visible++
			}
		}
		chars := 0
		for _, word := range study.SpeechUnits(text).Words {
			if word.Weight < 0 || math.IsNaN(word.Weight) || word.Chars <= 0 {
				t.Fatalf("a word %+v in %q", word, text)
			}
			chars += word.Chars
		}
		if chars != visible {
			t.Fatalf("the words take %d characters of %q, the line shows %d", chars, text, visible)
		}
	})
}
