// Package ptbr words what the app shows in Brazilian Portuguese: language
// names, offsets, clock times and the names of the subtitle lines.
package ptbr

import (
	"fmt"
	"strings"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// languageNames are the languages named in the interface, by base tag.
var languageNames = map[string]string{
	"en": "inglês", "pt": "português", "es": "espanhol", "fr": "francês",
	"de": "alemão", "it": "italiano", "ja": "japonês", "ko": "coreano", "zh": "chinês",
	"ru": "russo", "nl": "holandês", "pl": "polonês", "tr": "turco", "ar": "árabe",
}

// regionNames tell the regional variants people look for.
var regionNames = map[media.LanguageTag]string{"pt-BR": "português do Brasil", "pt-PT": "português de Portugal"}

// LanguageName names a language in lower case: "inglês", "português do
// Brasil"; the tag itself when unknown, "idioma desconhecido" when empty.
func LanguageName(tag media.LanguageTag) string {
	if name, found := regionNames[tag]; found {
		return name
	}
	if name, found := languageNames[tag.Base()]; found {
		return name
	}
	if tag == "" {
		return "idioma desconhecido"
	}
	return string(tag)
}

// Offset shows an offset as "+0,150 s" or "−0,150 s".
func Offset(offset subtitle.Offset) string {
	text := strings.Replace(fmt.Sprintf("%+.3f s", offset.Duration().Seconds()), ".", ",", 1)
	return strings.Replace(text, "-", "−", 1)
}

// Clock shows a duration as h:mm:ss, or m:ss under an hour.
func Clock(d time.Duration) string {
	d = max(d, 0).Truncate(time.Second)
	hours, minutes, seconds := d/time.Hour, (d%time.Hour)/time.Minute, (d%time.Minute)/time.Second
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// LineNames name each line by its place on screen and its language, as in
// "Legenda de cima (inglês)".
type LineNames [2]string

// NewLineNames names the lines of the chosen tracks.
func NewLineNames(info media.Info, primary, translation uint64) LineNames {
	name := func(place string, number uint64) string {
		track, found := info.Track(number)
		if !found || track.Language == "" {
			return place
		}
		return fmt.Sprintf("%s (%s)", place, LanguageName(track.Language))
	}
	return LineNames{name("Legenda de cima", primary), name("Legenda de baixo", translation)}
}

// Notice words a sync change: "Legenda de cima (inglês): −0,150 s".
func (n LineNames) Notice(line session.Line, offset subtitle.Offset) string {
	if line != session.LinePrimary && line != session.LineTranslation {
		return Offset(offset)
	}
	return fmt.Sprintf("%s: %s", n[line], Offset(offset))
}
