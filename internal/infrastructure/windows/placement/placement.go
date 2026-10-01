//go:build windows

// Package placement decides where a player's window opens.
package placement

import (
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/windows/win32"
)

// ForVideo opens the player inside the primary monitor's work area, so a
// 1080p movie on a 900-pixel screen does not start past its edges; the user
// resizes it or goes fullscreen from there. A zero placement leaves it to
// the player.
func ForVideo(info media.Info) player.Placement {
	area, err := win32.WorkArea()
	if err != nil {
		return player.Placement{}
	}
	width, height := VideoSize(info)
	fit := layout.FitWindow(width, height, layout.Rect{
		X: float64(area.Left), Y: float64(area.Top),
		W: float64(area.Right - area.Left), H: float64(area.Bottom - area.Top),
	}, layout.InitialWindowFraction)
	return player.Placement{X: int(fit.X), Y: int(fit.Y), Width: int(fit.W), Height: int(fit.H)}
}

// VideoSize is the first video track's size, or zeros when unknown.
func VideoSize(info media.Info) (int, int) {
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		return videos[0].Width, videos[0].Height
	}
	return 0, 0
}
