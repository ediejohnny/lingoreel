package i18n_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/presentation/i18n"
)

// root is the repository, from this test's folder.
var root = filepath.Join("..", "..", "..")

// params are the {names} a text uses, in every plural form.
func params(entry i18n.Entry) []string {
	pattern := regexp.MustCompile(`\{([a-zA-Z]+)\}`)
	seen := map[string]bool{}
	texts := []string{entry.Text}
	for _, form := range entry.Forms {
		texts = append(texts, form)
	}
	for _, text := range texts {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			seen[match[1]] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Every catalog has the fallback's keys, each text with the same
// parameters, and nothing more.
func TestEveryCatalogMatchesTheFallback(t *testing.T) {
	t.Parallel()
	fallback, err := i18n.Load(i18n.Fallback)
	if err != nil {
		t.Fatal(err)
	}
	if len(fallback.Keys()) == 0 {
		t.Fatal("the fallback catalog is empty")
	}
	for _, tag := range i18n.Available() {
		catalog, err := i18n.Load(tag)
		if err != nil {
			t.Errorf("%s: %v", tag, err)
			continue
		}
		if !slices.Equal(catalog.Keys(), fallback.Keys()) {
			t.Errorf("%s has other keys than %s", tag, i18n.Fallback)
		}
		for _, key := range catalog.Keys() {
			entry, _ := catalog.Entry(key)
			want, _ := fallback.Entry(key)
			if !slices.Equal(params(entry), params(want)) {
				t.Errorf("%s %q uses %v, the fallback %v", tag, key, params(entry), params(want))
			}
			if (entry.Forms == nil) != (want.Forms == nil) {
				t.Errorf("%s %q: plural in one catalog only", tag, key)
			}
		}
	}
}

// usedKeys are the catalog keys written in source files: T("…") in Go, t("…")
// in the screens.
func usedKeys(t *testing.T, dir string, extensions []string, call *regexp.Regexp) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "wailsjs") {
			return filepath.SkipDir
		}
		if d.IsDir() || !slices.Contains(extensions, filepath.Ext(path)) || strings.Contains(path, ".test.") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range call.FindAllStringSubmatch(string(data), -1) {
			keys[match[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// Each key the code uses is in the catalog, and each key of the catalog is
// used: the Go side owns the keys outside "ui.", the screens those in it.
func TestTheCatalogHoldsExactlyTheKeysInUse(t *testing.T) {
	t.Parallel()
	fallback, err := i18n.Load(i18n.Fallback)
	if err != nil {
		t.Fatal(err)
	}
	inGo := usedKeys(t, "internal", []string{".go"}, regexp.MustCompile(`\bT\("([a-zA-Z0-9_.]+)"`))
	inScreens := usedKeys(t, filepath.Join("frontend", "src"), []string{".ts", ".tsx"}, regexp.MustCompile(`\bt\("([a-zA-Z0-9_.]+)"`))
	for key := range inGo {
		if _, found := fallback.Entry(key); !found {
			t.Errorf("the Go side uses %q, which the catalog lacks", key)
		}
	}
	for key := range inScreens {
		if _, found := fallback.Entry(key); !found {
			t.Errorf("the screens use %q, which the catalog lacks", key)
		}
	}
	for _, key := range fallback.Keys() {
		screens := strings.HasPrefix(key, "ui.")
		if (screens && !inScreens[key]) || (!screens && !inGo[key]) {
			t.Errorf("the catalog has %q, which nothing uses", key)
		}
	}
}

// A language is matched by its tag in any case, then by its base language;
// any other gets the fallback, and a key no catalog has shows as itself.
func TestTheLanguageAndItsTexts(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"pt-BR", "PT-br", "pt", "pt-PT"} {
		if got, found := i18n.Match(tag); !found || got != "pt-BR" {
			t.Errorf("Match(%q) = %q, %v", tag, got, found)
		}
	}
	if _, found := i18n.Match("xx"); found {
		t.Error("an unknown language matched a catalog")
	}
	catalog, err := i18n.Load("xx-YY")
	if err != nil || catalog.Tag() != i18n.Fallback {
		t.Fatalf("Load(xx-YY) = %v, %v; want the fallback", catalog, err)
	}
	if got := catalog.Text("no.such.key"); got != "no.such.key" {
		t.Errorf("a missing key = %q", got)
	}
	if got := catalog.Text("app.windowTitle", "video", "Filme.mkv"); got != "Filme.mkv — LingoReel" {
		t.Errorf("parameters = %q", got)
	}
}

// Plural categories of whole numbers, as CLDR has them.
func TestPluralCategories(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tag  string
		n    float64
		want string
	}{
		{"pt-BR", 0, "one"}, {"pt-BR", 1, "one"}, {"pt-BR", 2, "other"},
		{"pt-PT", 0, "other"}, {"pt-PT", 1, "one"},
		{"en", 0, "other"}, {"en", 1, "one"}, {"en", 2, "other"},
		{"fr", 0, "one"}, {"fr", 1, "one"}, {"fr", 2, "other"},
		{"ja", 1, "other"},
	} {
		if got := i18n.PluralCategory(c.tag, c.n); got != c.want {
			t.Errorf("PluralCategory(%s, %v) = %s, want %s", c.tag, c.n, got, c.want)
		}
	}
}