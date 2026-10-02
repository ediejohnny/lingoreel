package spdev

import (
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/infrastructure/windows/keyboard"
	"lingoreel/internal/infrastructure/windows/overlay"
	"lingoreel/internal/infrastructure/windows/playerwindow"
	"lingoreel/internal/presentation/ptbr"
)

// defaultLook draws the subtitles with the default profile's look.
func defaultLook() overlay.Look {
	return watch.LookOf(settings.DefaultProfile())
}

// openOverlay puts the subtitle window over a player's own window; its
// errors without a caller go to stderr.
var openOverlay = overlay.Opener(defaultLook, stderrLogger())

// openPlayerWindow opens the app's player window, as the app does, for the
// players that draw into it.
var openPlayerWindow = playerwindow.Opener(defaultLook, playerwindow.Labels{
	Tip: ptbr.BarTips(keyboard.Char), Item: ptbr.BarItem, Time: ptbr.BarTime,
}, ptbr.WindowTitle, stderrLogger())
