package spdev

import (
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/infrastructure/windows/overlay"
)

// openOverlay puts the subtitle window over the player with the default
// profile's look; its errors without a caller go to stderr.
var openOverlay = overlay.Opener(func() overlay.Look {
	profile := settings.DefaultProfile()
	return overlay.Look{Layout: profile.Layout, Primary: profile.Primary, Translation: profile.Translation}
}, stderrLogger())
