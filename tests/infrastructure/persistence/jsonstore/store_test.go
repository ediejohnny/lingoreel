package jsonstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lingoreel/internal/infrastructure/persistence/jsonstore"
)

type sample struct {
	Name  string            `json:"name"`
	Count int               `json:"count"`
	Tags  []string          `json:"tags"`
	Extra map[string]string `json:"extra"`
}

func defaultSample() sample {
	return sample{Name: "default", Count: 7, Tags: []string{"a"}, Extra: map[string]string{}}
}

func newSampleStore(t *testing.T, version int, migrations map[int]jsonstore.Migration) (*jsonstore.Store[sample], string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	return jsonstore.New(path, version, defaultSample, migrations), path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingDocumentReturnsDefaultsWithoutCreatingIt(t *testing.T) {
	store, path := newSampleStore(t, 1, nil)

	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, defaultSample()) {
		t.Fatalf("Load() = %+v, %v; want defaults", got, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load created the document: %v", err)
	}
	if store.Path() != path {
		t.Fatalf("Path() = %q, want %q", store.Path(), path)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	store, path := newSampleStore(t, 3, nil)
	want := sample{Name: "Inglês", Count: 0, Tags: []string{}, Extra: map[string]string{"k": "v"}}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %+v, %v; want %+v", got, err, want)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), `"version": 3`) || !strings.Contains(string(content), "Inglês") {
		t.Fatalf("saved document lacks version or text: %s", content)
	}
}

func TestSaveOverwritesAndLeavesNoTemporaryFiles(t *testing.T) {
	store, path := newSampleStore(t, 1, nil)

	for i := range 3 {
		if err := store.Save(sample{Name: "run", Count: i}); err != nil {
			t.Fatalf("Save #%d: %v", i, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory holds %v, want only config.json", names)
	}
	if got, _ := store.Load(); got.Count != 2 {
		t.Fatalf("Load().Count = %d, want the last saved value 2", got.Count)
	}
}

func TestLoadKeepsDefaultsForMissingFieldsAndIgnoresUnknownOnes(t *testing.T) {
	store, path := newSampleStore(t, 1, nil)
	writeFile(t, path, `{"version": 1, "name": "partial", "futureField": true}`)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := defaultSample()
	want.Name = "partial"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadRunsMigrationsInOrder(t *testing.T) {
	var calls []string
	migrations := map[int]jsonstore.Migration{
		1: func(document map[string]any) error {
			calls = append(calls, "1->2")
			document["name"] = document["title"]
			delete(document, "title")
			return nil
		},
		2: func(document map[string]any) error {
			calls = append(calls, "2->3")
			document["tags"] = []any{"migrated", document["name"]}
			return nil
		},
	}
	store, path := newSampleStore(t, 3, migrations)
	writeFile(t, path, `{"version": 1, "title": "old name", "count": 4}`)

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"1->2", "2->3"}) {
		t.Fatalf("migrations ran as %v", calls)
	}
	if got.Name != "old name" || got.Count != 4 || !reflect.DeepEqual(got.Tags, []string{"migrated", "old name"}) {
		t.Fatalf("migrated value = %+v", got)
	}

	calls = nil
	writeFile(t, path, `{"version": 2, "name": "middle"}`)
	if _, err := store.Load(); err != nil || !reflect.DeepEqual(calls, []string{"2->3"}) {
		t.Fatalf("from version 2 ran %v, %v; want only 2->3", calls, err)
	}

	calls = nil
	writeFile(t, path, `{"version": 3, "name": "current"}`)
	if _, err := store.Load(); err != nil || len(calls) != 0 {
		t.Fatalf("current version ran migrations %v, %v", calls, err)
	}
}

func TestLoadRejectsBadDocuments(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    error
	}{
		{"broken JSON", `{"version": 1, "name": `, jsonstore.ErrCorrupt},
		{"array instead of object", `[1, 2]`, jsonstore.ErrCorrupt},
		{"null document", `null`, jsonstore.ErrCorrupt},
		{"missing version", `{"name": "x"}`, jsonstore.ErrCorrupt},
		{"string version", `{"version": "1"}`, jsonstore.ErrCorrupt},
		{"fractional version", `{"version": 1.5}`, jsonstore.ErrCorrupt},
		{"zero version", `{"version": 0}`, jsonstore.ErrCorrupt},
		{"negative version", `{"version": -2}`, jsonstore.ErrCorrupt},
		{"trailing data", `{"version": 1} {"version": 1}`, jsonstore.ErrCorrupt},
		{"wrong field type", `{"version": 1, "count": "seven"}`, jsonstore.ErrCorrupt},
		{"newer version", `{"version": 3, "name": "future"}`, jsonstore.ErrNewerVersion},
		{"empty file", ``, jsonstore.ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, path := newSampleStore(t, 2, map[int]jsonstore.Migration{1: func(map[string]any) error { return nil }})
			writeFile(t, path, tc.content)

			got, err := store.Load()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Load() error = %v, want %v", err, tc.want)
			}
			if !reflect.DeepEqual(got, defaultSample()) {
				t.Fatalf("Load() returned %+v with an error, want defaults", got)
			}
		})
	}
}

func TestLoadRejectsOversizedDocumentButAcceptsTheLimit(t *testing.T) {
	store, path := newSampleStore(t, 1, nil)
	prefix, suffix := `{"version": 1, "name": "`, `"}`

	writeFile(t, path, prefix+strings.Repeat("x", jsonstore.MaxDocumentBytes-len(prefix)-len(suffix))+suffix)
	if _, err := store.Load(); err != nil {
		t.Fatalf("document of exactly the limit rejected: %v", err)
	}
	writeFile(t, path, prefix+strings.Repeat("x", jsonstore.MaxDocumentBytes)+suffix)
	if _, err := store.Load(); !errors.Is(err, jsonstore.ErrCorrupt) {
		t.Fatalf("oversized document error = %v, want ErrCorrupt", err)
	}
}

func TestLoadReportsMissingAndFailingMigrations(t *testing.T) {
	store, path := newSampleStore(t, 3, map[int]jsonstore.Migration{1: func(map[string]any) error { return nil }})
	writeFile(t, path, `{"version": 1}`)
	if _, err := store.Load(); !errors.Is(err, jsonstore.ErrMissingMigration) {
		t.Fatalf("missing migration error = %v, want ErrMissingMigration", err)
	}

	failure := errors.New("cannot convert")
	failing, failingPath := newSampleStore(t, 2, map[int]jsonstore.Migration{1: func(map[string]any) error { return failure }})
	writeFile(t, failingPath, `{"version": 1}`)
	if _, err := failing.Load(); !errors.Is(err, failure) {
		t.Fatalf("failing migration error = %v, want the migration's error", err)
	}
}

func TestSaveRejectsValuesThatClashWithTheVersionField(t *testing.T) {
	type clashing struct {
		Version int `json:"version"`
	}
	path := filepath.Join(t.TempDir(), "clash.json")
	store := jsonstore.New(path, 1, func() clashing { return clashing{} }, nil)

	if err := store.Save(clashing{Version: 9}); !errors.Is(err, jsonstore.ErrReservedField) {
		t.Fatalf("Save error = %v, want ErrReservedField", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a rejected Save still wrote the document")
	}
}

func TestSaveRejectsNonObjectValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.json")
	store := jsonstore.New(path, 1, func() []int { return nil }, nil)

	if err := store.Save([]int{1, 2}); err == nil {
		t.Fatal("Save accepted a JSON array")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a rejected Save still wrote the document")
	}
}
