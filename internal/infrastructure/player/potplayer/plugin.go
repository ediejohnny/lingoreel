package potplayer

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
)

// Plugin drives PotPlayer. It is the first plugin for a player that draws
// in its own window instead of the app's.
type Plugin struct {
	// Logger receives PotPlayer events that have no caller; nil discards them.
	Logger *slog.Logger
}

var _ players.Plugin = Plugin{}

// Descriptor names PotPlayer and tells how to install it.
func (Plugin) Descriptor() player.Descriptor {
	return player.Descriptor{
		ID:   player.PotPlayer,
		Name: "PotPlayer",
		Install: player.Install{
			WingetID:    "Daum.PotPlayer",
			DownloadURL: "https://potplayer.tv/",
			Benefit:     "Rápido e muito usado no Windows, com boa aceleração por hardware.",
		},
		DrawsInHost: false,
		// PotPlayer has no message for the audio track: it stays the one
		// PotPlayer picks, changeable in its own menu.
		FixedAudioTrack: true,
	}
}

// Hints tell where PotPlayer is usually installed.
func (Plugin) Hints() discovery.Hints {
	return discovery.Hints{
		ID:          player.PotPlayer,
		Executables: []string{"PotPlayerMini64.exe", "PotPlayerMini.exe"},
		Registry: []discovery.RegistryValue{
			{Root: discovery.CurrentUser, Path: `SOFTWARE\DAUM\PotPlayer64`, Name: "ProgramPath"},
			{Root: discovery.CurrentUser, Path: `SOFTWARE\DAUM\PotPlayer`, Name: "ProgramPath"},
			{Root: discovery.LocalMachine, Path: discovery.AppPathsKey + "PotPlayerMini64.exe"},
		},
		ProgramFiles:  []string{`DAUM\PotPlayer\PotPlayerMini64.exe`, `PotPlayer\PotPlayerMini64.exe`, `DAUM\PotPlayer\PotPlayerMini.exe`},
		UninstallName: regexp.MustCompile(`(?i)^PotPlayer`),
	}
}

// Launch opens a video in PotPlayer.
func (p Plugin) Launch(ctx context.Context, executable string, request player.LaunchRequest) (players.Running, error) {
	session, err := LaunchWith(ctx, executable, request, Options{Logger: p.Logger})
	if err != nil {
		return nil, err // never a nil *Session inside a non-nil Running
	}
	return session, nil
}
