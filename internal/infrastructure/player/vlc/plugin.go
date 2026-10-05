package vlc

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
)

// Plugin drives VLC. It is the first player plugin and the example the guide
// for new plugins follows.
type Plugin struct {
	// Logger receives VLC events that have no caller; nil discards them.
	Logger *slog.Logger
}

var _ players.Plugin = Plugin{}

// Descriptor names VLC and tells how to install it.
func (Plugin) Descriptor() player.Descriptor {
	return player.Descriptor{
		ID:   player.VLC,
		Name: "VLC",
		Install: player.Install{
			WingetID:    "VideoLAN.VLC",
			DownloadURL: "https://www.videolan.org/vlc/download-windows.html",
			Benefit:     "O player mais popular: abre praticamente qualquer formato e é ótimo se você já usa.",
		},
		// VLC 3 ignores the close button of its own window: it must draw
		// into the app's, which closes like any other.
		DrawsInHost: true,
	}
}

// Hints tell where VLC is usually installed.
func (Plugin) Hints() discovery.Hints {
	return discovery.Hints{
		ID:          player.VLC,
		Executables: []string{"vlc.exe"},
		Registry: []discovery.RegistryValue{
			{Root: discovery.LocalMachine, Path: `SOFTWARE\VideoLAN\VLC`},
			{Root: discovery.LocalMachine, Path: `SOFTWARE\VideoLAN\VLC`, Name: "InstallDir", IsDirectory: true},
			{Root: discovery.LocalMachine, Path: `SOFTWARE\WOW6432Node\VideoLAN\VLC`, Name: "InstallDir", IsDirectory: true},
			{Root: discovery.LocalMachine, Path: discovery.AppPathsKey + "vlc.exe"},
		},
		ProgramFiles:  []string{`VideoLAN\VLC\vlc.exe`},
		UninstallName: regexp.MustCompile(`(?i)^VLC media player`),
	}
}

// Launch opens a video in VLC.
func (p Plugin) Launch(ctx context.Context, executable string, request player.LaunchRequest) (players.Running, error) {
	session, err := LaunchWith(ctx, executable, request, Options{Logger: p.Logger})
	if err != nil {
		return nil, err // never a nil *Session inside a non-nil Running
	}
	return session, nil
}
