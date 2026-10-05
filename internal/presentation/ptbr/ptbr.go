// Package ptbr words what the app shows: language names, offsets, clock
// times, the names of the subtitle lines and every other text, from the
// catalog of the interface's language (Brazilian Portuguese for now).
package ptbr

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/i18n"
)

// catalog is the interface's language, chosen once when the app starts
// (Use); until then, and in tests, the fallback's.
var (
	catalog         atomic.Pointer[i18n.Catalog]
	fallbackCatalog = sync.OnceValue(func() *i18n.Catalog {
		c, err := i18n.Load(i18n.Fallback)
		if err != nil {
			panic(fmt.Sprintf("the embedded catalog does not load: %v", err)) // a test loads it
		}
		return c
	})
)

// Use makes a catalog the interface's language. The app calls it once, at
// start, before any text is shown.
func Use(c *i18n.Catalog) { catalog.Store(c) }

// T is a key's text in the interface's language, with its parameters (a
// name, then its value). Keys are always written whole, never built, so a
// test can find each one in the catalog.
func T(key string, params ...any) string {
	if c := catalog.Load(); c != nil {
		return c.Text(key, params...)
	}
	return fallbackCatalog().Text(key, params...)
}

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
