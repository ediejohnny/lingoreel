package ptbr

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// AudioLabel names an audio track for a list, such as "English · DTS ·
// Commentary · default".
func AudioLabel(track media.Track) string {
	parts := []string{Capitalized(LanguageName(track.Language))}
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
