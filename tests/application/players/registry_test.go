package players_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
)

// stubPlugin describes a player and records launches.
type stubPlugin struct {
	descriptor player.Descriptor
	launched   []string
	err        error
}

func (s *stubPlugin) Descriptor() player.Descriptor { return s.descriptor }

func (s *stubPlugin) Launch(_ context.Context, executable string, _ player.LaunchRequest) (players.Running, error) {
	s.launched = append(s.launched, executable)
	return nil, s.err
}

func stub(id player.ID, name string) *stubPlugin {
	return &stubPlugin{descriptor: player.Descriptor{
		ID: id, Name: name,
		Install: player.Install{DownloadURL: "https://example.org/" + string(id), Benefit: "Um bom player."},
	}}
}

func TestRegistryRefusesBrokenPlugins(t *testing.T) {
	t.Parallel()
	bad := stub("vlc", "VLC")
	bad.descriptor.Install.DownloadURL = "http://example.org"
	for name, list := range map[string][]players.Plugin{
		"invalid descriptor": {bad},
		"same id twice":      {stub("vlc", "VLC"), stub("vlc", "Outro VLC")},
		"nil plugin":         {stub("vlc", "VLC"), nil},
	} {
		if _, err := players.NewRegistry(list...); !errors.Is(err, player.ErrInvalidDescriptor) {
			t.Errorf("%s: NewRegistry = %v, want ErrInvalidDescriptor", name, err)
		}
	}
}

func TestRegistryFindsAndLaunchesItsPlugins(t *testing.T) {
	t.Parallel()
	vlc, mpv := stub("vlc", "VLC"), stub("mpv", "mpv")
	registry, err := players.NewRegistry(vlc, mpv)
	if err != nil {
		t.Fatal(err)
	}
	if !registry.Supported("vlc") || !registry.Supported("mpv") || registry.Supported("potplayer") {
		t.Fatal("Supported does not follow the registered plugins")
	}
	if got := registry.Descriptors(); len(got) != 2 || got[0].ID != "vlc" || got[1].ID != "mpv" {
		t.Fatalf("Descriptors = %+v, want vlc then mpv", got)
	}
	if _, err := registry.Launch(t.Context(), player.Installation{ID: "mpv", Path: `C:\mpv\mpv.exe`}, player.LaunchRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(mpv.launched) != 1 || mpv.launched[0] != `C:\mpv\mpv.exe` || len(vlc.launched) != 0 {
		t.Fatalf("launches: vlc %v, mpv %v; want only mpv at its path", vlc.launched, mpv.launched)
	}
	if _, err := registry.Launch(t.Context(), player.Installation{ID: "potplayer", Path: `C:\p.exe`}, player.LaunchRequest{}); !errors.Is(err, players.ErrUnknownPlayer) {
		t.Fatalf("launching a player without a plugin = %v, want ErrUnknownPlayer", err)
	}
	mpv.err = errors.New("boom")
	if _, err := registry.Launch(t.Context(), player.Installation{ID: "mpv"}, player.LaunchRequest{}); err == nil {
		t.Fatal("the plugin's launch error was lost")
	}
}
