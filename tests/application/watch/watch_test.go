package watch_test

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
	"lingoreel/internal/domain/subtitle"
)

// --- media ---

type fakeReader struct{}

func (fakeReader) Analyze(string) (media.Info, error) {
	return media.Info{Duration: time.Minute, Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo, Width: 1280, Height: 720, FrameRate: 25},
		{Number: 2, Type: media.TrackAudio, Language: "pt"},
		{Number: 3, Type: media.TrackAudio, Language: "en"},
		{Number: 4, Type: media.TrackSubtitle, Language: "en", Readable: true},
		{Number: 5, Type: media.TrackSubtitle, Language: "pt-BR", Readable: true},
	}}, nil
}
func (fakeReader) Check(string) error { return nil }
func (fakeReader) ReadSubtitles(context.Context, string, []uint64, func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	line := func(id, text string, start, end int) subtitle.Event {
		return subtitle.Event{ID: id, Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond, OriginalText: text}
	}
	return map[uint64]media.SubtitleContent{
		4: {Events: []subtitle.Event{line("e1", "Hello there.", 1000, 3000)}},
		5: {Events: []subtitle.Event{line("p1", "Olá.", 1000, 3000)}},
	}, nil
}

// --- player ---

type fakeRunning struct {
	samples chan playback.Sample
	lost    error
	mu      sync.Mutex
	closed  int
}

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{Pause: true, Seek: true, Speed: true, Precision: playback.PrecisionExact}
}
func (r *fakeRunning) Play() error                     { return nil }
func (r *fakeRunning) Pause() error                    { return nil }
func (r *fakeRunning) SeekTo(time.Duration) error      { return nil }
func (r *fakeRunning) SetSpeed(playback.Speed) error   { return nil }
func (r *fakeRunning) StepFrame(bool) error            { return errors.New("no") }
func (r *fakeRunning) SelectAudioTrack(int) error      { return nil }
func (r *fakeRunning) Samples() <-chan playback.Sample { return r.samples }
func (r *fakeRunning) ProcessID() int                  { return 4242 }
func (r *fakeRunning) Err() error                      { return r.lost }
func (r *fakeRunning) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
	return nil
}
func (r *fakeRunning) closes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type fakePlugin struct {
	running  *fakeRunning
	requests []player.LaunchRequest
}

func (p *fakePlugin) Descriptor() player.Descriptor {
	return player.Descriptor{ID: "fake", Name: "Fake", Install: player.Install{DownloadURL: "https://example.org", Benefit: "Teste."}}
}
func (p *fakePlugin) Launch(_ context.Context, _ string, request player.LaunchRequest) (players.Running, error) {
	p.requests = append(p.requests, request)
	return p.running, nil
}

// --- display and shortcuts ---

type fakeDisplay struct {
	mu      sync.Mutex
	shown   []session.Screen
	notices []string
	closed  bool
}

func (d *fakeDisplay) Show(screen session.Screen) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shown = append(d.shown, screen)
	return nil
}
func (d *fakeDisplay) Notify(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, text)
	return nil
}
func (d *fakeDisplay) Done() <-chan struct{} { return nil }
func (d *fakeDisplay) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}
func (d *fakeDisplay) last() (session.Screen, int, []string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.shown) == 0 {
		return session.Screen{}, 0, d.notices, d.closed
	}
	return d.shown[len(d.shown)-1], len(d.shown), append([]string(nil), d.notices...), d.closed
}

type fakeShortcuts struct {
	presses chan watch.Press
	once    sync.Once
	closed  bool
}

func (s *fakeShortcuts) Presses() <-chan watch.Press { return s.presses }
func (s *fakeShortcuts) Close() error {
	s.once.Do(func() { close(s.presses); s.closed = true })
	return nil
}

type recordingListener struct {
	mu     sync.Mutex
	synced []subtitle.Offset
}

func (l *recordingListener) SubtitlesChanged(time.Duration, session.Visible) {}
func (l *recordingListener) Synced(_ session.Line, offset subtitle.Offset) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.synced = append(l.synced, offset)
}

// --- harness ---

type rig struct {
	running   *fakeRunning
	plugin    *fakePlugin
	display   *fakeDisplay
	shortcuts *fakeShortcuts
	listener  *recordingListener
	ports     watch.Ports
	request   watch.Request
}

func newRig(t *testing.T) *rig {
	t.Helper()
	service := analysis.NewService(fakeReader{}, nil)
	result, err := service.Analyze(t.Context(), `C:\v.mkv`, media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{
		running:   &fakeRunning{samples: make(chan playback.Sample, 4)},
		display:   &fakeDisplay{},
		shortcuts: &fakeShortcuts{presses: make(chan watch.Press, 4)},
		listener:  &recordingListener{},
	}
	r.plugin = &fakePlugin{running: r.running}
	registry, err := players.NewRegistry(r.plugin)
	if err != nil {
		t.Fatal(err)
	}
	r.ports = watch.Ports{
		Service: service, Registry: registry,
		OpenDisplay: func(context.Context, players.Running, media.Info) (session.Display, error) {
			return r.display, nil
		},
		StartShortcuts: func(int) (watch.Shortcuts, error) { return r.shortcuts, nil },
	}
	r.request = watch.Request{
		Analysis: result, Tracks: result.Suggested, Player: player.Installation{ID: "fake", Path: `C:\fake.exe`},
		Start: 2 * time.Second, Paused: true, Window: player.Placement{X: 1, Y: 2, Width: 640, Height: 360},
		Steps: subtitle.DefaultNudgeSteps(),
	}
	return r
}

func (r *rig) start(t *testing.T) *watch.Session {
	t.Helper()
	s, err := watch.Start(t.Context(), r.ports, r.request, r.listener, func(line session.Line, offset subtitle.Offset) string {
		return offset.Duration().String()
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { s.Stop() })
	return s
}

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

func TestStartLaunchesWithTheRequest(t *testing.T) {
	r := newRig(t)
	r.start(t)
	if len(r.plugin.requests) != 1 {
		t.Fatalf("launched %d times", len(r.plugin.requests))
	}
	got := r.plugin.requests[0]
	// English audio is the second audio track: index 1. The window, start and
	// pause come from the request; the width from the video track.
	if got.File != `C:\v.mkv` || got.AudioIndex != 1 || got.Start != 2*time.Second || !got.StartPaused ||
		got.Window != r.request.Window || got.VideoWidth != 1280 {
		t.Fatalf("launch request = %+v", got)
	}
}

func TestSubtitlesReachTheDisplay(t *testing.T) {
	r := newRig(t)
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 1500 * time.Millisecond, Duration: time.Minute, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()}
	eventually(t, "both lines on the display", func() bool {
		screen, _, _, _ := r.display.last()
		return screen.Primary == "Hello there." && screen.Translation == "Olá." && screen.TranslationVisible
	})
	_, before, _, _ := r.display.last()
	s.SetTranslationVisible(false)
	screen, after, _, _ := r.display.last()
	if after != before+1 || screen.TranslationVisible || screen.Translation != "Olá." {
		t.Fatalf("hiding the translation showed %+v (%d -> %d draws); want the same lines, translation hidden", screen, before, after)
	}
	if s.TranslationVisible() {
		t.Fatal("TranslationVisible still true")
	}
}

func TestShortcutsAndButtonsMoveTheLines(t *testing.T) {
	r := newRig(t)
	s := r.start(t)
	r.shortcuts.presses <- watch.Press{Index: 1} // top line later
	r.shortcuts.presses <- watch.Press{Index: 99}
	r.shortcuts.presses <- watch.Press{Index: 2} // bottom line earlier
	eventually(t, "two sync changes", func() bool {
		r.listener.mu.Lock()
		defer r.listener.mu.Unlock()
		return len(r.listener.synced) == 2
	})
	if s.Offset(session.LinePrimary) != subtitle.Offset(50*time.Millisecond) || s.Offset(session.LineTranslation) != subtitle.Offset(-50*time.Millisecond) {
		t.Fatalf("offsets = %v / %v, want +50ms / -50ms", s.Offset(session.LinePrimary), s.Offset(session.LineTranslation))
	}
	if offset, ok := s.Nudge(session.LinePrimary, true); !ok || offset != subtitle.Offset(100*time.Millisecond) {
		t.Fatalf("Nudge = %v %v", offset, ok)
	}
	s.ResetSync(session.LinePrimary)
	if s.Offset(session.LinePrimary) != 0 {
		t.Fatal("ResetSync left an offset")
	}
	_, _, notices, _ := r.display.last()
	if len(notices) != 4 || notices[3] != "0s" {
		t.Fatalf("notices = %q, want one per change ending with the reset", notices)
	}
}

func TestPlayerClosingEndsTheSession(t *testing.T) {
	r := newRig(t)
	s := r.start(t)
	close(r.running.samples)
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("the session did not end with the player")
	}
	if err := s.Err(); err != nil {
		t.Fatalf("Err after the user closed the player = %v", err)
	}
	if _, _, _, closed := r.display.last(); !closed || !r.shortcuts.closed || r.running.closes() != 1 {
		t.Fatalf("display closed %v, shortcuts closed %v, player closes %d", closed, r.shortcuts.closed, r.running.closes())
	}
	if err := s.Stop(); err != nil || r.running.closes() != 1 {
		t.Fatalf("Stop after the end = %v, player closes %d", err, r.running.closes())
	}
}

func TestACrashIsReported(t *testing.T) {
	r := newRig(t)
	r.running.lost = playback.ErrPlayerLost
	s := r.start(t)
	close(r.running.samples)
	<-s.Done()
	if !errors.Is(s.Err(), playback.ErrPlayerLost) {
		t.Fatalf("Err = %v, want ErrPlayerLost", s.Err())
	}
}

func TestStopClosesEverythingOnce(t *testing.T) {
	r := newRig(t)
	s := r.start(t)
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, closed := r.display.last(); !closed || r.running.closes() != 1 {
		t.Fatalf("display closed %v, player closes %d", closed, r.running.closes())
	}
}

func TestStartFailuresCleanUp(t *testing.T) {
	t.Run("player without a plugin", func(t *testing.T) {
		r := newRig(t)
		r.request.Player.ID = "other"
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); !errors.Is(err, watch.ErrNoPlugin) {
			t.Fatalf("err = %v, want ErrNoPlugin", err)
		}
		if len(r.plugin.requests) != 0 {
			t.Fatal("launched a player anyway")
		}
	})
	t.Run("display fails", func(t *testing.T) {
		r := newRig(t)
		r.ports.OpenDisplay = func(context.Context, players.Running, media.Info) (session.Display, error) {
			return nil, errors.New("no window")
		}
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
			t.Fatal("Start succeeded")
		}
		if r.running.closes() != 1 {
			t.Fatalf("player closes = %d, want the player closed", r.running.closes())
		}
	})
	t.Run("shortcuts fail", func(t *testing.T) {
		r := newRig(t)
		r.ports.StartShortcuts = func(int) (watch.Shortcuts, error) { return nil, errors.New("taken") }
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
			t.Fatal("Start succeeded")
		}
		if _, _, _, closed := r.display.last(); !closed || r.running.closes() != 1 {
			t.Fatalf("display closed %v, player closes %d", closed, r.running.closes())
		}
	})
	t.Run("missing ports", func(t *testing.T) {
		r := newRig(t)
		if _, err := watch.Start(t.Context(), watch.Ports{}, r.request, nil, nil); err == nil {
			t.Fatal("Start without ports succeeded")
		}
	})
}

func TestScreenStacksSimultaneousLines(t *testing.T) {
	visible := session.Visible{
		Primary:     []subtitle.Event{{ID: "a", OriginalText: "One."}, {ID: "b", OriginalText: "Two.", OverrideText: "Dois."}},
		Translation: nil,
	}
	got := watch.Screen(visible, false)
	if got.Primary != "One.\nDois." || got.Translation != "" || got.TranslationVisible {
		t.Fatalf("Screen = %+v", got)
	}
}
