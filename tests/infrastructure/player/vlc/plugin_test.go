//go:build windows

package vlc_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/vlc"
	"github.com/ediejohnny/lingoreel/tests/infrastructure/player/plugintest"
)

func TestVLCPluginDescriptor(t *testing.T) {
	plugintest.Descriptor(t, vlc.Plugin{})
}

// The VLC plugin passes the conformance kit every plugin must pass.
func TestVLCPluginPassesTheKit(t *testing.T) {
	executable, video := realVLC(t)
	plugintest.Launch(t, vlc.Plugin{}, executable, video)
}
