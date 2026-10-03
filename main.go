// Command LingoReel is the desktop app: it opens a video in the user's
// player and draws two subtitles over it. This file only wires the pieces.
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/infrastructure/media/matroska"
	"lingoreel/internal/infrastructure/persistence/projectfile"
	"lingoreel/internal/infrastructure/persistence/settingsfile"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/install"
	"lingoreel/internal/infrastructure/player/plugins"
	"lingoreel/internal/infrastructure/subtitlefile"
	"lingoreel/internal/infrastructure/windows/appwindow"
	"lingoreel/internal/infrastructure/windows/hotkeys"
	"lingoreel/internal/infrastructure/windows/keyboard"
	"lingoreel/internal/infrastructure/windows/overlay"
	"lingoreel/internal/infrastructure/windows/placement"
	"lingoreel/internal/infrastructure/windows/playerwindow"
	"lingoreel/internal/infrastructure/windows/preview"
	"lingoreel/internal/presentation/app"
	"lingoreel/internal/presentation/ptbr"
)

//go:embed all:frontend/dist
var assets embed.FS

// windowTitle names the app window; it also finds it to keep its place.
const windowTitle = "LingoReel"

// Window sizes of the app, in device-independent pixels.
const (
	windowWidth     = 1024
	windowHeight    = 720
	windowMinWidth  = 760
	windowMinHeight = 540
)

// logFileName keeps the last session's log next to the settings, for
// problem reports; it never holds subtitle text.
const logFileName = "lingoreel.log"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "LingoReel:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := settingsfile.Dir()
	if err != nil {
		return err
	}
	logger, closeLog := openLog(dir)
	defer closeLog()

	registry, hints, err := plugins.Registry(logger)
	if err != nil {
		return err
	}
	files := settingsfile.Open(dir)
	look := func() overlay.Look { return watch.LookOf(activeProfile(files)) }
	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	deps := app.Deps{
		Service: service, Registry: registry,
		Detect: func(configured map[player.ID]string) []player.Installation {
			return discovery.Detect(discovery.SystemEnvironment{}, hints, configured)
		},
		Ports: watch.Ports{
			Service: service, Registry: registry,
			OpenWindow: playerwindow.Opener(look, playerwindow.Labels{
				Tip: ptbr.BarTips(keyboard.Char), Item: ptbr.BarItem, Time: ptbr.BarTime,
			}, ptbr.WindowTitle, logger),
			OpenDisplay:    overlay.Opener(look, logger),
			PracticeNotice: ptbr.PracticeNotice,
			StartShortcuts: func(target watch.ShortcutTarget, chords []shortcut.Chord) (watch.Shortcuts, error) {
				return hotkeys.StartSync(target, chords, logger)
			},
		},
		Settings:    files,
		Installer:   install.Winget{},
		Window:      placement.ForVideo,
		Logger:      logger,
		KeyChar:     keyboard.Char,
		SettingsDir: dir,
		OpenFolder:  openFolder,
		Projects:    projectfile.Open(dir),
	}
	if len(os.Args) > 1 {
		deps.InitialVideo = os.Args[1]
	}
	samples, err := preview.Start()
	if err != nil {
		logger.Warn("the settings preview is off", "error", err)
	} else {
		deps.Preview = samples
		defer samples.Close()
	}
	api, lifecycle := app.New(deps)

	return wails.Run(&options.App{
		Title: windowTitle, Width: windowWidth, Height: windowHeight,
		MinWidth: windowMinWidth, MinHeight: windowMinHeight,
		// Hidden until it is back where the user left it.
		StartHidden: true,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			lifecycle.Attach(ctx, app.NewWailsUI(ctx))
			runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { lifecycle.Dropped(paths) })
			restoreWindow(ctx, lifecycle, logger)
		},
		OnBeforeClose: func(context.Context) bool {
			if hwnd, err := appwindow.Find(windowTitle); err == nil {
				if bounds, err := appwindow.Bounds(hwnd); err == nil {
					lifecycle.SaveWindow(bounds)
				}
			}
			return false
		},
		OnShutdown:  func(context.Context) { lifecycle.Shutdown() },
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Bind:        []any{api},
		Windows:     &windows.Options{Theme: windows.SystemDefault},
	})
}

// restoreWindow puts the app window back where it was when it closed and
// shows it; without a saved place, or with it off every monitor, the window
// shows where Wails opens it.
func restoreWindow(ctx context.Context, lifecycle app.Lifecycle, logger *slog.Logger) {
	defer runtime.WindowShow(ctx)
	bounds := lifecycle.WindowBounds()
	hwnd, err := appwindow.Find(windowTitle)
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

// openLog writes the log of this run to the settings folder; without it,
// the app runs with no log.
func openLog(dir string) (*slog.Logger, func()) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}
	}
	file, err := os.Create(filepath.Join(dir, logFileName))
	if err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}
	}
	return slog.New(slog.NewTextHandler(file, nil)), func() { file.Close() }
}
