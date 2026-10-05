package watch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// The doubles of a watch session: the media reader, the player and its
// plugin, the app's window, the display, the shortcuts and the listener,
// and the rig that starts a session with them.

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
	// audio makes the player switch audio tracks; loud, change its volume.
	audio, loud bool
	// reconnecting is what Reconnecting answers.
	reconnecting atomic.Bool
}

func (r *fakeRunning) Reconnecting() bool { return r.reconnecting.Load() }

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, Speed: true, Precision: playback.PrecisionExact,
		FrameStepForward: r.frames, FrameStepBackward: r.frames, AudioTrackSelection: r.audio,
		Volume: r.loud, MaxVolume: playback.MaxVolume,
	}
}

// SetVolume answers only when loud makes the player change its volume.
func (r *fakeRunning) SetVolume(volume playback.Volume) error {
	r.record(fmt.Sprintf("volume %d", volume.Percent()))
	return nil
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
func (r *fakeRunning) SelectAudioTrack(index int) error {
	r.record(fmt.Sprintf("audio %d", index))
	return nil
}
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
	// syncedTracks are the tracks the syncs were told with.
	syncedTracks []uint64
	edited       []editedLine
	marked       []markedPassage
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
	track uint64
	edits map[string]project.Edit
}

func (l *recordingListener) SubtitlesChanged(time.Duration, session.Visible) {}
func (l *recordingListener) Edited(line session.Line, track uint64, edits map[string]project.Edit) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.edited = append(l.edited, editedLine{line, track, edits})
}

func (l *recordingListener) edits() []editedLine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]editedLine(nil), l.edited...)
}

func (l *recordingListener) Synced(_ session.Line, track uint64, offset subtitle.Offset) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.synced = append(l.synced, offset)
	l.syncedTracks = append(l.syncedTracks, track)
}

func (l *recordingListener) syncs() ([]subtitle.Offset, []uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]subtitle.Offset(nil), l.synced...), append([]uint64(nil), l.syncedTracks...)
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
