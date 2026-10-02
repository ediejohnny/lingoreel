// Package watch is the use case of watching a video: it loads the chosen
// subtitles, opens the video in a player plugin, draws the subtitles over it,
// listens to the sync shortcuts and keeps everything in step until the player
// closes or the session is stopped. The app and the development tool share it.
package watch

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/application/players"
	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

// SyncShortcut moves one subtitle line earlier or later.
type SyncShortcut struct {
	Line  session.Line
	Later bool
}

// SyncShortcuts are the four sync shortcuts, in the order their presses
// refer to: the line on top earlier and later, then the one below.
var SyncShortcuts = []SyncShortcut{
	{session.LinePrimary, false}, {session.LinePrimary, true},
	{session.LineTranslation, false}, {session.LineTranslation, true},
}

// Press is one press of SyncShortcuts[Index]; Repeat tells the key is held.
type Press struct {
	Index  int
	Repeat bool
}

// Shortcuts delivers the presses of the sync shortcuts while the player is in
// the foreground. The Windows hotkeys adapter implements it.
type Shortcuts interface {
	Presses() <-chan Press
	Close() error
}

// ShortcutTarget is the player the sync shortcuts follow: the app's window
// it draws into, or else (Window zero) any window of its process.
type ShortcutTarget struct {
	ProcessID int
	Window    uintptr
}

// Remote is what the controls on the player window drive: the same
// session the app's panel drives, so both always agree.
type Remote interface {
	State() playback.Sample
	Controls() *session.Controls
	HasTranslation() bool
	TranslationVisible() bool
	SetTranslationVisible(visible bool)
	// ReplayLine is "Treinar de novo" of the Modo Karaokê.
	ReplayLine() error
}

// Window is the app's own window a player draws its video into. It shows
// the subtitles (it is the session's display) and its own controls over the
// video, and closing it by any means ends the session.
type Window interface {
	session.Display
	// Handle is the window the player draws into.
	Handle() uintptr
	// Closed is closed when the user closes the window.
	Closed() <-chan struct{}
	// Attach hands the window the session its controls drive, once the
	// player shows the video.
	Attach(remote Remote)
}

// WindowRequest describes the player window to open.
type WindowRequest struct {
	// Title names the video, usually its file name.
	Title     string
	Placement player.Placement
	Info      media.Info
}

// Ports are the outside pieces a session uses; nil ones are left out.
type Ports struct {
	Service  *analysis.Service
	Registry *players.Registry
	// OpenWindow opens the app's window for players that draw into one
	// (player.Descriptor.DrawsInHost); nil lets every player open its own.
	OpenWindow func(ctx context.Context, request WindowRequest) (Window, error)
	// OpenDisplay puts the subtitle display over a player's own window; nil
	// shows nothing (the development tool prints instead).
	OpenDisplay func(ctx context.Context, running players.Running, info media.Info) (session.Display, error)
	// StartShortcuts listens to SyncShortcuts while the player is in the
	// foreground; nil leaves the shortcuts off.
	StartShortcuts func(target ShortcutTarget) (Shortcuts, error)
	// PracticeNotice words what happens to a passage being studied for a
	// notice on the video; nil shows none.
	PracticeNotice func(session.PracticeEvent, session.PracticeStatus) string
	Now            func() time.Time
}

// Request is what to watch and how.
type Request struct {
	Analysis analysis.Analysis
	// Tracks are the chosen audio, primary and translation tracks; zero
	// means none (or the player's choice for audio).
	Tracks media.Selection
	Player player.Installation
	Start  time.Duration
	Paused bool
	// Window is where the player's window opens; zero leaves it to the player.
	Window player.Placement
	Steps  subtitle.NudgeSteps
	// TranslationHidden starts with the translation hidden (study mode on demand).
	TranslationHidden bool
	// Karaoke is the Modo Karaokê to start with, its indicator and how many
	// times each line plays again (0 is off).
	Karaoke     session.Karaoke
	Indicator   study.Indicator
	LineRepeats int
}

// Listener hears what happens in a session. Its methods run on the
// session's goroutines and must return quickly.
type Listener interface {
	// SubtitlesChanged reports new lines on screen at a playback position.
	SubtitlesChanged(position time.Duration, visible session.Visible)
	// Synced reports a line's new offset after a sync shortcut or a reset.
	Synced(line session.Line, offset subtitle.Offset)
}

// Session is a video being watched.
type Session struct {
	running   players.Running
	display   session.Display
	shortcuts Shortcuts
	clock     *session.Clock
	controls  *session.Controls
	subtitles *session.Subtitles
	sync      *session.Sync
	listener  Listener
	loaded    map[uint64]analysis.LoadedTrack
	notice    func(session.Line, subtitle.Offset) string
	// hasTranslation tells a translation track is loaded.
	hasTranslation bool

	translationVisible atomic.Bool
	lastVisible        atomic.Pointer[session.Visible]
	practice           *session.Practice
	indicator          atomic.Value // study.Indicator
	// lap is how the current lap of a passage shows the lines (the ladder).
	lap atomic.Pointer[study.Lap]

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	err      error
}

// ErrNoPlugin reports a player the registry cannot drive.
var ErrNoPlugin = errors.New("player not supported")

// Start opens the video and begins the session. It returns once the player
// shows the video; the session then runs until the player closes or Stop.
// notice words a sync change for the notice on the video; nil shows none.
func Start(ctx context.Context, ports Ports, request Request, listener Listener, notice func(session.Line, subtitle.Offset) string) (*Session, error) {
	if ports.Service == nil || ports.Registry == nil {
		return nil, errors.New("watch: missing analysis service or player registry")
	}
	if !ports.Registry.Supported(request.Player.ID) {
		return nil, fmt.Errorf("%w: %s", ErrNoPlugin, request.Player.ID)
	}
	now := ports.Now
	if now == nil {
		now = time.Now
	}
	info := request.Analysis.Info
	lines := []uint64{request.Tracks.Primary, request.Tracks.Translation}
	loaded, err := ports.Service.LoadSubtitles(ctx, request.Analysis, lines, nil)
	if err != nil {
		return nil, err
	}

	launch := player.LaunchRequest{
		File: request.Analysis.Path, AudioIndex: audioIndex(info, request.Tracks.Audio),
		StartPaused: request.Paused, Start: request.Start, Window: request.Window,
	}
	launch.VideoWidth, _ = videoSize(info)
	window, err := openWindow(ctx, ports, request)
	if err != nil {
		return nil, err
	}
	if window != nil {
		launch.Host = window.Handle()
	}
	running, err := ports.Registry.Launch(ctx, request.Player, launch)
	if err != nil {
		return nil, errors.Join(err, closeWindow(window))
	}

	s := &Session{
		running: running, listener: listener, loaded: loaded, notice: notice,
		clock:          session.NewClock(running.Capabilities().Precision, now),
		hasTranslation: request.Tracks.Translation != 0 && loaded[request.Tracks.Translation].Timeline != nil,
		done:           make(chan struct{}),
	}
	s.translationVisible.Store(!request.TranslationHidden)
	s.subtitles = session.NewSubtitles(trackOf(loaded, request.Tracks.Primary), trackOf(loaded, request.Tracks.Translation))
	s.controls = session.NewControls(running, s.clock, frameRate(info))
	s.sync = session.NewSync(s.subtitles, request.Steps, now)
	s.subtitles.SetKaraoke(request.Karaoke)
	s.indicator.Store(cmp.Or(request.Indicator, study.IndicatorBar))
	s.practice = session.NewPractice(s.controls, s.subtitles, session.PracticeHooks{
		Event: s.practiced(ports.PracticeNotice), Look: s.setLap,
	}, now)
	if err := s.practice.SetLineRepeats(request.LineRepeats); err != nil {
		return nil, errors.Join(err, running.Close(), closeWindow(window))
	}

	target := ShortcutTarget{ProcessID: running.ProcessID()}
	switch {
	case window != nil:
		s.display, target.Window = window, window.Handle()
	case ports.OpenDisplay != nil:
		if s.display, err = ports.OpenDisplay(ctx, running, info); err != nil {
			return nil, errors.Join(fmt.Errorf("opening the subtitle display: %w", err), running.Close())
		}
	}
	if ports.StartShortcuts != nil {
		if s.shortcuts, err = ports.StartShortcuts(target); err != nil {
			return nil, errors.Join(fmt.Errorf("starting the sync shortcuts: %w", err), running.Close(), s.closeDisplay())
		}
		go s.listenShortcuts()
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	runner := session.NewRunner(s.clock, s.subtitles, s.changed, session.NewRealTimer)
	runner.Follow(s.practice)
	go s.run(runCtx, runner)
	if window != nil {
		window.Attach(s)
		go s.endWhenClosed(window.Closed())
	}
	return s, nil
}

// openWindow opens the app's window when the player draws into one.
func openWindow(ctx context.Context, ports Ports, request Request) (Window, error) {
	plugin, found := ports.Registry.Plugin(request.Player.ID)
	if ports.OpenWindow == nil || !found || !plugin.Descriptor().DrawsInHost {
		return nil, nil
	}
	window, err := ports.OpenWindow(ctx, WindowRequest{
		Title: filepath.Base(request.Analysis.Path), Placement: request.Window, Info: request.Analysis.Info,
	})
	if err != nil {
		return nil, fmt.Errorf("opening the player window: %w", err)
	}
	return window, nil
}

func closeWindow(window Window) error {
	if window == nil {
		return nil
	}
	if err := window.Close(); err != nil {
		return fmt.Errorf("closing the player window: %w", err)
	}
	return nil
}

// endWhenClosed ends the session when the user closes the player window:
// the close button, Alt+F4 or the taskbar. It returns with the session.
func (s *Session) endWhenClosed(closed <-chan struct{}) {
	select {
	case <-closed:
		s.stopOnce.Do(s.cancel)
	case <-s.done:
	}
}

func (s *Session) run(ctx context.Context, runner *session.Runner) {
	err := runner.Run(ctx, s.running.Samples())
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	// Asked before Close, which ends the player on purpose.
	if lost := s.running.Err(); err == nil && lost != nil {
		err = lost
	}
	var shortcutsErr error
	if s.shortcuts != nil {
		shortcutsErr = s.shortcuts.Close()
	}
	// The player first: it may draw into the display's window.
	s.err = errors.Join(err, shortcutsErr, s.running.Close(), s.closeDisplay())
	close(s.done)
}

// changed runs on the runner's loop for every change on screen.
func (s *Session) changed(position time.Duration, visible session.Visible) {
	s.lastVisible.Store(&visible)
	s.show(visible)
	if s.listener != nil {
		s.listener.SubtitlesChanged(position, visible)
	}
}

func (s *Session) show(visible session.Visible) {
	if s.display == nil {
		return
	}
	// The display only fails while its window goes away; the session ends then.
	_ = s.display.Show(s.screen(visible))
}

func (s *Session) listenShortcuts() {
	for press := range s.shortcuts.Presses() {
		if press.Index < 0 || press.Index >= len(SyncShortcuts) {
			continue
		}
		shortcut := SyncShortcuts[press.Index]
		if offset, ok := s.sync.Nudge(shortcut.Line, shortcut.Later, press.Repeat); ok {
			s.synced(shortcut.Line, offset)
		}
	}
}

func (s *Session) synced(line session.Line, offset subtitle.Offset) {
	if s.display != nil && s.notice != nil {
		_ = s.display.Notify(s.notice(line, offset)) // a lost notice is cosmetic
	}
	if s.listener != nil {
		s.listener.Synced(line, offset)
	}
}

// Nudge moves a line one step, as its sync shortcut does.
func (s *Session) Nudge(line session.Line, later bool) (subtitle.Offset, bool) {
	offset, ok := s.sync.Nudge(line, later, false)
	if ok {
		s.synced(line, offset)
	}
	return offset, ok
}

// ResetSync puts a line back at its file's timing.
func (s *Session) ResetSync(line session.Line) {
	s.sync.Reset(line)
	s.synced(line, s.subtitles.Offset(line))
}

// Offset is a line's current offset.
func (s *Session) Offset(line session.Line) subtitle.Offset {
	return s.subtitles.Offset(line)
}

// SetTranslationVisible shows or hides the translation; the primary line
// never moves either way.
func (s *Session) SetTranslationVisible(visible bool) {
	s.translationVisible.Store(visible)
	if last := s.lastVisible.Load(); last != nil {
		s.show(*last)
	}
}

// HasTranslation tells whether the session shows a translation line.
func (s *Session) HasTranslation() bool {
	return s.hasTranslation
}

// TranslationVisible tells whether the translation is shown.
func (s *Session) TranslationVisible() bool {
	return s.translationVisible.Load()
}

// Controls drive the playback.
func (s *Session) Controls() *session.Controls {
	return s.controls
}

// State is the playback right now, extrapolated by the clock.
func (s *Session) State() playback.Sample {
	return s.clock.State()
}

// Loaded are the subtitle tracks the session shows, with the corrections
// applied to their files.
func (s *Session) Loaded() map[uint64]analysis.LoadedTrack {
	return s.loaded
}

// Done is closed when the session has ended and everything is closed.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Err tells why the session ended once Done is closed:
// playback.ErrPlayerLost for a player that crashed, nil for a normal end.
func (s *Session) Err() error {
	select {
	case <-s.done:
		return s.err
	default:
		return nil
	}
}

// Stop ends the session: the player closes, with the display and the
// shortcuts. It waits for the end and is safe to call more than once.
func (s *Session) Stop() error {
	s.stopOnce.Do(s.cancel)
	<-s.done
	return s.err
}

func (s *Session) closeDisplay() error {
	if s.display == nil {
		return nil
	}
	if err := s.display.Close(); err != nil {
		return fmt.Errorf("closing the subtitle display: %w", err)
	}
	return nil
}

// Screen is what the display shows for the visible events: one text per
// line, simultaneous events stacked.
func Screen(visible session.Visible, translationVisible bool) session.Screen {
	return session.Screen{
		Primary:            stack(visible.Primary),
		Translation:        stack(visible.Translation),
		TranslationVisible: translationVisible,
	}
}

func stack(events []subtitle.Event) string {
	texts := make([]string, len(events))
	for i, event := range events {
		texts[i] = event.Text()
	}
	return strings.Join(texts, "\n")
}

func trackOf(loaded map[uint64]analysis.LoadedTrack, number uint64) *session.Track {
	track, found := loaded[number]
	if !found {
		return nil
	}
	return &session.Track{Timeline: track.Timeline}
}

// audioIndex converts a track number into the position among audio tracks
// that players expect; -1 keeps the player's own choice.
func audioIndex(info media.Info, number uint64) int {
	for index, track := range info.TracksOf(media.TrackAudio) {
		if track.Number == number {
			return index
		}
	}
	return -1
}

// videoSize is the first video track's size, or zeros when unknown.
func videoSize(info media.Info) (int, int) {
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		return videos[0].Width, videos[0].Height
	}
	return 0, 0
}

func frameRate(info media.Info) float64 {
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		return videos[0].FrameRate
	}
	return 0
}
