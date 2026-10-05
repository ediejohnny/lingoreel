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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
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

// syncCommands are the commands of SyncShortcuts, in the same order.
var syncCommands = []shortcut.Command{
	shortcut.SyncPrimaryEarlier, shortcut.SyncPrimaryLater,
	shortcut.SyncTranslationEarlier, shortcut.SyncTranslationLater,
}

// globalCommands are the commands registered system-wide while the player is
// in the foreground: the sync steps and the passage marks, which must work
// with a player that owns its window (PotPlayer) as well as with the app's.
var globalCommands = append(slices.Clone(syncCommands), shortcut.PassageMarkStart, shortcut.PassageMarkEnd)

// GlobalCommands lists the system-wide commands, in the order their presses
// refer to.
func GlobalCommands() []shortcut.Command {
	return slices.Clone(globalCommands)
}

// GlobalChords are the chords of GlobalCommands in a set of shortcuts.
func GlobalChords(keys shortcut.Map) []shortcut.Chord {
	chords := make([]shortcut.Chord, len(globalCommands))
	for i, command := range globalCommands {
		chords[i] = keys.Of(command)
	}
	return chords
}

// Press is one press of GlobalCommands()[Index]; Repeat tells the key is
// held.
type Press struct {
	Index  int
	Repeat bool
}

// Shortcuts delivers the presses of the global shortcuts while the player is
// in the foreground. The Windows hotkeys adapter implements it.
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
	// Mode is the study mode; when it always shows the translation, the
	// translation cannot be hidden.
	Mode() study.Mode
	// Run carries out a shortcut's command; repeat tells the key is held.
	Run(command shortcut.Command, repeat bool) error
	// Reveal is the translation key going down (pressed) or up; a held
	// key's repeats are not presses.
	Reveal(pressed bool)
	// Select picks a line on screen for editing; the video must be paused.
	Select(line session.Line) (Selection, error)
	// Edit changes the selected line, as the box over the video corrects
	// its text; the video must be paused.
	Edit(change EditChange) error

	// What the bar's menus show and change: the passage being marked or
	// studied and its options, the saved passages, the Modo Karaokê, the
	// study mode and the sync.
	MarkedStart() (time.Duration, bool)
	Passage() session.PracticeStatus
	PassageOptions() (study.PracticePlan, bool)
	SetPassageOptions(plan study.PracticePlan, snap bool) error
	SavedPassages() []SavedPassage
	StudySavedAt(index int) error
	Karaoke() session.Karaoke
	SetKaraokeSettings(karaoke session.Karaoke) error
	Indicator() study.Indicator
	SetIndicator(indicator study.Indicator) error
	LineRepeats() int
	SetLineRepeats(times int) error
	SetMode(mode study.Mode) error
	HasPrimary() bool
	ResetSync(line session.Line)
	// OpenEditor pauses, picks the line on screen and brings the app's
	// panel, where the editor is, to the front.
	OpenEditor(line session.Line) error
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
	// SetShortcuts changes the keys the window takes.
	SetShortcuts(keys shortcut.Map)
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
	// StartShortcuts listens to GlobalCommands, on the given chords, while
	// the player is in the foreground; nil leaves the shortcuts off.
	StartShortcuts func(target ShortcutTarget, chords []shortcut.Chord) (Shortcuts, error)
	// WindowHearsKeys tells that the window OpenWindow opens takes every key
	// itself, sync ones too (it is part of the app's window, whose panel
	// takes them as well): its sessions start no system-wide shortcuts,
	// which would take "<" from the panel's fields. A player in a window of
	// its own keeps them.
	WindowHearsKeys bool
	// PracticeNotice words what happens to a passage being studied for a
	// notice on the video; nil shows none.
	PracticeNotice func(session.PracticeEvent, session.PracticeStatus) string
	// MarkNotice words what a passage mark did for a notice on the video;
	// nil shows none.
	MarkNotice func(MarkEvent, study.Segment) string
	// VolumeNotice words the volume after a volume key or mute, with the
	// player's ceiling, for a notice on the video; nil shows none.
	VolumeNotice func(volume, ceiling playback.Volume, muted bool) string
	// ShowPanel brings the app's panel to the front, where the line editor
	// is; nil leaves it where it is.
	ShowPanel func()
	Now       func() time.Time
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
	// Mode is the study mode, which decides whether the translation starts
	// hidden; empty is ModeDual.
	Mode study.Mode
	// PrecisionStep is the jump of the precision shortcuts; zero is
	// DefaultPrecisionStep.
	PrecisionStep time.Duration
	// Saved is what the project keeps for each line: its offset and the
	// user's edits, applied when they still fit the file.
	Saved map[session.Line]project.Track
	// Karaoke is the Modo Karaokê to start with, its indicator and how many
	// times each line plays again (0 is off).
	Karaoke     session.Karaoke
	Indicator   study.Indicator
	LineRepeats int
	// Speed is the playback speed to start with; zero keeps the player's.
	Speed playback.Speed
	// Shortcuts are the keys of the commands; Reveal is how the translation
	// key works.
	Shortcuts shortcut.Map
	Reveal    study.RevealBehavior
	// NoSyncNotice leaves out the notice of a sync change on the video.
	NoSyncNotice bool
	// Passage, when set, is studied as soon as the player shows the video:
	// the session opens at its start. Start is then ignored.
	Passage *PassageStart
	// PassagePlan is how a passage closed by the mark shortcut is studied,
	// until the user picks other options; the zero plan pauses at the end.
	PassagePlan study.PracticePlan
}

// PassageStart is a passage to study from the start of a session.
type PassageStart struct {
	Segment study.Segment
	Plan    study.PracticePlan
}

// Listener hears what happens in a session. Its methods run on the
// session's goroutines and must return quickly.
type Listener interface {
	// SubtitlesChanged reports new lines on screen at a playback position.
	SubtitlesChanged(position time.Duration, visible session.Visible)
	// Synced reports a line's new offset after a sync shortcut or a reset.
	Synced(line session.Line, offset subtitle.Offset)
	// Edited reports a line's edits, as the project keeps them, after an
	// edit, an undo or a redo.
	Edited(line session.Line, edits map[string]project.Edit)
	// PassageMarked reports a passage closed by the mark shortcut, already
	// being studied with plan, so it can be saved.
	PassageMarked(segment study.Segment, plan study.PracticePlan)
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
	mode               atomic.Value // study.Mode
	precision          atomic.Int64 // time.Duration
	lastVisible        atomic.Pointer[session.Visible]
	// showMu keeps the last visible lines and what the display shows in step:
	// a redraw always paints the newest lines, never ones a change replaced.
	showMu    sync.Mutex
	practice  *session.Practice
	indicator atomic.Value // study.Indicator
	// lap is how the current lap of a passage shows the lines (the ladder).
	lap atomic.Pointer[study.Lap]
	// keys are the shortcuts; reveal is how the translation key works;
	// quiet leaves out the sync notice.
	keys   atomic.Pointer[shortcut.Map]
	reveal atomic.Value // study.RevealBehavior
	quiet  atomic.Bool
	// restart starts the sync shortcuts again on other chords; shortcutsMu
	// guards shortcuts once the session runs, and ending, set when they
	// close for good.
	restart     func(chords []shortcut.Chord) (Shortcuts, error)
	shortcutsMu sync.Mutex
	ending      bool

	editor editing

	// marks is the passage start the mark shortcut set, and the plan a
	// passage it closes is studied with.
	marks      marking
	markNotice func(MarkEvent, study.Segment) string
	// volumeNotice words the volume for a notice; now is the session's clock.
	volumeNotice func(volume, ceiling playback.Volume, muted bool) string
	now          func() time.Time
	// saved are the video's saved passages, told by the app; showPanel
	// brings the app's panel up for the line editor.
	saved     atomic.Pointer[[]SavedPassage]
	showPanel func()

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	err      error
}

// ErrNoPlugin reports a player the registry cannot drive.
var ErrNoPlugin = errors.New("player not supported")

// Why a session did not start, for the app to word it.
var (
	// ErrSubtitles: the chosen subtitles could not be read.
	ErrSubtitles = errors.New("the subtitles could not be loaded")
	// ErrDisplay: the app's player window or the subtitle display did not open.
	ErrDisplay = errors.New("the subtitle display could not be opened")
	// ErrLaunch: the player did not start or did not answer in time.
	ErrLaunch = errors.New("the player did not start")
)

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
	mode := cmp.Or(request.Mode, study.ModeDual)
	if !mode.Valid() {
		return nil, fmt.Errorf("unknown study mode %q", mode)
	}
	if request.PrecisionStep < 0 {
		return nil, fmt.Errorf("negative precision step %v", request.PrecisionStep)
	}
	now := ports.Now
	if now == nil {
		now = time.Now
	}
	info := request.Analysis.Info
	lines := []uint64{request.Tracks.Primary, request.Tracks.Translation}
	loaded, err := ports.Service.LoadSubtitles(ctx, request.Analysis, lines, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSubtitles, err)
	}

	if request.Passage != nil {
		if err := request.Passage.Plan.Validate(); err != nil {
			return nil, err
		}
		// The video opens where the passage starts, so the first lap is not
		// a seek away from what the player shows first.
		request.Start = request.Passage.Segment.Start
	}
	markPlan := request.PassagePlan
	if markPlan.Mode == "" {
		markPlan.Mode = study.EndPause
	}
	if err := markPlan.Validate(); err != nil {
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
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrLaunch, err), closeWindow(window))
	}

	s := &Session{
		running: running, listener: listener, loaded: loaded, notice: notice,
		clock:          session.NewClock(running.Capabilities().Precision, now),
		hasTranslation: request.Tracks.Translation != 0 && loaded[request.Tracks.Translation].Timeline != nil,
		done:           make(chan struct{}),
	}
	s.mode.Store(mode)
	s.translationVisible.Store(mode.StartsRevealed())
	s.precision.Store(int64(cmp.Or(request.PrecisionStep, DefaultPrecisionStep)))
	s.editor.history = project.NewHistory(project.DefaultHistoryLimit)
	var tracks [2]*session.Track
	for line, number := range map[session.Line]uint64{session.LinePrimary: request.Tracks.Primary, session.LineTranslation: request.Tracks.Translation} {
		if tracks[line], err = s.prepare(line, trackOf(loaded, number), request.Saved[line]); err != nil {
			return nil, errors.Join(err, running.Close(), closeWindow(window))
		}
	}
	s.subtitles = session.NewSubtitles(tracks[session.LinePrimary], tracks[session.LineTranslation])
	s.controls = session.NewControls(running, s.clock, frameRate(info))
	// A level that could not be synced leaves the bar at 100%, as before;
	// the next volume key sets the player's level anyway.
	_ = s.controls.SyncVolume()
	s.sync = session.NewSync(s.subtitles, request.Steps, now)
	s.subtitles.SetKaraoke(request.Karaoke)
	s.indicator.Store(cmp.Or(request.Indicator, study.IndicatorBar))
	s.practice = session.NewPractice(s.controls, s.subtitles, session.PracticeHooks{
		Event: s.practiced(ports.PracticeNotice), Look: s.setLap,
	}, now)
	if err := s.practice.SetLineRepeats(request.LineRepeats); err != nil {
		return nil, errors.Join(err, running.Close(), closeWindow(window))
	}
	if err := s.startSpeed(request.Speed); err != nil {
		return nil, errors.Join(err, running.Close(), closeWindow(window))
	}
	keys := request.Shortcuts.Resolved()
	s.keys.Store(&keys)
	s.reveal.Store(cmp.Or(request.Reveal, study.RevealToggle))
	s.quiet.Store(request.NoSyncNotice)
	s.markNotice, s.showPanel = ports.MarkNotice, ports.ShowPanel
	s.volumeNotice, s.now = ports.VolumeNotice, now
	s.marks.plan, s.marks.snap = markPlan, true

	target := ShortcutTarget{ProcessID: running.ProcessID()}
	switch {
	case window != nil:
		s.display, target.Window = window, window.Handle()
		window.SetShortcuts(keys)
	case ports.OpenDisplay != nil:
		if s.display, err = ports.OpenDisplay(ctx, running, info); err != nil {
			return nil, errors.Join(fmt.Errorf("%w: %w", ErrDisplay, err), running.Close())
		}
	}
	if ports.StartShortcuts != nil && !(window != nil && ports.WindowHearsKeys) {
		s.restart = func(chords []shortcut.Chord) (Shortcuts, error) { return ports.StartShortcuts(target, chords) }
		if s.shortcuts, err = s.restart(GlobalChords(keys)); err != nil {
			return nil, errors.Join(fmt.Errorf("starting the global shortcuts: %w", err), running.Close(), s.closeDisplay())
		}
		go s.listenShortcuts(s.shortcuts)
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
	if passage := request.Passage; passage != nil {
		// Saved passages were fitted to the lines when they were studied.
		if err := s.practice.Start(passage.Segment.FitTo(info.Duration), passage.Plan); err != nil {
			return nil, errors.Join(fmt.Errorf("studying the passage: %w", err), s.Stop())
		}
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
		return nil, fmt.Errorf("%w: opening the player window: %w", ErrDisplay, err)
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
	s.shortcutsMu.Lock()
	s.ending = true
	if s.shortcuts != nil {
		shortcutsErr = s.shortcuts.Close()
		s.shortcuts = nil
	}
	s.shortcutsMu.Unlock()
	// The player first: it may draw into the display's window.
	s.err = errors.Join(err, shortcutsErr, s.running.Close(), s.closeDisplay())
	close(s.done)
}

// changed runs on the runner's loop for every change on screen.
func (s *Session) changed(position time.Duration, visible session.Visible) {
	s.showMu.Lock()
	s.lastVisible.Store(&visible)
	s.show(visible)
	s.showMu.Unlock()
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

// listenShortcuts runs the presses of one set of global shortcuts until it
// is closed: at the end of the session, or when the chords change. A held
// key repeats only the commands made to repeat: holding Ctrl+F10 saves one
// passage, not one per repeat.
func (s *Session) listenShortcuts(shortcuts Shortcuts) {
	for press := range shortcuts.Presses() {
		if press.Index < 0 || press.Index >= len(globalCommands) {
			continue
		}
		command := globalCommands[press.Index]
		if press.Repeat && !command.Repeats() {
			continue
		}
		_ = s.Run(command, press.Repeat) // told on the video when it goes wrong
	}
}

func (s *Session) synced(line session.Line, offset subtitle.Offset) {
	if s.display != nil && s.notice != nil && !s.quiet.Load() {
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
// never moves either way. A mode that always shows it keeps it shown.
func (s *Session) SetTranslationVisible(visible bool) {
	if !visible && !s.Mode().Hideable() {
		return
	}
	s.translationVisible.Store(visible)
	s.redraw()
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

// prepare gives a line its saved offset and edits.
func (s *Session) prepare(line session.Line, track *session.Track, saved project.Track) (*session.Track, error) {
	if track == nil {
		return nil, nil
	}
	timeline, err := s.editor.prepareLine(line, track.Timeline, saved)
	if err != nil {
		return nil, fmt.Errorf("applying the saved edits: %w", err)
	}
	return &session.Track{Timeline: timeline, Offset: subtitle.NewOffset(saved.Offset.Duration())}, nil
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
