// Package app is the Go side of the app window: the API the screens call and
// the events they receive. It only translates between the screens and the use
// cases (analysis, watch, players, settings); the rules live below it.
package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/fault"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Events sent to the window.
const (
	EventVideoDropped = "video:dropped"
	EventPlayback     = "playback:state"
	EventEnded        = "playback:ended"
	EventInstall      = "player:install"
	// EventFullscreen tells the video went fullscreen (true) or left it.
	EventFullscreen = "window:fullscreen"
	// EventPanel asks the screens to open the panel beside the video on a
	// section (PanelRequest).
	EventPanel = "panel:open"
)

// UI is what the app needs from the window toolkit, so the API can be tested
// without a window.
type UI interface {
	Emit(event string, data any)
	// OpenVideoDialog asks for a video file; empty when cancelled.
	OpenVideoDialog() (string, error)
	// OpenSubtitleDialog asks for a subtitle file, from anywhere; empty when
	// cancelled.
	OpenSubtitleDialog() (string, error)
	// OpenProgramDialog asks for a program (.exe); empty when cancelled.
	OpenProgramDialog(title string) (string, error)
	// SaveSubtitleDialog asks where to export a subtitle, suggesting a file
	// name; empty when cancelled.
	SaveSubtitleDialog(name string) (string, error)
	OpenURL(url string)
	// CopyText puts a text on the clipboard.
	CopyText(text string) error
	// SetTitle names the window: the video playing, or the app.
	SetTitle(title string)
	// SetAlwaysOnTop keeps the window in front of the other programs', or
	// lets it go behind them.
	SetAlwaysOnTop(on bool)
	// HideWindow takes the window off the screen at once; the app keeps
	// running until it quits.
	HideWindow()
	// Quit closes the app.
	Quit()
}

// Stage is the place for the video inside the app's window: whether the
// screen shows it, the room the screen's controls take around it, and the
// fullscreen.
type Stage interface {
	Place(visible bool, insets layout.Insets)
	SetFullscreen(on bool)
	// SetChrome changes how the controls over the video look: the bar, the
	// box for corrections, and the theme the menu over the video follows.
	SetChrome(bar settings.BarPreferences, edit appearance.EditStyle, theme settings.Theme)
}

// Projects keeps each video's project: the user's corrections, the offsets,
// the last choices and the saved passages. TrackKey names a track so the
// name survives a new analysis of the video.
type Projects interface {
	Load(video string) (project.Project, error)
	Save(video string, p project.Project) error
	// Remove forgets a video's project wherever it is kept.
	Remove(video string) error
	TrackKey(video string, track media.Track) string
	// AddedPath is how the project writes a subtitle file picked for the
	// video; ResolveAdded is the file such a path names.
	AddedPath(video, path string) string
	ResolveAdded(video, added string) string
}

// Previewer draws sample subtitles for the settings screen, with a sample
// notice when it is not empty, and lists the fonts they may use.
type Previewer interface {
	Preview(look session.Look, width, height, dpi int, screen session.Screen, notice string) ([]byte, error)
	// PreviewBar draws the control bar in a style at the bottom of a sample
	// picture, with its speed list open; clock is its time text, and speed
	// words a speed.
	PreviewBar(style appearance.BarStyle, width, height, dpi int, clock string, speed func(playback.Speed) string) ([]byte, error)
	Fonts() ([]string, error)
}

// Settings stores the preferences.
type Settings interface {
	LoadConfig() (settings.Config, error)
	SaveConfig(settings.Config) error
	LoadProfiles() (settings.Profiles, error)
	SaveProfiles(settings.Profiles) error
}

// Installer installs players.
type Installer interface {
	Available() bool
	Install(ctx context.Context, packageID string) error
}

// Deps are the pieces the composition root wires in.
type Deps struct {
	Service  *analysis.Service
	Registry *players.Registry
	// Detect lists the players on the computer; configured are the paths
	// the user set for some of them.
	Detect    func(configured map[player.ID]string) []player.Installation
	Ports     watch.Ports
	Settings  Settings
	Installer Installer
	// FFmpeg finds the user's FFmpeg, for the karaoke mode to decode the
	// actors' voice ahead and for the seek bar's stills; nil offers none.
	FFmpeg FFmpeg
	// Stills are the seek bar's stills kept on the disk, for the settings to
	// tell their size and clear them; nil tells nothing.
	Stills StillsCache
	// Faults tells why writing, renaming or removing a file failed, so the
	// message can say it and what to do; nil leaves the messages without it.
	Faults func(error) fault.Kind
	// Window places the player's window for a video; nil leaves it to the player.
	Window func(info media.Info) player.Placement
	// InitialVideo is the file the app was opened with, if any.
	InitialVideo string
	Logger       *slog.Logger
	// Preview draws the settings screen's sample; nil shows none.
	Preview Previewer
	// KeyChar names the keys with the user's keyboard layout; nil uses the
	// US characters.
	KeyChar ptbr.KeyChar
	// SettingsDir is where the settings live; OpenFolder shows a folder in
	// the file explorer.
	SettingsDir string
	OpenFolder  func(path string) error
	// WriteSubtitle writes an exported subtitle to the file the user picked,
	// in the format its name asks for, and returns the path written; nil
	// exports nothing.
	WriteSubtitle func(path string, events []subtitle.Event) (string, error)
	// Projects keeps each video's project; nil keeps nothing between runs.
	// SaveDelay is how long after a change the waiting saves happen (the
	// offsets, the position, the speed); zero is 2 s. Later runs them after
	// that delay and returns what stops them; nil is time.AfterFunc.
	Projects  Projects
	SaveDelay time.Duration
	Later     func(delay time.Duration, run func()) (stop func() bool)
	// About is the app's and the system's version, for the diagnostics;
	// Monitors lists the screens (nil lists none); LogDir holds the logs.
	About    About
	Monitors func() ([]Monitor, error)
	LogDir   string
	// Associations lets Windows open videos with the app; nil offers none.
	Associations Associations
	// SystemLanguage is the Windows display language, for the interface to
	// speak when none is chosen and for the first profile; nil speaks the
	// default.
	SystemLanguage func() string
	// FirstRun tells the app opens for the first time: the welcome asks what
	// the person studies before anything else.
	FirstRun bool
	// Machine reads the computer's hardware, once, in the background: its
	// level decides what the first run starts with, and a computer of
	// another level later is asked about. Nil reads nothing: the computer is
	// medium and never asked about.
	Machine func() capacity.Machine
	// SpeechPacks are the languages' speech packs kept in the app's folder;
	// nil keeps none.
	SpeechPacks SpeechPacks
	// OpenSpeech opens a language's speech pack, to hear which language an
	// audio track speaks; nil hears none.
	OpenSpeech func(language string) (SpeechModel, error)
	// AskRetry asks, with Windows' own box, whether to try again a save that
	// failed as the app closes; nil never asks.
	AskRetry func(title, text string) bool
	// Stage puts the video inside the app's window, for players that draw
	// into one (it opens Ports.OpenWindow's windows); nil leaves each video
	// in a window of its own.
	Stage Stage
}

// Associations writes and removes the app's place in Windows' "Abrir com"
// and in the videos' context menu.
type Associations interface {
	Enabled() (bool, error)
	Enable() error
	Disable() error
}

// About names the versions the diagnostics show.
type About struct {
	Version string
	Windows string
}

// Monitor is a screen as the diagnostics show it.
type Monitor struct {
	Width, Height, X, Y int
	DPI                 int
	Primary             bool
}

// API is bound to the window: every exported method is callable from the
// screens.
type API struct {
	deps   Deps
	logger *slog.Logger

	mu       sync.Mutex
	ctx      context.Context
	ui       UI
	config   settings.Config
	profiles settings.Profiles
	video    *analysis.Analysis
	project  *videoProject
	watching *watching
	// listen is the hearing of the open video's voice, nil when none runs.
	listen *listening
	// hearLanguage is the hearing of the open video's untold audio tracks,
	// nil when none runs.
	hearLanguage *languageHearing
	initial      string
	// resume is where a session the player lost had reached, for
	// "Assistir de novo".
	resume *resumePoint
	// closing is set while a session ends before the window closes; shutDown
	// once the window's message loop is gone, when the window is not to be
	// touched any more.
	closing, shutDown bool
	// adding lets one subtitle file be added at a time, so two never replace
	// each other's analysis.
	adding sync.Mutex
	// projectWrites lets one project write run at a time, outside a.mu, and
	// settingsWrites one settings change (changeSettings); neither is ever
	// taken with a.mu held.
	projectWrites, settingsWrites sync.Mutex
	// welcoming tells the first run's welcome is not answered yet.
	welcoming bool
	// machine is the computer as read, once; computerChanged tells the
	// question of a computer of another level waits for an answer.
	machine         func() capacity.Machine
	computerChanged bool
	// settingsNotes tell, in the language of the moment, why a settings file
	// could not be read and what becomes of it.
	settingsNotes []func() string
}

type resumePoint struct {
	video string
	at    time.Duration
}

// Lifecycle connects the API to the window. It is kept apart from the API so
// the screens cannot call it.
type Lifecycle struct {
	api *API
}

// ErrNotReady reports a call before the window started.
var ErrNotReady = worded(func() string { return ptbr.T("error.notReady") })

var (
	errSettingsNotSaved = worded(func() string { return ptbr.T("error.settings.notSaved") })
	errProfilesNotSaved = worded(func() string { return ptbr.T("error.profile.notSaved") })
)

// message is an error the user reads, worded each time it is shown so it is
// in the interface's language; each one is a value of its own for errors.Is.
type message struct{ text func() string }

func (m *message) Error() string { return m.text() }

// worded makes an error of a catalog text.
func worded(text func() string) error { return &message{text: text} }

// reasoned is a failed write's message followed by why it failed and what
// to do; errors.Is still finds the message, and the cause for another
// message that wraps this one.
type reasoned struct {
	message, cause error
	kind           fault.Kind
}

func (r *reasoned) Error() string { return r.message.Error() + " " + ptbr.FaultReason(r.kind) }

func (r *reasoned) Unwrap() []error { return []error{r.message, r.cause} }

// because adds to message, a failed write's, why err happened when the
// system told something that helps; otherwise message stands alone.
func (a *API) because(message, err error) error {
	if a.deps.Faults == nil {
		return message
	}
	kind := a.deps.Faults(err)
	if ptbr.FaultReason(kind) == "" {
		return message
	}
	return &reasoned{message: message, cause: err, kind: kind}
}

// New creates the API and its lifecycle.
func New(deps Deps) (*API, Lifecycle) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	api := &API{
		deps: deps, logger: logger, initial: deps.InitialVideo, welcoming: deps.FirstRun,
		config: settings.DefaultConfig(), profiles: settings.DefaultProfiles(),
	}
	measure := deps.Machine
	if measure == nil {
		measure = func() capacity.Machine { return capacity.Machine{} }
	}
	api.machine = sync.OnceValue(measure)
	return api, Lifecycle{api: api}
}

// InitialVideo returns, once, the video the app was opened with (a double
// click on a video file); empty otherwise.
func (a *API) InitialVideo() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	path := a.initial
	a.initial = ""
	return path
}

// Attach starts the API for a window: it loads the settings, keeping the
// defaults when the files are damaged (the problem is logged, never fatal).
func (l Lifecycle) Attach(ctx context.Context, ui UI) {
	a := l.api
	// The computer is met in the background once the settings are loaded
	// (the deferred calls run in turn: a.mu is let go first).
	defer func() { go a.meetComputer() }()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx, a.ui = ctx, ui
	if a.deps.Settings == nil {
		return
	}
	if config, err := a.deps.Settings.LoadConfig(); err != nil {
		a.logger.Warn("settings could not be read; using the defaults", "error", err)
		a.settingsNotes = append(a.settingsNotes, unreadNote("config.json", err))
	} else {
		a.config = config
	}
	if a.deps.Stage != nil {
		a.deps.Stage.SetChrome(a.config.Bar, a.config.Edit, a.config.Theme)
	}
	if profiles, err := a.deps.Settings.LoadProfiles(); err != nil {
		a.logger.Warn("profiles could not be read; using the defaults", "error", err)
		a.settingsNotes = append(a.settingsNotes, unreadNote("profiles.json", err))
	} else {
		a.profiles = profiles
	}
}

// unreadNote tells why a settings file could not be read: one written by a
// newer version or one Windows would not open is left as it is, and the
// changes last until the app closes; a damaged one is copied aside before
// the first save over it.
func unreadNote(file string, err error) func() string {
	switch {
	case errors.Is(err, settings.ErrNewerVersion):
		return func() string { return ptbr.T("settings.file.newer", "file", file) }
	case errors.Is(err, settings.ErrUnreadable):
		return func() string { return ptbr.T("settings.file.unreadable", "file", file) }
	default:
		return func() string { return ptbr.T("settings.file.damaged", "file", file) }
	}
}

// Dropped handles files dropped on the window: the first one goes to the
// screens, which analyze it.
func (l Lifecycle) Dropped(paths []string) {
	if len(paths) == 0 {
		return
	}
	l.api.emit(EventVideoDropped, paths[0])
}

// VideoArgument finds the video in a command line's arguments (without the
// program): the first one that is not an option, made absolute against dir.
// Windows passes the file of a double click, "Abrir com" or the context menu
// this way.
func VideoArgument(args []string, dir string) (string, bool) {
	for _, arg := range args {
		arg = strings.Trim(strings.TrimSpace(arg), `"`)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if !filepath.IsAbs(arg) {
			arg = filepath.Join(dir, arg)
		}
		return filepath.Clean(arg), true
	}
	return "", false
}

// WindowBounds is where the app's window was when it last closed.
func (l Lifecycle) WindowBounds() settings.WindowBounds {
	l.api.mu.Lock()
	defer l.api.mu.Unlock()
	return l.api.config.Window
}

// SaveWindow keeps where the app's window is as it closes; a failure is
// only logged, the window is closing.
func (l Lifecycle) SaveWindow(bounds settings.WindowBounds) {
	a := l.api
	a.mu.Lock()
	same := a.config.Window == bounds
	a.mu.Unlock()
	if same {
		return
	}
	if err := a.changeConfig(func(next *settings.Config) error {
		next.Window = bounds
		return nil
	}); err != nil {
		a.logger.Warn("the window's place was not saved", "error", err)
	}
}

// Shutdown ends a running session when the window closes.
func (l Lifecycle) Shutdown() {
	// Closing the app during the welcome skips it: the next opening starts
	// with what Windows tells.
	if err := l.api.SkipWelcome(); err != nil {
		l.api.logger.Warn("the welcome was not skipped", "error", err)
	}
	l.api.mu.Lock()
	l.api.shutDown = true
	l.api.mu.Unlock()
	// The window is closing: a failure to end the session has no one to tell.
	_ = l.api.StopWatching()
	l.api.flushProjectAtExit()
}

// EndBeforeClosing ends a running session before the window closes, so a
// video drawn inside the window never outlives the window's message loop;
// then it calls quit. It reports whether the window must wait for that
// (true), or may close now: no session (false). The window is hidden at
// once, so the app looks closed while the player quits and the project is
// saved behind it.
func (l Lifecycle) EndBeforeClosing(quit func()) bool {
	a := l.api
	a.mu.Lock()
	running := a.watching != nil
	if running && a.closing {
		a.mu.Unlock()
		return true // asked again while the session ends
	}
	a.closing = running
	a.mu.Unlock()
	if !running {
		return false
	}
	a.ui.HideWindow()
	go func() {
		// The window is closing: a failure to end the session has no one to tell.
		_ = a.StopWatching()
		quit()
	}()
	return true
}

// FullscreenChanged tells the screens the video went fullscreen or left
// it, whatever asked for it, so they hide or show their controls.
func (l Lifecycle) FullscreenChanged(on bool) {
	l.api.emit(EventFullscreen, on)
}

// ShowEditor opens the panel beside the video on the line editor: the bar's
// "Correct the study subtitle…" has picked the line.
// KeepOnTop puts the window in front of the other programs' when the
// settings ask for it; showing the window (Wails' WindowShow) leaves that
// place, so this follows every show.
func (l Lifecycle) KeepOnTop() {
	l.api.keepOnTop()
}

func (l Lifecycle) ShowEditor() {
	l.api.emit(EventPanel, PanelRequest{Section: PanelEditor})
}

// window is the window the API is attached to, nil before Attach.
func (a *API) window() UI {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ui
}

func (a *API) emit(event string, data any) {
	ui := a.window()
	if ui != nil {
		ui.Emit(event, data)
	}
}

// setTitle names the window, unless it is gone.
func (a *API) setTitle(title string) {
	a.mu.Lock()
	ui, gone := a.ui, a.shutDown
	a.mu.Unlock()
	if ui != nil && !gone {
		ui.SetTitle(title)
	}
}

// context returns the window's context, or ErrNotReady before Attach.
func (a *API) context() (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		return nil, ErrNotReady
	}
	return a.ctx, nil
}

// profile is the active study profile.
func (a *API) profile() settings.Profile {
	if profile, found := a.profiles.Find(a.config.ActiveProfile); found {
		return profile
	}
	return settings.DefaultProfile()
}
