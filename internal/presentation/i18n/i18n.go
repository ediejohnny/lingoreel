// Package i18n reads the interface's text catalogs. Each language has a
// folder under locales, named by its tag (pt-BR), whose JSON files hold its
// texts by key; the Go side and the screens read the same files. A text is
// a string with named parameters ("Volume {percent}%"), or plural forms by
// the language's categories ({"one": "{count} volta", "other": "{count}
// voltas"}), picked by the "count" parameter.
package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

//go:embed locales
var locales embed.FS

// Fallback is the language whose catalog is complete: every other one is
// checked against it, and a text missing elsewhere falls back to it.
const Fallback = "pt-BR"

// ErrNoCatalog reports a language without a catalog.
var ErrNoCatalog = errors.New("no catalog for the language")

// Entry is one text: plain, or its plural forms.
type Entry struct {
	Text  string
	Forms map[string]string
}

// UnmarshalJSON reads a string or an object of plural forms.
func (e *Entry) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &e.Text); err == nil {
		return nil
	}
	if err := json.Unmarshal(data, &e.Forms); err != nil {
		return errors.New("a text is a string or an object of plural forms")
	}
	if _, ok := e.Forms["other"]; !ok {
		return errors.New(`plural forms without "other"`)
	}
	return nil
}

// Catalog is one language's texts. It never changes once loaded and is safe
// for concurrent use.
type Catalog struct {
	tag     string
	entries map[string]Entry
	backup  *Catalog
}

// Available lists the languages with a catalog, sorted.
func Available() []string {
	dirs, err := fs.ReadDir(locales, "locales")
	if err != nil {
		return nil
	}
	var tags []string
	for _, dir := range dirs {
		if dir.IsDir() {
			tags = append(tags, dir.Name())
		}
	}
	slices.Sort(tags)
	return tags
}

// Match is the catalog for a language tag ("pt-br", "fr-CA", "en"): the
// same tag in any case, then one of the same base language, then none.
func Match(tag string) (string, bool) {
	available := Available()
	for _, candidate := range available {
		if strings.EqualFold(candidate, tag) {
			return candidate, true
		}
	}
	base, _, _ := strings.Cut(tag, "-")
	for _, candidate := range available {
		if candidateBase, _, _ := strings.Cut(candidate, "-"); base != "" && strings.EqualFold(candidateBase, base) {
			return candidate, true
		}
	}
	return "", false
}

// Load reads a language's catalog, backed by the fallback's for texts it
// lacks; a language without one gets the fallback.
func Load(tag string) (*Catalog, error) {
	matched, found := Match(tag)
	if !found {
		matched = Fallback
	}
	catalog, err := read(matched)
	if err != nil {
		return nil, err
	}
	if matched != Fallback {
		if catalog.backup, err = read(Fallback); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

// read merges the JSON files of one language's folder.
func read(tag string) (*Catalog, error) {
	dir := path.Join("locales", tag)
	files, err := fs.ReadDir(locales, dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNoCatalog, tag)
	}
	catalog := &Catalog{tag: tag, entries: map[string]Entry{}}
	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".json" {
			continue
		}
		data, err := locales.ReadFile(path.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", tag, file.Name(), err)
		}
		var entries map[string]Entry
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", tag, file.Name(), err)
		}
		for key, entry := range entries {
			if _, repeated := catalog.entries[key]; repeated {
				return nil, fmt.Errorf("%s: the key %q is in two files", tag, key)
			}
			catalog.entries[key] = entry
		}
	}
	return catalog, nil
}

// Tag is the catalog's language.
func (c *Catalog) Tag() string { return c.tag }

// Keys lists the catalog's own keys, sorted.
func (c *Catalog) Keys() []string {
	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// Entry is a key's text in this catalog only, for checking catalogs.
func (c *Catalog) Entry(key string) (Entry, bool) {
	entry, found := c.entries[key]
	return entry, found
}

// Text is a key's text with its parameters filled in; params alternate a
// name and a value. A plural text takes the form of its "count". A key no
// catalog has comes back as itself, so a missing text shows and is caught.
func (c *Catalog) Text(key string, params ...any) string {
	entry, found := c.entries[key]
	if !found && c.backup != nil {
		return c.backup.Text(key, params...)
	}
	if !found {
		return key
	}
	text := entry.Text
	if entry.Forms != nil {
		form, ok := entry.Forms[PluralCategory(c.tag, countOf(params))]
		if !ok {
			form = entry.Forms["other"]
		}
		text = form
	}
	for i := 0; i+1 < len(params); i += 2 {
		text = strings.ReplaceAll(text, "{"+fmt.Sprint(params[i])+"}", fmt.Sprint(params[i+1]))
	}
	return text
}

// countOf is the "count" parameter, 0 when absent or not a number.
func countOf(params []any) float64 {
	for i := 0; i+1 < len(params); i += 2 {
		if params[i] != "count" {
			continue
		}
		switch n := params[i+1].(type) {
		case int:
			return float64(n)
		case int64:
			return float64(n)
		case float64:
			return n
		}
	}
	return 0
}

// PluralCategory is the CLDR cardinal category of a whole number in a
// language: "one" or "other" (the languages with more forms get theirs when
// their catalog comes).
func PluralCategory(tag string, n float64) string {
	base, region, _ := strings.Cut(tag, "-")
	switch strings.ToLower(base) {
	case "ja", "ko", "zh":
		return "other"
	case "fr":
		if n >= 0 && n < 2 {
			return "one"
		}
		return "other"
	case "pt":
		// Brazilian Portuguese says "0 volta"; Portugal's "0 voltas".
		if n == 1 || (!strings.EqualFold(region, "PT") && n == 0) {
			return "one"
		}
		return "other"
	}
	if n == 1 {
		return "one"
	}
	return "other"
}
