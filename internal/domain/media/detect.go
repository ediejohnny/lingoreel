package media

import (
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

// frequentWords are common words of each language. Words shared by more than
// one language ("que", "de", "no") are dropped at start-up, so every hit is
// evidence for exactly one language.
var frequentWords = map[string][]string{
	"en": {"the", "and", "you", "what", "this", "that", "with", "have", "are", "was", "your", "just",
		"know", "don", "get", "there", "they", "here", "all", "out", "how", "why", "can", "will", "would",
		"right", "okay", "yeah", "he", "she", "it", "is", "not", "for", "of", "to"},
	"pt": {"não", "você", "eu", "isso", "está", "vou", "tem", "aqui", "tudo", "sim", "então", "também",
		"agora", "fazer", "ele", "ela", "meu", "minha", "nós", "vamos", "muito", "obrigado", "quem",
		"onde", "como", "uma", "com", "para", "mas", "do", "da", "é", "são", "foi"},
	"es": {"qué", "está", "estoy", "eso", "esto", "yo", "sí", "bien", "aquí", "pero", "tengo", "vamos",
		"ahora", "nada", "usted", "muy", "gracias", "dónde", "cómo", "el", "los", "las", "una", "con",
		"para", "del", "es", "son", "fue", "y"},
	"fr": {"je", "tu", "vous", "est", "pas", "une", "ce", "ça", "il", "elle", "nous", "qui", "mais",
		"pour", "avec", "suis", "oui", "bien", "très", "rien", "merci", "où", "comment", "les", "des",
		"du", "et", "sont"},
}

var wordLanguage = buildWordLanguage()

func buildWordLanguage() map[string]string {
	owners := map[string]map[string]bool{}
	for language, words := range frequentWords {
		for _, word := range words {
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
