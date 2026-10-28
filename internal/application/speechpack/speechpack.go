// Package speechpack keeps the languages' speech packs in the app's own
// folder: it fetches the one the user asks for (ONNX Runtime with the first),
// one at a time, tells how far it got, and removes one no longer wanted.
package speechpack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

var (
	// ErrUnknown reports a language without a pack.
	ErrUnknown = errors.New("speech pack: no pack for that language")
	// ErrBusy reports a fetch asked for while another is under way.
	ErrBusy = errors.New("speech pack: another pack is being fetched")
	// ErrNotPublished reports a file its server does not hand over (not
	// published yet, or taken down).
	ErrNotPublished = errors.New("speech pack: the file is not published")
	// ErrNotAsPublished reports a file that arrived other than published (its
	// size or SHA-256), so it was not kept.
	ErrNotAsPublished = errors.New("speech pack: the file arrived other than published")
)

const (
	// folder holds the packs inside the app's folder.
	folder = "speech"
	// runtimeName and modelName are the files' names on this computer.
	runtimeName = "onnxruntime.dll"
	modelName   = "model.onnx"
)

// Fetcher fetches a published file to dest, over HTTPS, as published (its
// size and SHA-256), telling progress; nothing is left when it fails. Its
// errors wrap ErrNotPublished or ErrNotAsPublished when they are that.
type Fetcher func(ctx context.Context, url string, file speech.File, dest string, progress func(done, total int64)) error

// Options are what a Manager needs: the app's folder, where the packs are
// published (a URL ending in "/"), how a file is fetched, the small files a
// pack carries inside the app, and who is told of every change. Runtime and
// Packs are the catalogue, speech.Runtime and speech.Packs() when empty.
type Options struct {
	Dir     string
	Base    string
	Fetch   Fetcher
	Extras  func(language string) (map[string][]byte, error)
	Changed func()
	Runtime speech.File
	Packs   []speech.Pack
}

// State is a language's pack as it stands: ready to use, being fetched (with
// the bytes done of the total, the runtime's included when it comes along),
// or what stopped its last fetch.
type State struct {
	Language string
	// Size is what fetching it takes now: whichever of the model and the
	// runtime is not here.
	Size     int64
	Ready    bool
	Fetching bool
	Done     int64
	Total    int64
	Failed   error
}

// Manager keeps the packs. It is safe for concurrent use.
type Manager struct {
	options Options
	mu      sync.Mutex
	// fetching is the language under way, "" when none.
	fetching    string
	done, total int64
	cancel      context.CancelFunc
	failed      map[string]error
	running     sync.WaitGroup
}

// New starts a Manager; nothing is fetched until asked.
func New(options Options) *Manager {
	if options.Changed == nil {
		options.Changed = func() {}
	}
	if options.Runtime == (speech.File{}) {
		options.Runtime = speech.Runtime
	}
	if options.Packs == nil {
		options.Packs = speech.Packs()
	}
	return &Manager{options: options, failed: map[string]error{}}
}

func (m *Manager) packFor(language string) (speech.Pack, bool) {
	for _, pack := range m.options.Packs {
		if pack.Language == language {
			return pack, true
		}
	}
	return speech.Pack{}, false
}

// States tells every language's pack, in the catalogue's order.
func (m *Manager) States() []State {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtimeHere := m.present(m.runtimePath(), m.options.Runtime.Size)
	var states []State
	for _, pack := range m.options.Packs {
		state := State{Language: pack.Language, Failed: m.failed[pack.Language]}
		modelHere := m.ready(pack)
		if !modelHere {
			state.Size += pack.Model.Size
		}
		if !runtimeHere {
			state.Size += m.options.Runtime.Size
		}
		state.Ready = runtimeHere && modelHere
		if m.fetching == pack.Language {
			state.Fetching, state.Done, state.Total = true, m.done, m.total
		}
		states = append(states, state)
	}
	return states
}

// Paths tells where a ready pack's runtime and model are (its vocab.json and
// meta.json beside the model); ok is false while the pack is not ready.
func (m *Manager) Paths(language string) (runtime, model string, ok bool) {
	pack, known := m.packFor(language)
	if !known {
		return "", "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.present(m.runtimePath(), m.options.Runtime.Size) || !m.ready(pack) {
		return "", "", false
	}
	return m.runtimePath(), filepath.Join(m.packDir(language), modelName), true
}

// Fetch starts fetching what a language's pack lacks in the background (the
// runtime first, then the model) and returns at once; a pack already here is
// left as it is.
func (m *Manager) Fetch(language string) error {
	pack, known := m.packFor(language)
	if !known {
		return fmt.Errorf("%w: %q", ErrUnknown, language)
	}
	m.mu.Lock()
	if m.fetching != "" {
		m.mu.Unlock()
		return ErrBusy
	}
	needRuntime := !m.present(m.runtimePath(), m.options.Runtime.Size)
	needModel := !m.ready(pack)
	if !needRuntime && !needModel {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.fetching, m.cancel, m.done, m.total = language, cancel, 0, 0
	if needRuntime {
		m.total += m.options.Runtime.Size
	}
	if needModel {
		m.total += pack.Model.Size
	}
	delete(m.failed, language)
	m.running.Add(1)
	m.mu.Unlock()
	m.options.Changed()

	go func() {
		defer m.running.Done()
		err := m.fetch(ctx, pack, needRuntime, needModel)
		cancel()
		m.mu.Lock()
		m.fetching, m.cancel = "", nil
		if err != nil && !errors.Is(err, context.Canceled) {
			m.failed[language] = err
		}
		m.mu.Unlock()
		m.options.Changed()
	}()
	return nil
}

// Cancel stops the fetch under way, if any; what it fetched of the file at
// hand is removed.
func (m *Manager) Cancel() {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Remove takes a language's pack off the computer; the runtime goes too once
// no pack is left. A pack being fetched is not removed.
func (m *Manager) Remove(language string) error {
	if _, known := m.packFor(language); !known {
		return fmt.Errorf("%w: %q", ErrUnknown, language)
	}
	m.mu.Lock()
	defer func() {
		m.mu.Unlock()
		m.options.Changed()
	}()
	if m.fetching != "" {
		return ErrBusy
	}
	delete(m.failed, language)
	if err := os.RemoveAll(m.packDir(language)); err != nil {
		return err
	}
	for _, pack := range m.options.Packs {
		if _, err := os.Stat(m.packDir(pack.Language)); err == nil {
			return nil
		}
	}
	if err := os.Remove(m.runtimePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Close stops a fetch under way and waits for it to leave nothing behind.
func (m *Manager) Close() {
	m.Cancel()
	m.running.Wait()
}

func (m *Manager) fetch(ctx context.Context, pack speech.Pack, needRuntime, needModel bool) error {
	var before int64
	progress := func(done, _ int64) {
		m.mu.Lock()
		m.done = before + done
		m.mu.Unlock()
		m.options.Changed()
	}
	if needRuntime {
		if err := m.options.Fetch(ctx, m.options.Base+m.options.Runtime.Name, m.options.Runtime, m.runtimePath(), progress); err != nil {
			return err
		}
		before = m.options.Runtime.Size
	}
	if !needModel {
		return nil
	}
	extras, err := m.options.Extras(pack.Language)
	if err != nil {
		return err
	}
	dir := m.packDir(pack.Language)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// The small files go first: the model, the largest and last, makes the
	// pack ready.
	for name, data := range extras {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return m.options.Fetch(ctx, m.options.Base+pack.Model.Name, pack.Model, filepath.Join(dir, modelName), progress)
}

// ready tells a pack's model is whole and its small files are beside it.
func (m *Manager) ready(pack speech.Pack) bool {
	dir := m.packDir(pack.Language)
	if !m.present(filepath.Join(dir, modelName), pack.Model.Size) {
		return false
	}
	for _, name := range []string{"vocab.json", "meta.json"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// present tells a file is here with its exact size; its SHA-256 was checked
// when it was fetched.
func (m *Manager) present(path string, size int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

func (m *Manager) runtimePath() string { return filepath.Join(m.options.Dir, folder, runtimeName) }

func (m *Manager) packDir(language string) string {
	return filepath.Join(m.options.Dir, folder, language)
}
