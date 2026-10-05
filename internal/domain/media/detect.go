package media

import (
	"embed"
	"io/fs"
	"path"
	"strings"
	"unicode"
)

// Content detection needs enough evidence and a clear winner; otherwise the
// language stays unknown rather than guessed.
const (
	maxDetectionWords = 5000
	minDetectionHits  = 15
	minDetectionLead  = 2.0
)

// frequentWords are common words of each language, one file per language
// tag under words/: a language is added by adding its file. Words shared by
// more than one language ("que", "de", "no") are dropped at start-up, so
// every hit is evidence for exactly one language.
//
//go:embed words/*.txt
var frequentWords embed.FS

var wordLanguage = buildWordLanguage()

func buildWordLanguage() map[string]string {
	owners := map[string]map[string]bool{}
	files, _ := fs.Glob(frequentWords, "words/*.txt") // the pattern is well formed
	for _, file := range files {
		data, err := frequentWords.ReadFile(file)
		if err != nil {
			continue
		}
		language := strings.TrimSuffix(path.Base(file), ".txt")
		for _, word := range strings.Fields(string(data)) {
			if owners[word] == nil {
				owners[word] = map[string]bool{}
			}
			owners[word][language] = true
		}
	}
	index := map[string]string{}
	for word, languages := range owners {
		if len(languages) == 1 {
			for language := range languages {
				index[word] = language
			}
		}
	}
	return index
}

// DetectLanguage guesses the language of subtitle text from its frequent
// words. It returns "" when the text is too short or no language clearly
// wins.
func DetectLanguage(text string) LanguageTag {
	hits := map[string]int{}
	words := 0
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if words++; words > maxDetectionWords {
			break
		}
		if language, found := wordLanguage[word]; found {
			hits[language]++
		}
	}
	best, second := "", 0
	for language, count := range hits {
		if best == "" || count > hits[best] || (count == hits[best] && language < best) {
			if best != "" {
				second = max(second, hits[best])
			}
			best = language
		} else {
			second = max(second, count)
		}
	}
	if best == "" || hits[best] < minDetectionHits || float64(hits[best]) < minDetectionLead*float64(second) {
		return ""
	}
	return LanguageTag(best)
}
