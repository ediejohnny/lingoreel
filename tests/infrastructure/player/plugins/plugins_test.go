package plugins_test

import (
	"testing"

	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/plugins"
)

func TestTheShippedPluginsRegister(t *testing.T) {
	registry, hints, err := plugins.Registry(nil)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	if !registry.Supported(player.VLC) {
		t.Fatal("VLC is not supported")
	}
	// Every plugin brings its hints, before the players without a plugin.
	if len(hints) != len(plugins.All(nil))+len(discovery.Known()) || hints[0].ID != player.VLC {
		t.Fatalf("hints = %d, first %q; want the plugins' first, then the known players", len(hints), hints[0].ID)
	}
	for _, plugin := range plugins.All(nil) {
		if plugin.Hints().ID != plugin.Descriptor().ID {
			t.Errorf("plugin %s gives hints for %s", plugin.Descriptor().ID, plugin.Hints().ID)
		}
	}
}
