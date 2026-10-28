// Command LingoReel is the desktop app: it opens a video in the user's
// player and draws two subtitles over it. This file only wires the pieces.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	sys "golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/buildinfo"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/diskfault"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/download"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/logfile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/seekable"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/projectfile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/settingsfile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/stillcache"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/install"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/plugins"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
	packs "github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/appwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/association"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/audiotap"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/hotkeys"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/keyboard"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/memory"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/placement"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/power"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/preview"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/processor"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

//go:embed all:frontend/dist
var assets embed.FS

// windowTitle names the app window until a video plays in it.
const windowTitle = "LingoReel"

// windowClass is the app window's own class: it finds the window to keep
// its place and to draw the video in it, whatever its title says.
const windowClass = "LingoReelApp"

// Window sizes of the app, in device-independent pixels.
const (
	windowWidth     = 1024
	windowHeight    = 720
	windowMinWidth  = 760
	windowMinHeight = 540
)

// singleInstanceID names the app for Windows, so a second launch finds the
// first one.
const singleInstanceID = "lingoreel-edie-johnny"

// logFolder holds the daily logs next to the settings, for problem reports;
// they never hold subtitle text.
const logFolder = "logs"

// thumbnailsFolder holds the films' seek-bar stills, next to the settings.
const thumbnailsFolder = "thumbnails"

// speechPacks is where the languages' speech packs are published, with the
// version that first carried them.
const speechPacks = "https://github.com/ediejohnny/lingoreel/releases/download/v0.29.0/"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "LingoReel:", err)
		// A window app has no console: Windows' own box tells it, in the
		// Windows display language when the app had not chosen one yet.
		if !spoken.Load() {
			_, _ = ptbr.UseLanguage("", systemLanguage())
		}
		win32.Tell(ptbr.StartupError(err))
		os.Exit(1)
	}
}

// spoken tells the interface's language was chosen.
var spoken atomic.Bool

func run() error {
	dir, err := settingsfile.Dir()
	if err != nil {
		return err
	}
	logger, closeLog, logDir := openLog(filepath.Join(dir, logFolder))
	defer closeLog()
	logger.Info("starting", "version", buildinfo.Version, "windows", windowsVersion())
	defer logger.Info("closed")

	registry, hints, err := plugins.Registry(logger)
	if err != nil {
		return err
	}
	files := settingsfile.Open(dir)
	firstRun := files.FirstRun()
	stills := stillcache.Cache{Dir: filepath.Join(dir, thumbnailsFolder)}
	go stills.Sweep() // packages a closing left half written
	// The interface speaks its language from the first text on; a
	// configuration that does not load is reported by the app later.
	config, _ := files.LoadConfig()
	speakLanguage(config.Language, logger)
	look := func() overlay.Look { return watch.LookOf(activeProfile(files)) }
	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	// started is the window's context, for a second launch and the player
	// window's menus to reach it; lifecycle is set once the API exists.
	var started atomic.Pointer[context.Context]
	var lifecycle app.Lifecycle
	fetchPack := packs.Fetcher(download.Client{})
	voicePacks := speechpack.New(speechpack.Options{
		Dir: dir, Disk: packs.Disk{}, Base: speechPacks, Extras: packs.Extras,
		Fetch: func(ctx context.Context, url string, file speech.File, dest string, progress func(done, total int64)) error {
			err := fetchPack(ctx, url, file, dest, progress)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("speech pack file not fetched", "file", file.Name, "error", err)
			}
			return err
		},
		// Asked for only from the window, once the lifecycle is set.
		Changed: func() { lifecycle.SpeechPacksChanged() },
	})
	defer voicePacks.Close()
	// The video plays inside the app's own window: one window for all.
	stage := playerwindow.NewStage(playerwindow.StageOptions{
		Parent: func() uintptr {
			hwnd, err := appwindow.Find(windowClass)
			if err != nil {
				return 0
			}
			return uintptr(hwnd)
		},
		OnFullscreen: func(on bool) { lifecycle.FullscreenChanged(on) },
		App:          func() playerwindow.AppActions { return lifecycle },
	})
	deps := app.Deps{
		Service: service, Registry: registry,
		Detect: func(configured map[player.ID]string) []player.Installation {
			return discovery.Detect(discovery.SystemEnvironment{}, hints, configured)
		},
		Ports: watch.Ports{
			Service: service, Registry: registry,
			OpenWindow: stage.Opener(look, playerwindow.Labels{
				Tip: ptbr.BarTips(keyboard.Char), Item: ptbr.BarItem, Time: ptbr.BarTime,
				Entry: ptbr.BarEntries(keyboard.Char), Status: ptbr.BarStatus, Moment: ptbr.Clock,
				EditHint: ptbr.EditHint, EditEmpty: ptbr.EditEmpty, Menu: ptbr.MenuTexts(keyboard.Char),
			}, ptbr.WindowTitle, logger),
			WindowHearsKeys: true,
			OpenDisplay:     overlay.Opener(look, logger),
			PracticeNotice:  ptbr.PracticeNotice,
			MarkNotice:      ptbr.MarkNotice,
			VolumeNotice:    ptbr.VolumeNotice,
			// The line editor is on the panel beside the video; with a player
			// in a window of its own, the app's window comes to the front.
			ShowPanel: func() {
				stage.SetFullscreen(false)
				lifecycle.ShowEditor()
				if ctx := started.Load(); ctx != nil {
					showWindow(*ctx, lifecycle)
				}
			},
			StartShortcuts: func(target watch.ShortcutTarget, chords []shortcut.Chord) (watch.Shortcuts, error) {
				return hotkeys.StartGlobal(target, chords, logger)
			},
			Model:       silero.Embedded{},
			Power:       power.Status{},
			Tap:         audiotap.Tap{},
			FindDecoder: findFFmpeg,
			OpenStills:  openStills,
			StillsStopped: func(err error) {
				logger.Info("the seek bar's stills stopped", "reason", err)
			},
			// The stills stay in the app's own folder, never beside the film.
			KeepStills: stills,
			StillsUnkept: func(err error) {
				logger.Warn("keeping the seek bar's stills", "err", err)
			},
		},
		AskRetry:      win32.AskRetry,
		Stage:         stagePort{stage},
		Settings:      files,
		Installer:     install.Winget{},
		FFmpeg:        userFFmpeg{},
		Stills:        stills,
		Faults:        diskfault.Of,
		Window:        placement.ForVideo,
		Logger:        logger,
		KeyChar:       keyboard.Char,
		SettingsDir:   dir,
		OpenFolder:    openFolder,
		WriteSubtitle: subtitleformat.WriteFile,
		Projects:      projectfile.Open(dir),
		About:         app.About{Version: buildinfo.Version, Windows: windowsVersion()},
		Monitors:      monitors,
		LogDir:        logDir,
		// The interface speaks Windows' language when none is chosen.
		SystemLanguage: systemLanguage,
		// Read before anything is saved: the first run opens the welcome.
		FirstRun:    firstRun,
		Machine:     func() capacity.Machine { return measureMachine(logger) },
		SpeechPacks: voicePacks,
		// Hearing an audio track's language is background work: one
		// processor, so the film keeps the rest.
		OpenSpeech: func(language string) (app.SpeechModel, error) {
			runtime, model, ok := voicePacks.Paths(language)
			if !ok {
				return nil, fmt.Errorf("no speech pack for %q", language)
			}
			return packs.Open(runtime, model, 1)
		},
	}
	if executable, err := os.Executable(); err == nil {
		deps.Associations = association.Registrar{
			Base: `Software\Classes`, Executable: executable, Extensions: analysis.VideoExtensions(),
			Names: func() association.Names {
				kind, menu := ptbr.AssociationNames()
				return association.Names{Type: kind, Menu: menu}
			},
		}
	}
	if workingDir, err := os.Getwd(); err == nil {
		deps.InitialVideo, _ = app.VideoArgument(os.Args[1:], workingDir)
	}
	samples, err := preview.Start()
	if err != nil {
		logger.Warn("the settings preview is off", "error", err)
	} else {
		deps.Preview = samples
		defer samples.Close()
	}
	var api *app.API
	api, lifecycle = app.New(deps)

	return wails.Run(&options.App{
		Title: windowTitle, Width: windowWidth, Height: windowHeight,
		MinWidth: windowMinWidth, MinHeight: windowMinHeight,
		// Hidden until it is back where the user left it.
		StartHidden: true,
		AssetServer: &assetserver.Options{Assets: assets},
		// Opening a video with the app already open brings this window up and
		// opens the video here, instead of a second app.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID,
			OnSecondInstanceLaunch: func(second options.SecondInstanceData) {
				logger.Info("opened again", "args", len(second.Args))
				if ctx := started.Load(); ctx != nil {
					showWindow(*ctx, lifecycle)
				}
				if video, found := app.VideoArgument(second.Args, second.WorkingDirectory); found {
					lifecycle.Dropped([]string{video})
				}
			},
		},
		OnStartup: func(ctx context.Context) {
			started.Store(&ctx)
			lifecycle.Attach(ctx, app.NewWailsUI(ctx))
			runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { lifecycle.Dropped(paths) })
			restoreWindow(ctx, lifecycle, logger)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			// A video playing in the window ends first, then the app quits and
			// comes back here with nothing playing.
			if lifecycle.EndBeforeClosing(func() { runtime.Quit(ctx) }) {
				return true
			}
			if hwnd, err := appwindow.Find(windowClass); err == nil {
				if bounds, err := appwindow.Bounds(hwnd); err == nil {
					lifecycle.SaveWindow(bounds)
				}
			}
			return false
		},
		OnShutdown:  func(context.Context) { lifecycle.Shutdown() },
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Bind:        []any{api},
		Windows: &windows.Options{
			Theme: windows.SystemDefault, WindowClassName: windowClass,
			// The screen measures the video's place in its own pixels.
			DisablePinchZoom: true,
			Messages:         webViewMessages(),
		},
	})
}

// webViewMessages word, in the interface's language, what Windows shows
// when the WebView2 runtime the window needs is missing or fails.
func webViewMessages() *windows.Messages {
	texts := ptbr.WebViewMessages()
	return &windows.Messages{
		InstallationRequired: texts.InstallationRequired, UpdateRequired: texts.UpdateRequired,
		MissingRequirements: texts.MissingRequirements, Webview2NotInstalled: texts.NotInstalled,
		Error: texts.Error, FailedToInstall: texts.FailedToInstall, DownloadPage: texts.DownloadPage,
		PressOKToInstall: texts.PressOKToInstall, ContactAdmin: texts.ContactAdmin,
		InvalidFixedWebview2: texts.InvalidFixed, WebView2ProcessCrash: texts.Crashed,
	}
}

// stagePort places the video on the stage for the app.
type stagePort struct{ *playerwindow.Stage }

func (s stagePort) Place(visible bool, insets layout.Insets) {
	s.SetLayout(playerwindow.StageLayout{Visible: visible, Insets: insets})
}

func (s stagePort) SetChrome(bar settings.BarPreferences, edit appearance.EditStyle, theme settings.Theme) {
	shade := map[settings.Theme]playerwindow.Theme{settings.ThemeLight: playerwindow.ThemeLight, settings.ThemeDark: playerwindow.ThemeDark}[theme]
	s.Stage.SetChrome(playerwindow.Chrome{Bar: bar.Style, HideDelay: bar.HideDelay.Std(), Edit: edit, Theme: shade})
}

// showWindow brings the app window up, in front of the others when the
// settings say so: Wails' WindowShow leaves that place.
func showWindow(ctx context.Context, lifecycle app.Lifecycle) {
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	lifecycle.KeepOnTop()
}

// restoreWindow puts the app window back where it was when it closed and
// shows it; without a saved place, or with it off every monitor, the window
// shows where Wails opens it.
func restoreWindow(ctx context.Context, lifecycle app.Lifecycle, logger *slog.Logger) {
	// Deferred calls run last first: shown, then put in front.
	defer lifecycle.KeepOnTop()
	defer runtime.WindowShow(ctx)
	bounds := lifecycle.WindowBounds()
	hwnd, err := appwindow.Find(windowClass)
	if err != nil {
		logger.Warn("the app window was not found to restore it", "error", err)
		return
	}
	restored, err := appwindow.Restore(hwnd, bounds)
	if err != nil {
		logger.Warn("the app window was not restored", "error", err)
	}
	if restored && bounds.Maximized {
		runtime.WindowMaximise(ctx)
	}
}

// openFolder shows a folder in the file explorer.
func openFolder(path string) error {
	return exec.Command("explorer.exe", path).Start()
}

// systemLanguage is the Windows display language, empty when unknown.
func systemLanguage() string {
	if languages, err := sys.GetUserPreferredUILanguages(sys.MUI_LANGUAGE_NAME); err == nil && len(languages) > 0 {
		return languages[0]
	}
	return ""
}

// speakLanguage makes the interface speak the language configured, else the
// Windows display language, each when the app speaks it, else English.
func speakLanguage(configured string, logger *slog.Logger) {
	system := systemLanguage()
	language, err := ptbr.UseLanguage(configured, system)
	if err != nil {
		logger.Warn("the interface's catalog did not load", "error", err)
		return
	}
	spoken.Store(true)
	logger.Info("interface language", "language", language, "windows", system)
}

// measureMachine reads the computer's hardware: its processor, its logical
// processors and its memory. How busy it is now is left out: that passes.
func measureMachine(logger *slog.Logger) capacity.Machine {
	machine := capacity.Machine{Processor: processor.Name(), Processors: goruntime.NumCPU(), Memory: memory.Total()}
	logger.Info("computer", "processors", machine.Processors, "memoryGiB", machine.Identity().MemoryGiB, "level", machine.Level())
	return machine
}

// activeProfile is the profile the user studies with, or the default one.
func activeProfile(files *settingsfile.Files) settings.Profile {
	config, configErr := files.LoadConfig()
	profiles, profilesErr := files.LoadProfiles()
	if configErr == nil && profilesErr == nil {
		if profile, found := profiles.Find(config.ActiveProfile); found {
			return profile
		}
	}
	return settings.DefaultProfile()
}

// openLog writes the day's log in dir or, when dir cannot take it (a
// read-only or full disk), in the user's temporary folder; it returns the
// folder used, empty when neither could, and the app then runs with no log.
func openLog(dir string) (*slog.Logger, func(), string) {
	for _, folder := range []string{dir, filepath.Join(os.TempDir(), "LingoReel", logFolder)} {
		if file, err := logfile.Open(folder, time.Now); err == nil {
			return slog.New(slog.NewTextHandler(file, nil)), func() { file.Close() }, folder
		}
	}
	return slog.New(slog.DiscardHandler), func() {}, ""
}

// monitors lists the screens for the diagnostics.
func monitors() ([]app.Monitor, error) {
	found, err := win32.Monitors()
	if err != nil {
		return nil, err
	}
	list := make([]app.Monitor, len(found))
	for i, m := range found {
		list[i] = app.Monitor{
			Width: int(m.Bounds.Right - m.Bounds.Left), Height: int(m.Bounds.Bottom - m.Bounds.Top),
			X: int(m.Bounds.Left), Y: int(m.Bounds.Top), DPI: int(m.DPI), Primary: m.Primary,
		}
	}
	return list, nil
}

// windowsVersion is "10.0.19045" for the log.
func windowsVersion() string {
	v := sys.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

// userFFmpeg is the user's own FFmpeg for the settings screen, which tells
// one that did not answer in time apart from one that is missing.
type userFFmpeg struct{}

func (userFFmpeg) Find(ctx context.Context, chosen string) (string, error) {
	path, err := ffmpeg.Find(ctx, chosen)
	return path, slowOf(err)
}
func (userFFmpeg) Check(ctx context.Context, path string) error {
	return slowOf(ffmpeg.Check(ctx, path))
}
func (userFFmpeg) WingetID() string { return ffmpeg.WingetID }

func slowOf(err error) error {
	if errors.Is(err, ffmpeg.ErrSlow) {
		return fmt.Errorf("%w: %w", app.ErrFFmpegSlow, err)
	}
	return err
}

// findFFmpeg looks for the user's own FFmpeg, to decode the actors' voice
// ahead of the video; the app ships none.
func findFFmpeg(ctx context.Context, chosen string) voice.Decoder {
	path, err := ffmpeg.Find(ctx, chosen)
	if err != nil {
		return nil
	}
	return ffmpeg.Decoder{Path: path}
}

// openStills prepares a film for the seek bar's stills with the user's own
// FFmpeg; one that did not answer in time leaves the stills to be asked for
// again, one missing ends them.
func openStills(ctx context.Context, chosen, file string) (thumbnail.Film, error) {
	path, err := ffmpeg.Find(ctx, chosen)
	switch {
	case errors.Is(err, ffmpeg.ErrSlow):
		return nil, fmt.Errorf("%w: %w", thumbnail.ErrBusy, err)
	case err != nil:
		return nil, fmt.Errorf("%w: %w", thumbnail.ErrNoStills, err)
	}
	return ffmpeg.Stills{Path: path, Indexed: seekable.Indexed}.Open(ctx, file)
}
