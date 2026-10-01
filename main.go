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
	"lingoreel/internal/infrastructure/media/matroska"
	"lingoreel/internal/infrastructure/persistence/settingsfile"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/install"
	"lingoreel/internal/infrastructure/player/plugins"
	"lingoreel/internal/infrastructure/subtitlefile"
	"lingoreel/internal/infrastructure/windows/hotkeys"
	"lingoreel/internal/infrastructure/windows/overlay"
	"lingoreel/internal/infrastructure/windows/placement"
	"lingoreel/internal/presentation/app"
)

//go:embed all:frontend/dist
var assets embed.FS

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
	look := func() overlay.Look {
		profile := activeProfile(files)
		return overlay.Look{Layout: profile.Layout, Primary: profile.Primary, Translation: profile.Translation}
	}
	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	deps := app.Deps{
		Service: service, Registry: registry,
		Detect: func(configured map[player.ID]string) []player.Installation {
			return discovery.Detect(discovery.SystemEnvironment{}, hints, configured)
		},
		Ports: watch.Ports{
			Service: service, Registry: registry,
			OpenDisplay: overlay.Opener(look, logger),
			StartShortcuts: func(processID int) (watch.Shortcuts, error) {
				return hotkeys.StartSync(processID, logger)
			},
		},
		Settings:  files,
		Installer: install.Winget{},
		Window:    placement.ForVideo,
		Logger:    logger,
	}
	if len(os.Args) > 1 {
		deps.InitialVideo = os.Args[1]
	}
	api, lifecycle := app.New(deps)

	return wails.Run(&options.App{
		Title: "LingoReel", Width: windowWidth, Height: windowHeight,
		MinWidth: windowMinWidth, MinHeight: windowMinHeight,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			lifecycle.Attach(ctx, app.NewWailsUI(ctx))
			runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { lifecycle.Dropped(paths) })
		},
		OnShutdown:  func(context.Context) { lifecycle.Shutdown() },
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Bind:        []any{api},
		Windows:     &windows.Options{Theme: windows.SystemDefault},
	})
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
