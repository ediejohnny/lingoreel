package media

import "strings"

// samplePhrases say "this is a preview" in the languages whose letters
// every font of the preview draws, by base language.
var samplePhrases = map[string]string{
	"en": "This is a preview.",
	"pt": "Esta é uma prévia.",
	"es": "Esta es una vista previa.",
	"fr": "Ceci est un aperçu.",
	"de": "Das ist eine Vorschau.",
	"it": "Questa è un'anteprima.",
}

// SamplePhrase is the preview's sample line in a language, so each line of
// the preview speaks its own; a language without one gets the English.
func SamplePhrase(tag LanguageTag) string {
	if strings.EqualFold(string(tag), "pt-PT") {
		return "Isto é uma pré-visualização."
	}
	if phrase, found := samplePhrases[strings.ToLower(tag.Base())]; found {
		return phrase
	}
	return samplePhrases["en"]
}
