//go:build windows && amd64

package overlay

import (
	"context"
	"log/slog"

	"lingoreel/internal/application/players"
	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/infrastructure/windows/placement"
)

// Look is how the subtitles are drawn.
type Look = session.Look

// The overlay changes its look while it shows.
var _ session.Restyler = (*Overlay)(nil)

// Opener returns what a watch session uses to put the overlay over its
// player, drawing with the look look returns at that moment.
func Opener(look func() Look, logger *slog.Logger) func(context.Context, players.Running, media.Info) (session.Display, error) {
	return func(ctx context.Context, running players.Running, info media.Info) (session.Display, error) {
		current := look()
		options := Options{
			ProcessID: running.ProcessID(), Layout: current.Layout,
			Primary: current.Primary, Translation: current.Translation, Logger: logger,
			PreviewColor: current.PreviewColor, FillColor: current.FillColor,
		}
		options.VideoWidth, options.VideoHeight = placement.VideoSize(info)
		// A plugin that knows its player's windows names them itself; the
		// others leave the largest window of the process to be found.
		if windowed, ok := running.(players.Windowed); ok {
			options.Window, options.Area = windowed.Window, windowed.VideoArea
		}
		screen, err := Open(ctx, options)
		if err != nil {
			return nil, err // never a nil *Overlay inside a non-nil Display
		}
		return screen, nil
	}
}
