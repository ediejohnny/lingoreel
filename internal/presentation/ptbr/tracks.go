package ptbr

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// AudioLabel names an audio track for a list, such as "English · DTS ·
// Commentary · default". A track whose file does not tell its language says
// so: with none, or with only the format's default (English, guessed), the
// language is not the file's word.
func AudioLabel(track media.Track) string {
	language := Capitalized(LanguageName(track.Language))
	switch {
	case track.LanguageHeard:
		language = T("track.audio.heard", "language", language)
	case track.Language == "" || track.LanguageGuessed:
		language = T("track.audio.untold")
	}
	parts := []string{language}
	if track.Codec != "" {
		parts = append(parts, strings.TrimPrefix(track.Codec, "A_"))
	}
	if track.Name != "" {
		parts = append(parts, track.Name)
	}
	if track.Default {
		parts = append(parts, T("track.default"))
	}
	return strings.Join(parts, " · ")
}

// SubtitleLabel names a subtitle track for a list, such as "English — Full
// (file)".
func SubtitleLabel(track media.Track) string {
	label := Capitalized(LanguageName(track.Language))
	if track.Name != "" {
		label += " — " + track.Name
	}
	var flags []string
	if track.Forced {
		flags = append(flags, T("track.forced"))
	}
	if track.HearingImpaired {
		flags = append(flags, T("track.hearingImpaired"))
	}
	if track.External() {
		flags = append(flags, T("track.external"))
	}
	if len(flags) > 0 {
		label += " (" + strings.Join(flags, ", ") + ")"
	}
	return label
}

// Capitalized starts a text with a capital letter, as a language's name at
// the start of a label.
func Capitalized(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return text
	}
	return string(unicode.ToUpper(first)) + text[size:]
}
