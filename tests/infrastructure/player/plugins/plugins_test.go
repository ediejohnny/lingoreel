package plugins_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/plugins"
)

func TestTheShippedPluginsRegister(t *testing.T) {
	registry, hints, err := plugins.Registry(nil)
	if err != nil {
		t.Fatalf("Registry: %v", err)
	}
	for _, id := range []player.ID{player.VLC, player.PotPlayer} {
		if !registry.Supported(id) {
			t.Fatalf("%s is not supported", id)
		}
		// A player with a plugin is no longer one of the "coming soon".
		for _, hints := range discovery.Known() {
			if hints.ID == id {
				t.Fatalf("%s has a plugin; its hints belong to the plugin", id)
			}
		}
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
