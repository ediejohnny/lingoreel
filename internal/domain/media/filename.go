package media

import (
	"regexp"
	"strings"
)

// FileHints is what a subtitle file name says about its content.
type FileHints struct {
	Language        LanguageTag
	Forced          bool
	HearingImpaired bool
}

// fileNameSeparators split names such as "Movie.2019.pt-BR.forced",
// "2_English" or "Movie (Portuguese Brazil)".
var fileNameSeparators = regexp.MustCompile(`[.\s_()\[\]{},;+-]+`)

var (
	forcedWords          = map[string]bool{"forced": true, "forçada": true, "forcada": true, "forçadas": true, "forcadas": true, "foreign": true}
	hearingImpairedWords = map[string]bool{"sdh": true, "hi": true, "cc": true, "hearing": true, "impaired": true}
	// yearToken ends the title of a release name ("Ice.Age.2016.720p…"):
	// the words before it are the film's, never a language or a flag.
	yearToken = regexp.MustCompile(`^(19|20)\d\d$`)
)

// HintsFromFileName reads the language and flags from a subtitle file name,
// ignoring the part it shares with the video name (so "Quillon.2020.srt" does
// not read "quillon" or "2020") and a title that ends with a year (so neither
// "Ice", Icelandic's code, nor "Foreign" counts in "Ice.Age.2016" or
// "Foreign.Correspondent.1940"). Codes ("en", "por", "pt-BR", "pt_br") and
// language names in English or Portuguese are recognized; "hi" means hearing
// impaired, as in release names, not Hindi. Names are often wrong or missing,
// so what the file says inside decides first (SubtitleLanguage).
//
// Both are file names without their folders.
func HintsFromFileName(fileName, videoName string) FileHints {
	name := withoutExtension(fileName)
	video := withoutExtension(videoName)
	if video != "" && len(name) >= len(video) && strings.EqualFold(name[:len(video)], video) {
		name = name[len(video):]
	}
	tokens := fileNameSeparators.Split(strings.ToLower(name), -1)
	for i, token := range tokens {
		if yearToken.MatchString(token) {
			tokens = tokens[i+1:]
			break
		}
	}

	var hints FileHints
	for i, token := range tokens {
		switch {
		case token == "":
			continue
		case forcedWords[token]:
			hints.Forced = true
			continue
		case hearingImpairedWords[token]:
			hints.HearingImpaired = true
			continue
		}
		if hints.Language != "" {
			continue
		}
		if base := codeOrName(token); base != "" {
			region := ""
			if i+1 < len(tokens) {
				region = regionToken(tokens[i+1])
			}
			if region == "" {
				region = regionFromName(base, name)
			}
			hints.Language = LanguageTag(base)
			if region != "" {
				hints.Language = LanguageTag(base + "-" + region)
			}
		}
	}
	return hints
}

// codeOrName reads a token as an ISO 639 code or a language name.
func codeOrName(token string) string {
	if len(token) == 2 || len(token) == 3 {
		if base, _ := parseTag(token); base != "" && isKnownBase(base) {
			return base
		}
	}
	return languageNames[token]
}

// isKnownBase keeps two-letter tokens such as "to" or "of" from being read
// as languages: only bases the app knows by name or by ISO 639-2 count.
func isKnownBase(base string) bool {
	for _, known := range alpha3ToAlpha2 {
		if known == base {
			return true
		}
	}
	return false
}

// regionToken accepts the region part of "pt-BR" or "es-419" split into
// tokens.
func regionToken(token string) string {
	if len(token) == 2 && token != "hi" && token != "cc" {
		return strings.ToUpper(token)
	}
	if len(token) == 3 && strings.Trim(token, "0123456789") == "" {
		return token
	}
	return ""
}

// withoutExtension drops what follows the last dot of a file name.
func withoutExtension(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		return name[:dot]
	}
	return name
}
