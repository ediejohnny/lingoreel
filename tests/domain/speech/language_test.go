package speech_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

func TestALinesSurenessIsItsHeardWordsMean(t *testing.T) {
	t.Parallel()
	got, ok := speech.LineSureness([]speech.Word{
		{Heard: true, Confidence: 0.9}, {Heard: false}, {Heard: true, Confidence: 0.5},
	})
	if !ok || got < 0.699 || got > 0.701 {
		t.Errorf("LineSureness = %v, %v; want 0.7 from the two heard words", got, ok)
	}
	if _, ok := speech.LineSureness([]speech.Word{{Heard: false}}); ok {
		t.Error("a line with no word heard has a sureness")
	}
}

func TestALanguageScoresTheMedianOfItsLines(t *testing.T) {
	t.Parallel()
	// A song and a wrong line do not pull an otherwise sure language down.
	odd := speech.ScoreLanguage("en", []float64{0.9, 0.1, 0.92, 0.88, 0.05})
	if odd.Score != 0.88 || odd.Lines != 5 {
		t.Errorf("odd count: %+v, want the median 0.88 of 5 lines", odd)
	}
	even := speech.ScoreLanguage("pt", []float64{0.2, 0.4, 0.6, 0.8})
	if even.Score < 0.499 || even.Score > 0.501 {
		t.Errorf("even count: %+v, want the middle two's mean 0.5", even)
	}
	if none := speech.ScoreLanguage("es", nil); none.Lines != 0 || none.Score != 0 {
		t.Errorf("no line: %+v", none)
	}
}

func TestTheHeardLanguageIsSureAndClearlyAhead(t *testing.T) {
	t.Parallel()
	enough := speech.MinLanguageLines
	cases := []struct {
		name   string
		scores []speech.LanguageScore
		want   string
		ok     bool
	}{
		{"one sure language", []speech.LanguageScore{{Language: "en", Score: 0.9, Lines: enough}}, "en", true},
		{"sure and clearly ahead", []speech.LanguageScore{{Language: "en", Score: 0.35, Lines: enough}, {Language: "pt", Score: 0.85, Lines: enough}}, "pt", true},
		{"two close languages", []speech.LanguageScore{{Language: "en", Score: 0.8, Lines: enough}, {Language: "pt", Score: 0.7, Lines: enough}}, "", false},
		{"nothing sure enough", []speech.LanguageScore{{Language: "en", Score: 0.4, Lines: enough}, {Language: "pt", Score: 0.1, Lines: enough}}, "", false},
		{"too few lines to judge", []speech.LanguageScore{{Language: "en", Score: 0.95, Lines: enough - 1}}, "", false},
		{"a close rival with too few lines is not judged", []speech.LanguageScore{{Language: "en", Score: 0.85, Lines: enough}, {Language: "pt", Score: 0.84, Lines: 1}}, "en", true},
		{"no candidate", nil, "", false},
	}
	for _, c := range cases {
		got, ok := speech.HeardLanguage(c.scores)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: HeardLanguage = %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
