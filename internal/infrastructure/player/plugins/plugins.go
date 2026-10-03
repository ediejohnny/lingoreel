// Package plugins lists the player plugins the app ships with. A new player
// is one more line in All (see docs/plugins-de-player.md).
package plugins

import (
	"log/slog"

	"lingoreel/internal/application/players"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/potplayer"
	"lingoreel/internal/infrastructure/player/vlc"
)

// Plugin is a player plugin that also tells where its player is usually
// installed.
type Plugin interface {
	players.Plugin
	Hints() discovery.Hints
}

// All returns the plugins, in registration order; logger receives the
// players' events that have no caller.
func All(logger *slog.Logger) []Plugin {
	return []Plugin{
		vlc.Plugin{Logger: logger},
		potplayer.Plugin{Logger: logger},
	}
}

// Registry registers the plugins and returns where to look for every known
// player: the plugins' hints first, then the players without a plugin.
func Registry(logger *slog.Logger) (*players.Registry, []discovery.Hints, error) {
	list := All(logger)
	asPlugins := make([]players.Plugin, len(list))
	hints := make([]discovery.Hints, 0, len(list))
	for i, plugin := range list {
		asPlugins[i] = plugin
		hints = append(hints, plugin.Hints())
	}
	registry, err := players.NewRegistry(asPlugins...)
	if err != nil {
		return nil, nil, err
	}
	return registry, append(hints, discovery.Known()...), nil
}
