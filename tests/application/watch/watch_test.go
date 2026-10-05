package watch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
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
	// order records the closes of the player and the window, shared with
	// fakeWindow.
	order *[]string
	// speeds are the speeds asked; speedErr is the answer.
	speeds   []playback.Speed
	speedErr error
	// frames makes the player step frames; calls records what it was asked.
	frames bool
	calls  []string
	// reconnecting is what Reconnecting answers.
	reconnecting atomic.Bool
}

func (r *fakeRunning) Reconnecting() bool { return r.reconnecting.Load() }

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, Speed: true, Precision: playback.PrecisionExact,
		FrameStepForward: r.frames, FrameStepBackward: r.frames,
	}
}
func (r *fakeRunning) Play() error  { r.record("play"); return nil }
func (r *fakeRunning) Pause() error { r.record("pause"); return nil }
func (r *fakeRunning) SeekTo(position time.Duration) error {
	r.record("seek " + position.String())
	return nil
}
func (r *fakeRunning) SetSpeed(speed playback.Speed) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.speeds = append(r.speeds, speed)
	r.calls = append(r.calls, fmt.Sprintf("speed %v", speed))
	return r.speedErr
}
func (r *fakeRunning) StepFrame(forward bool) error {
	if !r.frames {
		return errors.New("no")
	}
	r.record(map[bool]string{true: "frame forward", false: "frame back"}[forward])
	return nil
}
func (r *fakeRunning) record(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

// took returns the calls since the last time and forgets them.
func (r *fakeRunning) took() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := r.calls
	r.calls = nil
	return calls
}
func (r *fakeRunning) SelectAudioTrack(int) error      { return nil }
func (r *fakeRunning) Samples() <-chan playback.Sample { return r.samples }
func (r *fakeRunning) ProcessID() int                  { return 4242 }
func (r *fakeRunning) Err() error                      { return r.lost }
func (r *fakeRunning) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
	if r.order != nil {
		*r.order = append(*r.order, "player")
	}
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
	// hosted makes the player draw into the app's window.
	hosted    bool
	launchErr error
}

func (p *fakePlugin) Descriptor() player.Descriptor {
	return player.Descriptor{ID: "fake", Name: "Fake", Install: player.Install{DownloadURL: "https://example.org", Benefit: "Teste."}, DrawsInHost: p.hosted}
}
func (p *fakePlugin) Launch(_ context.Context, _ string, request player.LaunchRequest) (players.Running, error) {
	p.requests = append(p.requests, request)
	if p.launchErr != nil {
		return nil, p.launchErr
	}
	return p.running, nil
}

// fakeWindow is the app's player window.
type fakeWindow struct {
	fakeDisplay
	closed  chan struct{}
	order   *[]string
	mu      sync.Mutex
	remote  watch.Remote
	closes  int
	request watch.WindowRequest
	keys    shortcut.Map
	looks   []session.Look
}

const fakeHandle = 777

func (w *fakeWindow) Handle() uintptr         { return fakeHandle }
func (w *fakeWindow) Closed() <-chan struct{} { return w.closed }
func (w *fakeWindow) SetShortcuts(keys shortcut.Map) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys = keys
}
func (w *fakeWindow) Restyle(look session.Look) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.looks = append(w.looks, look)
	return nil
}
func (w *fakeWindow) Attach(remote watch.Remote) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.remote = remote
}
func (w *fakeWindow) Close() error {
	w.mu.Lock()
	w.closes++
	if w.order != nil {
		*w.order = append(*w.order, "window")
	}
	w.mu.Unlock()
	return w.fakeDisplay.Close()
}
func (w *fakeWindow) state() (watch.Remote, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.remote, w.closes
}

func (w *fakeWindow) lastKeys() shortcut.Map {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.keys
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
	edited []editedLine
	marked []markedPassage
}

// markedPassage is one call of PassageMarked.
type markedPassage struct {
	segment study.Segment
	plan    study.PracticePlan
}

func (l *recordingListener) PassageMarked(segment study.Segment, plan study.PracticePlan) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.marked = append(l.marked, markedPassage{segment, plan})
}

func (l *recordingListener) passages() []markedPassage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]markedPassage(nil), l.marked...)
}

// editedLine is one call of Edited.
type editedLine struct {
	line  session.Line
	edits map[string]project.Edit
}

func (l *recordingListener) SubtitlesChanged(time.Duration, session.Visible) {}
func (l *recordingListener) Edited(line session.Line, edits map[string]project.Edit) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.edited = append(l.edited, editedLine{line, edits})
}

func (l *recordingListener) edits() []editedLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]editedLine(nil), l.edited...)
}

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

	mu     sync.Mutex
	target watch.ShortcutTarget
	// chords are the chords of every start of the sync shortcuts; later
	// are the shortcuts of the starts after the first.
	chords [][]shortcut.Chord
	later  []*fakeShortcuts
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
		StartShortcuts: func(target watch.ShortcutTarget, chords []shortcut.Chord) (watch.Shortcuts, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.target = target
			r.chords = append(r.chords, chords)
			if len(r.chords) == 1 {
				return r.shortcuts, nil
			}
			next := &fakeShortcuts{presses: make(chan watch.Press, 4)}
			r.later = append(r.later, next)
			return next, nil
		},
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
		r.ports.StartShortcuts = func(watch.ShortcutTarget, []shortcut.Chord) (watch.Shortcuts, error) { return nil, errors.New("taken") }
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

// hostedRig is a rig whose player draws into the app's window.
func hostedRig(t *testing.T) (*rig, *fakeWindow, *[]string) {
	t.Helper()
	r := newRig(t)
	order := &[]string{}
	r.plugin.hosted = true
	r.running.order = order
	window := &fakeWindow{closed: make(chan struct{}), order: order}
	r.ports.OpenWindow = func(_ context.Context, request watch.WindowRequest) (watch.Window, error) {
		window.request = request
		return window, nil
	}
	r.ports.OpenDisplay = func(context.Context, players.Running, media.Info) (session.Display, error) {
		t.Error("a hosted player got an overlay of its own")
		return r.display, nil
	}
	return r, window, order
}

func TestAHostedPlayerDrawsIntoTheAppWindow(t *testing.T) {
	r, window, _ := hostedRig(t)
	s := r.start(t)
	got := r.plugin.requests[0]
	if got.Host != fakeHandle || got.Window != r.request.Window {
		t.Fatalf("launch request host %d window %+v, want host %d and the placement", got.Host, got.Window, fakeHandle)
	}
	if window.request.Title != "v.mkv" || window.request.Placement != r.request.Window || window.request.Info.Duration != time.Minute {
		t.Fatalf("window request = %+v", window.request)
	}
	// The sync shortcuts follow the window, not every window of the process.
	if r.target != (watch.ShortcutTarget{ProcessID: 4242, Window: fakeHandle}) {
		t.Fatalf("shortcut target = %+v", r.target)
	}
	remote, _ := window.state()
	if remote == nil || !remote.HasTranslation() || remote.Controls() != s.Controls() {
		t.Fatalf("the window's controls do not drive the session (remote %v)", remote)
	}
	r.running.samples <- playback.Sample{Position: 1500 * time.Millisecond, Duration: time.Minute, Speed: 1, Status: playback.StatusPaused, ObservedAt: time.Now()}
	eventually(t, "the subtitles in the window", func() bool {
		screen, _, _, _ := window.last()
		return screen.Primary == "Hello there."
	})
}

func TestClosingThePlayerWindowEndsTheSession(t *testing.T) {
	r, window, order := hostedRig(t)
	s := r.start(t)
	close(window.closed) // the close button, Alt+F4 or the taskbar
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("closing the window did not end the session")
	}
	if err := s.Err(); err != nil {
		t.Fatalf("Err after the user closed the window = %v", err)
	}
	_, closes := window.state()
	r.running.mu.Lock()
	sequence := append([]string(nil), *order...)
	r.running.mu.Unlock()
	// The player goes first: it draws into the window.
	if r.running.closes() != 1 || closes != 1 || len(sequence) != 2 || sequence[0] != "player" || sequence[1] != "window" {
		t.Fatalf("player closes %d, window closes %d, order %v", r.running.closes(), closes, sequence)
	}
}

func TestAPlayerWithItsOwnWindowGetsNoAppWindow(t *testing.T) {
	r := newRig(t)
	r.ports.OpenWindow = func(context.Context, watch.WindowRequest) (watch.Window, error) {
		t.Error("opened the app's window for a player that draws its own")
		return nil, errors.New("unexpected")
	}
	r.start(t)
	if got := r.plugin.requests[0]; got.Host != 0 {
		t.Fatalf("host = %d, want none", got.Host)
	}
	if r.target != (watch.ShortcutTarget{ProcessID: 4242}) {
		t.Fatalf("shortcut target = %+v", r.target)
	}
}

// Inside the app's window the window takes every key itself, and the app's
// panel takes them when it has the keyboard: system-wide shortcuts would
// only take "<" from the panel's fields.
func TestAWindowThatHearsTheKeysStartsNoGlobalShortcuts(t *testing.T) {
	r, window, _ := hostedRig(t)
	r.ports.WindowHearsKeys = true
	s := r.start(t)
	r.mu.Lock()
	starts := len(r.chords)
	r.mu.Unlock()
	if starts != 0 {
		t.Fatalf("the global shortcuts started %d times for a window that hears the keys", starts)
	}
	// Changing the keys starts none either; the window gets them.
	keys := shortcut.Defaults()
	keys[shortcut.SyncPrimaryEarlier] = shortcut.Chord{Key: "KeyJ"}
	if err := s.SetShortcuts(keys); err != nil {
		t.Fatalf("SetShortcuts: %v", err)
	}
	r.mu.Lock()
	starts = len(r.chords)
	r.mu.Unlock()
	if starts != 0 {
		t.Fatalf("changing the keys started the global shortcuts %d times", starts)
	}
	if got := window.lastKeys(); got[shortcut.SyncPrimaryEarlier] != (shortcut.Chord{Key: "KeyJ"}) {
		t.Fatalf("the window has %v for the sync key, want KeyJ", got[shortcut.SyncPrimaryEarlier])
	}
}

// A player in a window of its own (PotPlayer) keeps the global shortcuts:
// the app hears no key while that window is in front.
func TestAPlayerInItsOwnWindowKeepsTheGlobalShortcuts(t *testing.T) {
	r := newRig(t)
	r.ports.WindowHearsKeys = true // about the app's window, which this player does not use
	r.start(t)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.chords) != 1 || len(r.chords[0]) == 0 {
		t.Fatalf("global shortcuts started %d times, want once with the sync and mark keys", len(r.chords))
	}
}

func TestPlayerWindowFailuresCleanUp(t *testing.T) {
	t.Run("window fails", func(t *testing.T) {
		r, _, _ := hostedRig(t)
		r.ports.OpenWindow = func(context.Context, watch.WindowRequest) (watch.Window, error) {
			return nil, errors.New("no window")
		}
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
			t.Fatal("Start succeeded")
		}
		if len(r.plugin.requests) != 0 {
			t.Fatal("launched a player without its window")
		}
	})
	t.Run("player fails", func(t *testing.T) {
		r, window, _ := hostedRig(t)
		r.plugin.launchErr = errors.New("no player")
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
			t.Fatal("Start succeeded")
		}
		if _, closes := window.state(); closes != 1 {
			t.Fatalf("window closes = %d, want the window closed", closes)
		}
	})
	t.Run("shortcuts fail", func(t *testing.T) {
		r, window, _ := hostedRig(t)
		r.ports.StartShortcuts = func(watch.ShortcutTarget, []shortcut.Chord) (watch.Shortcuts, error) { return nil, errors.New("taken") }
		if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
			t.Fatal("Start succeeded")
		}
		if _, closes := window.state(); closes != 1 || r.running.closes() != 1 {
			t.Fatalf("window closes %d, player closes %d", closes, r.running.closes())
		}
	})
}

func TestKaraokeChoices(t *testing.T) {
	r := newRig(t)
	r.request.LineRepeats = study.MaxRepeats + 1
	if _, err := watch.Start(t.Context(), r.ports, r.request, nil, nil); err == nil {
		t.Fatal("Start with 11 line repeats succeeded")
	}
	if r.running.closes() != 1 {
		t.Fatalf("player closes = %d, want the player closed", r.running.closes())
	}

	r = newRig(t)
	r.request.Karaoke = session.Karaoke{Enabled: true, Rules: study.DefaultCueRules()}
	r.request.Indicator = study.IndicatorCountdown
	r.request.LineRepeats = 2
	s := r.start(t)
	if !s.Karaoke().Enabled || s.Indicator() != study.IndicatorCountdown || s.LineRepeats() != 2 {
		t.Fatalf("karaoke %+v indicator %q repeats %d", s.Karaoke(), s.Indicator(), s.LineRepeats())
	}
	s.SetKaraoke(false)
	if s.Karaoke().Enabled || s.Karaoke().Rules != study.DefaultCueRules() {
		t.Fatalf("turning it off lost the rules: %+v", s.Karaoke())
	}
	if err := s.SetIndicator("dots"); err == nil || s.Indicator() != study.IndicatorCountdown {
		t.Fatal("an unknown indicator was taken")
	}
	// No indicator asked: the bar.
	plain := newRig(t).start(t)
	if plain.Indicator() != study.IndicatorBar {
		t.Fatalf("default indicator %q", plain.Indicator())
	}
}

func TestStudyPassageHidesLinesByLap(t *testing.T) {
	r := newRig(t)
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 1500 * time.Millisecond, Duration: time.Minute, Speed: 1, Status: playback.StatusPlaying, ObservedAt: time.Now()}
	eventually(t, "the line on screen", func() bool { screen, _, _, _ := r.display.last(); return screen.Primary == "Hello there." })
	// "Esta fala" at 1.5 s: the line 1–3 s, with its lead-in.
	if segment, err := s.LinePassage(); err != nil || segment.Start != 500*time.Millisecond || segment.End != 3*time.Second {
		t.Fatalf("LinePassage = %+v, %v", segment, err)
	}
	plan := study.PracticePlan{Mode: study.EndPause, Ladder: true}
	if err := s.StudyPassage(study.Segment{Start: time.Second, End: 3 * time.Second}, plan, true); err != nil {
		t.Fatal(err)
	}
	if status := s.Passage(); !status.Active || status.Lap != 1 || status.Segment.Start != 500*time.Millisecond {
		t.Fatalf("passage %+v; want lap 1 fitted to the line (1 s - 0.5 s)", status)
	}

	s.StopPassage()
	if s.Passage().Active {
		t.Fatal("StopPassage left the passage")
	}
	if screen, _, _, _ := r.display.last(); screen.PrimaryHidden || !screen.TranslationVisible {
		t.Fatalf("after the passage the lines stay hidden: %+v", screen)
	}
}
