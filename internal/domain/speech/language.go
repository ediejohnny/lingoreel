package speech

import (
	"slices"
)

// Telling an audio track's language by its sound. Each candidate language's
// speech model places that language's subtitle lines in the track: spoken in
// the language, the model is sure of the lines' letters; spoken in another,
// it is not. A line's sureness is the mean of its heard words' confidence,
// and a language's is the median of its lines', so a line the subtitle got
// wrong, or a song, does not decide.

const (
	// MinLanguageLines is how many lines a language needs to be judged.
	MinLanguageLines = 4
	// minLanguageScore is the sureness that makes a language the track's.
	// Measured on real sound (Sintel and a dubbed film, English and
	// Portuguese tracks): the language spoken scores 0.76 to 0.85, a language
	// not spoken 0.20 to 0.26.
	minLanguageScore = 0.6
	// minLanguageLead is how far the language found must be above any other
	// judged, so two close candidates decide nothing; the leads measured were
	// 0.53 to 0.61.
	minLanguageLead = 0.2
)

// LanguageScore is how sure a language's model was of that language's lines
// in a track: the median of the lines' sureness, over Lines lines.
type LanguageScore struct {
	Language string
	Score    float64
	Lines    int
}

// LineSureness is how sure a model was of a line's words: the mean
// confidence of the words it heard; ok is false when it heard none.
func LineSureness(words []Word) (float64, bool) {
	sum, heard := 0.0, 0
	for _, w := range words {
		if w.Heard {
			sum += w.Confidence
			heard++
		}
	}
	if heard == 0 {
		return 0, false
	}
	return sum / float64(heard), true
}

// ScoreLanguage is a language's score from its lines' sureness.
func ScoreLanguage(language string, lines []float64) LanguageScore {
	if len(lines) == 0 {
		return LanguageScore{Language: language}
	}
	sorted := slices.Clone(lines)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	median := sorted[middle]
	if len(sorted)%2 == 0 {
		median = (sorted[middle-1] + sorted[middle]) / 2
	}
	return LanguageScore{Language: language, Score: median, Lines: len(lines)}
}

// HeardLanguage is the language the track is spoken in, when one judged
// language is sure enough and clearly above every other judged one; ok is
// false when the sound does not tell.
func HeardLanguage(scores []LanguageScore) (string, bool) {
	var judged []LanguageScore
	for _, s := range scores {
		if s.Lines >= MinLanguageLines {
			judged = append(judged, s)
		}
	}
	if len(judged) == 0 {
		return "", false
	}
	slices.SortFunc(judged, func(a, b LanguageScore) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		}
		return 0
	})
	best := judged[0]
	if !(best.Score >= minLanguageScore) {
		return "", false
	}
	if len(judged) > 1 && !(best.Score-judged[1].Score >= minLanguageLead) {
		return "", false
	}
	return best.Language, true
}
