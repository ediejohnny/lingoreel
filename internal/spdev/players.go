package spdev

import (
	"log/slog"
	"os"
	"strings"

	"lingoreel/internal/application/players"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/vlc"
)

// playerPlugin is a player plugin that also tells where its player is
// usually installed.
type playerPlugin interface {
	players.Plugin
	Hints() discovery.Hints
}

// plugins are the players spdev drives, one line each; a new player is one
// more line here (docs/plugins-de-player.md).
func plugins() []playerPlugin {
	return []playerPlugin{
		vlc.Plugin{Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))},
	}
}

// newRegistry registers the plugins and returns where their players live.
func newRegistry() (*players.Registry, []discovery.Hints, error) {
	list := plugins()
	asPlugins := make([]players.Plugin, len(list))
	hints := make([]discovery.Hints, len(list))
	for i, plugin := range list {
		asPlugins[i], hints[i] = plugin, plugin.Hints()
	}
	registry, err := players.NewRegistry(asPlugins...)
	if err != nil {
		return nil, nil, err
	}
	return registry, hints, nil
}

// supportedNames lists the players with a plugin, as "VLC, mpv".
func supportedNames(registry *players.Registry) string {
	var names []string
	for _, descriptor := range registry.Descriptors() {
		names = append(names, descriptor.Name)
	}
	return strings.Join(names, ", ")
}
