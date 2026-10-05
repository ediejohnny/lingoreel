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
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Events sent to the window.
const (
	EventVideoDropped = "video:dropped"
	EventPlayback     = "playback:state"
	EventEnded        = "playback:ended"
	EventInstall      = "player:install"
)

// UI is what the app needs from the window toolkit, so the API can be tested
// without a window.
type UI interface {
	Emit(event string, data any)
	// OpenVideoDialog asks for a video file; empty when cancelled.
	OpenVideoDialog() (string, error)
	// OpenProgramDialog asks for a program (.exe); empty when cancelled.
	OpenProgramDialog(title string) (string, error)
	// SaveSubtitleDialog asks where to export a subtitle, suggesting a file
	// name; empty when cancelled.
	SaveSubtitleDialog(name string) (string, error)
	OpenURL(url string)
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
}

// Previewer draws sample subtitles for the settings screen and lists the
// fonts they may use.
type Previewer interface {
	Preview(look session.Look, width, height, dpi int, screen session.Screen) ([]byte, error)
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
	// Projects keeps each video's project; nil keeps nothing between runs.
	// SaveDelay is how long after a change the waiting saves happen (the
	// offsets, the position, the speed); zero is 2 s.
	Projects  Projects
	SaveDelay time.Duration
	// About is the app's and the system's version, for the diagnostics;
	// Monitors lists the screens (nil lists none); LogDir holds the logs.
	About    About
	Monitors func() ([]Monitor, error)
	LogDir   string
	// Associations lets Windows open videos with the app; nil offers none.
	Associations Associations
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
	initial  string
	// resume is where a session the player lost had reached, for
	// "Assistir de novo".
	resume *resumePoint
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
var ErrNotReady = errors.New("A janela do app ainda está abrindo. Tente de novo em instantes.")

// New creates the API and its lifecycle.
func New(deps Deps) (*API, Lifecycle) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	api := &API{
		deps: deps, logger: logger, initial: deps.InitialVideo,
		config: settings.DefaultConfig(), profiles: settings.DefaultProfiles(),
	}
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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx, a.ui = ctx, ui
	if a.deps.Settings == nil {
		return
	}
	if config, err := a.deps.Settings.LoadConfig(); err != nil {
		a.logger.Warn("settings could not be read; using the defaults", "error", err)
	} else {
		a.config = config
	}
	if profiles, err := a.deps.Settings.LoadProfiles(); err != nil {
		a.logger.Warn("profiles could not be read; using the defaults", "error", err)
	} else {
		a.profiles = profiles
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
	defer a.mu.Unlock()
	if a.config.Window == bounds {
		return
	}
	next := a.config
	next.Window = bounds
	if err := a.commitConfig(next); err != nil {
		a.logger.Warn("the window's place was not saved", "error", err)
	}
}

// Shutdown ends a running session when the window closes.
func (l Lifecycle) Shutdown() {
	// The window is closing: a failure to end the session has no one to tell.
	_ = l.api.StopWatching()
}

func (a *API) emit(event string, data any) {
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui != nil {
		ui.Emit(event, data)
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

// saveConfig writes the configuration; a failure is logged and reported.
func (a *API) saveConfig() error {
	if a.deps.Settings == nil {
		return nil
	}
	if err := a.deps.Settings.SaveConfig(a.config); err != nil {
		a.logger.Warn("settings could not be saved", "error", err)
		return errors.New("Não foi possível salvar as configurações.")
	}
	return nil
}

func (a *API) saveProfiles() error {
	if a.deps.Settings == nil {
		return nil
	}
	if err := a.deps.Settings.SaveProfiles(a.profiles); err != nil {
		a.logger.Warn("profiles could not be saved", "error", err)
		return errors.New("Não foi possível salvar o perfil.")
	}
	return nil
}
