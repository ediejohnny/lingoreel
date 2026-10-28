package speechpack_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/download"
	fetching "github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// The catalogue's files are small stand-ins served over HTTPS; the fetch is
// the app's own (download.Client), so a pack is fetched as in the app.
var (
	runtimeBytes = bytes.Repeat([]byte("runtime "), 4096)
	englishBytes = bytes.Repeat([]byte("english model "), 8192)
	frenchBytes  = bytes.Repeat([]byte("french model "), 8192)
)

func published(name string, data []byte) speech.File {
	sum := sha256.Sum256(data)
	return speech.File{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// shelf serves the catalogue's files. A file can be swapped for other bytes,
// and the French model can be held until released.
type shelf struct {
	server   *httptest.Server
	mu       sync.Mutex
	files    map[string][]byte
	requests map[string]int
	hold     chan struct{}
}

func newShelf(t *testing.T) *shelf {
	t.Helper()
	s := &shelf{requests: map[string]int{}, files: map[string][]byte{
		"ort.dll": runtimeBytes, "speech-en.onnx": englishBytes, "speech-fr.onnx": frenchBytes,
	}}
	s.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		s.mu.Lock()
		data, hold := s.files[name], s.hold
		s.requests[name]++
		s.mu.Unlock()
		if data == nil {
			http.NotFound(w, r)
			return
		}
		if name == "speech-fr.onnx" && hold != nil {
			// Half the file goes, then the rest waits for the test.
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			_, _ = w.Write(data[:len(data)/2])
			w.(http.Flusher).Flush()
			select {
			case <-hold:
			case <-r.Context().Done():
				return
			}
			_, _ = w.Write(data[len(data)/2:])
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(s.server.Close)
	return s
}

// asked tells how many times a file was asked for.
func (s *shelf) asked(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[name]
}

func (s *shelf) manager(t *testing.T, dir string, changed func()) *speechpack.Manager {
	t.Helper()
	m := speechpack.New(speechpack.Options{
		Dir:   dir,
		Disk:  fetching.Disk{},
		Base:  s.server.URL + "/release/",
		Fetch: fetching.Fetcher(download.Client{HTTP: s.server.Client()}),
		Extras: func(language string) (map[string][]byte, error) {
			return map[string][]byte{"vocab.json": []byte(`{"a":1}`), "meta.json": []byte(`{"model":"` + language + `"}`)}, nil
		},
		Changed: func() {
			if changed != nil {
				changed()
			}
		},
		Runtime: published("ort.dll", runtimeBytes),
		Packs: []speech.Pack{
			{Language: "en", Model: published("speech-en.onnx", englishBytes)},
			{Language: "fr", Model: published("speech-fr.onnx", frenchBytes)},
		},
	})
	t.Cleanup(m.Close)
	return m
}

func stateOf(m *speechpack.Manager, language string) speechpack.State {
	for _, s := range m.States() {
		if s.Language == language {
			return s
		}
	}
	return speechpack.State{}
}

func settled(t *testing.T, m *speechpack.Manager, language string) speechpack.State {
	t.Helper()
	testkit.Eventually(t, language+"'s fetch to end", func() bool { return !stateOf(m, language).Fetching })
	return stateOf(m, language)
}

func TestAPackComesWithTheRuntimeOnceAndIsReadyToUse(t *testing.T) {
	t.Parallel()
	shelf, dir := newShelf(t), t.TempDir()
	var changes atomic.Int32
	m := shelf.manager(t, dir, func() { changes.Add(1) })

	before := stateOf(m, "en")
	if before.Ready || before.Size != int64(len(englishBytes)+len(runtimeBytes)) {
		t.Fatalf("before any fetch: %+v, want not ready and the runtime counted in its size", before)
	}
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	english := settled(t, m, "en")
	if !english.Ready || english.Failed != nil {
		t.Fatalf("English after its fetch: %+v", english)
	}
	if changes.Load() < 2 {
		t.Errorf("the screen was told %d times; the start and the end at least", changes.Load())
	}
	runtime, model, ok := m.Paths("en")
	if !ok {
		t.Fatal("a ready pack has no paths")
	}
	if data, _ := os.ReadFile(runtime); !bytes.Equal(data, runtimeBytes) {
		t.Error("the runtime on disk is not the one published")
	}
	if data, _ := os.ReadFile(model); !bytes.Equal(data, englishBytes) {
		t.Error("the model on disk is not the one published")
	}
	if data, _ := os.ReadFile(filepath.Join(filepath.Dir(model), "meta.json")); string(data) != `{"model":"en"}` {
		t.Errorf("meta.json = %q, want the one the app carries", data)
	}

	// French no longer counts the runtime, and does not fetch it again.
	if french := stateOf(m, "fr"); french.Ready || french.Size != int64(len(frenchBytes)) {
		t.Fatalf("French once the runtime is here: %+v", french)
	}
	if err := m.Fetch("fr"); err != nil {
		t.Fatal(err)
	}
	if french := settled(t, m, "fr"); !french.Ready {
		t.Fatalf("French after its fetch: %+v", french)
	}
	if n := shelf.asked("ort.dll"); n != 1 {
		t.Errorf("the runtime was fetched %d times, want once", n)
	}
	if _, _, ok := m.Paths("de"); ok {
		t.Error("a language without a pack has paths")
	}
	if err := m.Fetch("de"); !errors.Is(err, speechpack.ErrUnknown) {
		t.Errorf("fetching a language without a pack: %v, want ErrUnknown", err)
	}
}

func TestOnlyWhatAPackLacksIsFetchedAgain(t *testing.T) {
	t.Parallel()
	shelf, dir := newShelf(t), t.TempDir()
	// totals are the totals the screens were told while English was fetched.
	var mu sync.Mutex
	var totals []int64
	var m *speechpack.Manager
	m = shelf.manager(t, dir, func() {
		if s := stateOf(m, "en"); s.Fetching {
			mu.Lock()
			totals = append(totals, s.Total)
			mu.Unlock()
		}
	})
	// told takes the totals told since the last call, which must all be want.
	told := func(what string, want int64) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(totals) == 0 {
			t.Errorf("%s: the screens were told no total", what)
		}
		for _, total := range totals {
			if total != want {
				t.Errorf("%s: the screens were told a total of %d bytes, want %d", what, total, want)
				break
			}
		}
		totals = nil
	}
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if s := settled(t, m, "en"); !s.Ready {
		t.Fatalf("English after its fetch: %+v", s)
	}
	told("the first fetch", int64(len(runtimeBytes)+len(englishBytes)))

	// A ready pack asked for again fetches nothing.
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if s := stateOf(m, "en"); s.Fetching || shelf.asked("speech-en.onnx") != 1 || shelf.asked("ort.dll") != 1 {
		t.Fatalf("asking for a ready pack: %+v, the model asked %d times, the runtime %d", s, shelf.asked("speech-en.onnx"), shelf.asked("ort.dll"))
	}

	// Without the runtime the pack is not ready, and only the runtime comes back.
	runtime := filepath.Join(dir, "speech", "onnxruntime.dll")
	if err := os.Remove(runtime); err != nil {
		t.Fatal(err)
	}
	if s := stateOf(m, "en"); s.Ready || s.Size != int64(len(runtimeBytes)) {
		t.Fatalf("the runtime gone: %+v, want not ready, the runtime's size to fetch", s)
	}
	if _, _, ok := m.Paths("en"); ok {
		t.Error("a pack without its runtime has paths")
	}
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if s := settled(t, m, "en"); !s.Ready || shelf.asked("ort.dll") != 2 || shelf.asked("speech-en.onnx") != 1 {
		t.Fatalf("after fetching the runtime alone: %+v, the runtime asked %d times, the model %d", s, shelf.asked("ort.dll"), shelf.asked("speech-en.onnx"))
	}
	told("the runtime alone", int64(len(runtimeBytes)))

	// A model cut short on the disk is not ready, and only the model comes back.
	model := filepath.Join(dir, "speech", "en", "model.onnx")
	if err := os.WriteFile(model, englishBytes[:len(englishBytes)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if s := stateOf(m, "en"); s.Ready || s.Size != int64(len(englishBytes)) {
		t.Fatalf("the model cut short: %+v, want not ready, the model's size to fetch", s)
	}
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if s := settled(t, m, "en"); !s.Ready || shelf.asked("speech-en.onnx") != 2 || shelf.asked("ort.dll") != 2 {
		t.Fatalf("after fetching the model alone: %+v", s)
	}
	told("the model alone", int64(len(englishBytes)))

	// Without its small files the pack is not ready; they come back from the
	// app, and nothing is fetched.
	meta := filepath.Join(dir, "speech", "en", "meta.json")
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if s := stateOf(m, "en"); s.Ready || s.Size != 0 {
		t.Fatalf("the meta gone: %+v, want not ready with nothing to fetch", s)
	}
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if s := settled(t, m, "en"); !s.Ready || shelf.asked("speech-en.onnx") != 2 || shelf.asked("ort.dll") != 2 {
		t.Fatalf("after writing the small files again: %+v, the model asked %d times", s, shelf.asked("speech-en.onnx"))
	}
	if _, err := os.Stat(meta); err != nil {
		t.Errorf("meta.json did not come back: %v", err)
	}
	told("the small files alone", 0)
}

func TestAFileOtherThanPublishedLeavesThePackNotReady(t *testing.T) {
	t.Parallel()
	shelf, dir := newShelf(t), t.TempDir()
	shelf.mu.Lock()
	shelf.files["speech-en.onnx"] = bytes.ToUpper(englishBytes) // same size, other bytes
	shelf.mu.Unlock()
	m := shelf.manager(t, dir, nil)

	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	english := settled(t, m, "en")
	if english.Ready || !errors.Is(english.Failed, speechpack.ErrNotAsPublished) {
		t.Fatalf("after a damaged model: %+v, want not ready, failed as not as published", english)
	}
	if _, _, ok := m.Paths("en"); ok {
		t.Error("a pack whose model failed has paths")
	}
	if _, err := os.Stat(filepath.Join(dir, "speech", "en", "model.onnx")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the damaged model was left as the pack's: %v", err)
	}

	// Asking again, once the file is right, clears the failure.
	shelf.mu.Lock()
	shelf.files["speech-en.onnx"] = englishBytes
	shelf.mu.Unlock()
	if err := m.Fetch("en"); err != nil {
		t.Fatal(err)
	}
	if english := settled(t, m, "en"); !english.Ready || english.Failed != nil {
		t.Fatalf("after fetching again: %+v", english)
	}

	// A file the server does not hand over is told apart: not published yet.
	shelf.mu.Lock()
	delete(shelf.files, "speech-fr.onnx")
	shelf.mu.Unlock()
	if err := m.Fetch("fr"); err != nil {
		t.Fatal(err)
	}
	if french := settled(t, m, "fr"); french.Ready || !errors.Is(french.Failed, speechpack.ErrNotPublished) {
		t.Fatalf("French not on the server: %+v, want failed as not published", french)
	}
}

func TestOneFetchAtATimeAndACancelledOneLeavesNothing(t *testing.T) {
	t.Parallel()
	shelf, dir := newShelf(t), t.TempDir()
	shelf.hold = make(chan struct{})
	m := shelf.manager(t, dir, nil)

	if err := m.Fetch("fr"); err != nil {
		t.Fatal(err)
	}
	testkit.Eventually(t, "half of French to arrive", func() bool {
		s := stateOf(m, "fr")
		return s.Fetching && s.Done > int64(len(runtimeBytes))
	})
	if s := stateOf(m, "fr"); s.Total != int64(len(runtimeBytes)+len(frenchBytes)) || s.Done >= s.Total {
		t.Fatalf("under way: %+v, want part of the runtime and the model together", s)
	}
	if err := m.Fetch("en"); !errors.Is(err, speechpack.ErrBusy) {
		t.Errorf("a second fetch while one is under way: %v, want ErrBusy", err)
	}
	if err := m.Remove("en"); !errors.Is(err, speechpack.ErrBusy) {
		t.Errorf("removing while a fetch is under way: %v, want ErrBusy", err)
	}

	m.Cancel()
	french := settled(t, m, "fr")
	if french.Ready || french.Failed != nil {
		t.Fatalf("after a cancel: %+v, want not ready and no failure told", french)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "speech", "fr"))
	for _, entry := range entries {
		if entry.Name() != "vocab.json" && entry.Name() != "meta.json" {
			t.Errorf("a cancelled fetch left %s", entry.Name())
		}
	}
}

func TestRemovingTheLastPackTakesTheRuntimeToo(t *testing.T) {
	t.Parallel()
	shelf, dir := newShelf(t), t.TempDir()
	m := shelf.manager(t, dir, nil)
	for _, language := range []string{"en", "fr"} {
		if err := m.Fetch(language); err != nil {
			t.Fatal(err)
		}
		if s := settled(t, m, language); !s.Ready {
			t.Fatalf("%s after its fetch: %+v", language, s)
		}
	}
	runtime := filepath.Join(dir, "speech", "onnxruntime.dll")

	if err := m.Remove("en"); err != nil {
		t.Fatal(err)
	}
	if stateOf(m, "en").Ready || !stateOf(m, "fr").Ready {
		t.Fatalf("after removing English: en %+v, fr %+v", stateOf(m, "en"), stateOf(m, "fr"))
	}
	if _, err := os.Stat(runtime); err != nil {
		t.Fatalf("the runtime went while French still needs it: %v", err)
	}
	if err := m.Remove("fr"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(runtime); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the runtime stayed with no pack left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "speech", "fr")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("French's folder stayed: %v", err)
	}
}
