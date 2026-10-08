// Package ptbr words what the app shows: language names, offsets, clock
// times, the names of the subtitle lines and every other text, from the
// catalog of the interface's language (Brazilian Portuguese for now).
package ptbr

import (
	"fmt"
	"strconv"
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

// Use makes a catalog the interface's language, at once: every text is
// worded when it is shown, so the next one comes in the new language.
func Use(c *i18n.Catalog) {
	catalog.Store(c)
}

// UseLanguage makes the interface speak the language configured, else the
// system's, each when the app speaks it, else the default; it tells which.
func UseLanguage(configured, system string) (string, error) {
	c, err := i18n.Load(i18n.Choose(configured, system))
	if err != nil {
		return "", err
	}
	Use(c)
	return c.Tag(), nil
}

// Language is the interface's language, the tag of its catalog.
func Language() string {
	if c := catalog.Load(); c != nil {
		return c.Tag()
	}
	return fallbackCatalog().Tag()
}

// Languages are the interface's possible languages, each named in itself
// ("Português (Brasil)"), by tag.
func Languages() map[string]string {
	names := map[string]string{}
	for _, tag := range i18n.Available() {
		if c, err := i18n.Load(tag); err == nil {
			names[tag] = c.Text("app.languageName")
		}
	}
	return names
}

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
var languageNames = map[string]func() string{
	"en": func() string { return T("language.en") },
	"pt": func() string { return T("language.pt") },
	"es": func() string { return T("language.es") },
	"fr": func() string { return T("language.fr") },
	"de": func() string { return T("language.de") },
	"it": func() string { return T("language.it") },
	"ja": func() string { return T("language.ja") },
	"ko": func() string { return T("language.ko") },
	"zh": func() string { return T("language.zh") },
	"ru": func() string { return T("language.ru") },
	"nl": func() string { return T("language.nl") },
	"pl": func() string { return T("language.pl") },
	"tr": func() string { return T("language.tr") },
	"ar": func() string { return T("language.ar") },
}

// LanguageName names a language in lower case: "inglês", "português do
// Brasil" (the regional variants people look for); the tag itself when
// unknown, "idioma desconhecido" when empty.
func LanguageName(tag media.LanguageTag) string {
	switch tag {
	case "pt-BR":
		return T("language.ptBR")
	case "pt-PT":
		return T("language.ptPT")
	case "":
		return T("language.unknown")
	}
	if name, found := languageNames[tag.Base()]; found {
		return name()
	}
	return string(tag)
}

// Number writes a number with the interface's decimal separator, digits
// after it as strconv.FormatFloat takes them (-1 for as few as needed).
func Number(value float64, digits int) string {
	return decimal(strconv.FormatFloat(value, 'f', digits, 64))
}

// decimal puts the interface's decimal separator in a number written with a
// point.
func decimal(number string) string {
	return strings.Replace(number, ".", T("number.decimal"), 1)
}

// Offset shows an offset as "+0,150 s" or "−0,150 s".
func Offset(offset subtitle.Offset) string {
	number := decimal(fmt.Sprintf("%+.3f", offset.Duration().Seconds()))
	return T("unit.seconds", "value", strings.Replace(number, "-", "−", 1))
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

// LineNames name each line by its role and its language, as in
// "Legenda de estudo (inglês)".
type LineNames [2]string

// NewLineNames names the lines of the chosen tracks.
func NewLineNames(info media.Info, primary, translation uint64) LineNames {
	name := func(place string, number uint64) string {
		track, found := info.Track(number)
		if !found || track.Language == "" {
			return place
		}
		return T("line.withLanguage", "line", place, "language", LanguageName(track.Language))
	}
	return LineNames{name(T("line.primary"), primary), name(T("line.translation"), translation)}
}

// Notice words a sync change: "Legenda de estudo (inglês): −0,150 s".
func (n LineNames) Notice(line session.Line, offset subtitle.Offset) string {
	if !line.Valid() {
		return Offset(offset)
	}
	return T("notice.sync", "line", n[line], "offset", Offset(offset))
}
