package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/application/players"
	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/app"
)

// --- media ---

type fakeReader struct{}

func (fakeReader) Analyze(string) (media.Info, error) {
	return media.Info{Duration: 90 * time.Minute, Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo, Width: 1920, Height: 1080, FrameRate: 24},
		{Number: 2, Type: media.TrackAudio, Language: "pt", Codec: "A_AC3", Default: true},
		{Number: 3, Type: media.TrackAudio, Language: "en", Codec: "A_DTS"},
		{Number: 4, Type: media.TrackSubtitle, Language: "en", Name: "Full", Readable: true},
		{Number: 5, Type: media.TrackSubtitle, Language: "pt-BR", Readable: true},
		{Number: 6, Type: media.TrackSubtitle, Language: "pt", Codec: "S_HDMV/PGS"},
	}}, nil
}
func (fakeReader) Check(string) error { return nil }
func (fakeReader) ReadSubtitles(context.Context, string, []uint64, func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	line := subtitle.Event{ID: "1", Start: time.Second, End: 2 * time.Second, OriginalText: "Hi."}
	return map[uint64]media.SubtitleContent{4: {Events: []subtitle.Event{line}}, 5: {Events: []subtitle.Event{line}}}, nil
}

// --- player ---

type fakeRunning struct {
	samples chan playback.Sample
	once    sync.Once
	paused  int
}

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{Pause: true, Seek: true, Speed: true, Precision: playback.PrecisionExact}
}
func (r *fakeRunning) Play() error                     { return nil }
func (r *fakeRunning) Pause() error                    { r.paused++; return nil }
func (r *fakeRunning) SeekTo(time.Duration) error      { return nil }
func (r *fakeRunning) SetSpeed(playback.Speed) error   { return nil }
func (r *fakeRunning) StepFrame(bool) error            { return errors.New("no") }
func (r *fakeRunning) SelectAudioTrack(int) error      { return nil }
func (r *fakeRunning) Samples() <-chan playback.Sample { return r.samples }
func (r *fakeRunning) ProcessID() int                  { return 1 }
func (r *fakeRunning) Err() error                      { return nil }
func (r *fakeRunning) Close() error {
	r.once.Do(func() { close(r.samples) })
	return nil
}

type fakePlugin struct {
	mu       sync.Mutex
	launched []player.LaunchRequest
	running  *fakeRunning
}

func (p *fakePlugin) Descriptor() player.Descriptor {
	return player.Descriptor{ID: "fake", Name: "Fake Player", Install: player.Install{
		WingetID: "Fake.Player", DownloadURL: "https://example.org/fake", Benefit: "Um player de teste.",
	}}
}
func (p *fakePlugin) Launch(_ context.Context, _ string, request player.LaunchRequest) (players.Running, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.launched = append(p.launched, request)
	p.running = &fakeRunning{samples: make(chan playback.Sample, 1)}
	p.running.samples <- playback.Sample{Position: 30 * time.Second, Duration: 90 * time.Minute, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()}
	return p.running, nil
}
func (p *fakePlugin) current() *fakeRunning {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// --- window, settings, installer ---

type event struct {
	name string
	data any
}

type fakeUI struct {
	mu     sync.Mutex
	events []event
	urls   []string
	choose string
}

func (u *fakeUI) Emit(name string, data any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.events = append(u.events, event{name, data})
}
func (u *fakeUI) OpenVideoDialog() (string, error) { return u.choose, nil }
func (u *fakeUI) OpenURL(url string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.urls = append(u.urls, url)
}

// last returns the latest event of a name.
func (u *fakeUI) last(name string) (any, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for i := len(u.events) - 1; i >= 0; i-- {
		if u.events[i].name == name {
			return u.events[i].data, true
		}
	}
	return nil, false
}

type memorySettings struct {
	mu        sync.Mutex
	config    settings.Config
	profiles  settings.Profiles
	loadErr   error
	saves     int
	saveError error
}

func newMemorySettings() *memorySettings {
	return &memorySettings{config: settings.DefaultConfig(), profiles: settings.DefaultProfiles()}
}
func (m *memorySettings) LoadConfig() (settings.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config, m.loadErr
}
func (m *memorySettings) SaveConfig(c settings.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	if m.saveError != nil {
		return m.saveError
	}
	m.config = c
	return nil
}
func (m *memorySettings) LoadProfiles() (settings.Profiles, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.profiles, m.loadErr
}
func (m *memorySettings) SaveProfiles(p settings.Profiles) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	m.profiles = p
	return nil
}

type fakeInstaller struct {
	mu        sync.Mutex
	available bool
	installed []string
	err       error
}

func (f *fakeInstaller) Available() bool { return f.available }
func (f *fakeInstaller) Install(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.installed = append(f.installed, id)
	return f.err
}

// --- harness ---

type rig struct {
	api       *app.API
	lifecycle app.Lifecycle
	ui        *fakeUI
	plugin    *fakePlugin
	settings  *memorySettings
	installer *fakeInstaller
	installed []player.Installation
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		ui: &fakeUI{}, plugin: &fakePlugin{}, settings: newMemorySettings(),
		installer: &fakeInstaller{available: true},
		installed: []player.Installation{{ID: "fake", Path: `C:\fake\fake.exe`}, {ID: "potplayer", Path: `C:\pot\PotPlayerMini64.exe`}},
	}
	registry, err := players.NewRegistry(r.plugin)
	if err != nil {
		t.Fatal(err)
	}
	service := analysis.NewService(fakeReader{}, nil)
	r.api, r.lifecycle = app.New(app.Deps{
		Service: service, Registry: registry,
		Detect: func(map[player.ID]string) []player.Installation { return r.installed },
		Ports: watch.Ports{Service: service, Registry: registry,
			OpenDisplay: func(context.Context, players.Running, media.Info) (session.Display, error) { return nopDisplay{}, nil },
		},
		Settings: r.settings, Installer: r.installer,
		Window:       func(media.Info) player.Placement { return player.Placement{Width: 800, Height: 450} },
		InitialVideo: `D:\Filmes\Inicial.mkv`,
	})
	t.Cleanup(r.lifecycle.Shutdown)
	return r
}

func (r *rig) attach(t *testing.T) {
	t.Helper()
	r.lifecycle.Attach(t.Context(), r.ui)
}

type nopDisplay struct{}

func (nopDisplay) Show(session.Screen) error { return nil }
func (nopDisplay) Notify(string) error       { return nil }
func (nopDisplay) Done() <-chan struct{}     { return nil }
func (nopDisplay) Close() error              { return nil }

func eventually(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
