package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	"lingoreel/internal/domain/project"
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
	mu      sync.Mutex
	seeks   []time.Duration
	speeds  []playback.Speed
}

func (r *fakeRunning) lastSeek() (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.seeks) == 0 {
		return 0, false
	}
	return r.seeks[len(r.seeks)-1], true
}

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{Pause: true, Seek: true, Speed: true, Precision: playback.PrecisionExact}
}
func (r *fakeRunning) Play() error  { return nil }
func (r *fakeRunning) Pause() error { r.paused++; return nil }
func (r *fakeRunning) SeekTo(at time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seeks = append(r.seeks, at)
	return nil
}
func (r *fakeRunning) SetSpeed(speed playback.Speed) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.speeds = append(r.speeds, speed)
	return nil
}
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
	// program is what the program dialog answers.
	program string
	// savePath is what the save dialog answers; suggested keeps what it
	// was offered.
	savePath  string
	suggested []string
}

func (u *fakeUI) Emit(name string, data any) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.events = append(u.events, event{name, data})
}
func (u *fakeUI) OpenVideoDialog() (string, error)         { return u.choose, nil }
func (u *fakeUI) OpenProgramDialog(string) (string, error) { return u.program, nil }
func (u *fakeUI) SaveSubtitleDialog(suggested string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.suggested = append(u.suggested, suggested)
	return u.savePath, nil
}

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

// memoryProjects keeps each video's project as JSON, as a file would: what
// the app changes after a save is not in the store until it saves again.
type memoryProjects struct {
	mu      sync.Mutex
	files   map[string][]byte
	saves   int
	loadErr error
	saveErr error
}

func (m *memoryProjects) Load(video string) (project.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr != nil {
		return project.Project{}, m.loadErr
	}
	data, found := m.files[video]
	if !found {
		return project.New(), nil
	}
	var p project.Project
	err := json.Unmarshal(data, &p)
	return p, err
}

func (m *memoryProjects) Save(video string, p project.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	if m.saveErr != nil {
		return m.saveErr
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[video] = data
	return nil
}

func (m *memoryProjects) TrackKey(_ string, track media.Track) string {
	return fmt.Sprintf("track:%d", track.Number)
}

// stored is the project as last saved, and how many saves there were.
func (m *memoryProjects) stored(t *testing.T, video string) (project.Project, int) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	data, found := m.files[video]
	if !found {
		return project.Project{}, m.saves
	}
	var p project.Project
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return p, m.saves
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
	display   *recordingDisplay
	preview   *fakePreview
	projects  *memoryProjects
	log       *lockedBuffer
}

// lockedBuffer is the log's destination in the tests.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		ui: &fakeUI{}, plugin: &fakePlugin{}, settings: newMemorySettings(),
		installer: &fakeInstaller{available: true}, display: &recordingDisplay{}, preview: &fakePreview{}, projects: &memoryProjects{}, log: &lockedBuffer{},
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
			OpenDisplay: func(context.Context, players.Running, media.Info) (session.Display, error) { return r.display, nil },
		},
		Settings: r.settings, Installer: r.installer, Projects: r.projects,
		Logger:       slog.New(slog.NewTextHandler(r.log, nil)),
		Window:       func(media.Info) player.Placement { return player.Placement{Width: 800, Height: 450} },
		InitialVideo: `D:\Filmes\Inicial.mkv`,
		Preview:      r.preview, SettingsDir: `C:\Users\x\AppData\Roaming\LingoReel`,
		OpenFolder: func(path string) error {
			r.ui.mu.Lock()
			defer r.ui.mu.Unlock()
			r.ui.urls = append(r.ui.urls, "folder:"+path)
			return nil
		},
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

// recordingDisplay is the subtitle display; it keeps the looks it was given.
type recordingDisplay struct {
	nopDisplay
	mu      sync.Mutex
	looks   []session.Look
	notices []string
}

func (d *recordingDisplay) Notify(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, text)
	return nil
}

func (d *recordingDisplay) noticeCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.notices)
}

func (d *recordingDisplay) Restyle(look session.Look) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.looks = append(d.looks, look)
	return nil
}

func (d *recordingDisplay) lastLook() (session.Look, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.looks) == 0 {
		return session.Look{}, 0
	}
	return d.looks[len(d.looks)-1], len(d.looks)
}

// fakePreview draws nothing: it answers a fixed picture and keeps the
// requests.
type fakePreview struct {
	mu       sync.Mutex
	requests []previewCall
	err      error
}

type previewCall struct {
	look                session.Look
	width, height, dpi  int
	primary, translated string
}

func (p *fakePreview) Preview(look session.Look, width, height, dpi int, primary, translation string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, previewCall{look, width, height, dpi, primary, translation})
	return []byte("PNG!"), p.err
}

func (p *fakePreview) Fonts() ([]string, error) { return []string{"Arial", "Verdana"}, nil }
