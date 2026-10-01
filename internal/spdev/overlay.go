package spdev

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/windows/overlay"
	"lingoreel/internal/infrastructure/windows/win32"
)

// openOverlay puts the subtitle window over the player, with the default
// profile's look; its errors without a caller go to stderr.
func openOverlay(ctx context.Context, running runningPlayer, info media.Info) (session.Display, error) {
	profile := settings.DefaultProfile()
	options := overlay.Options{
		ProcessID: running.ProcessID(),
		Layout:    profile.Layout, Primary: profile.Primary, Translation: profile.Translation,
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	options.VideoWidth, options.VideoHeight = videoSize(info)
	screen, err := overlay.Open(ctx, options)
	if err != nil {
		return nil, err // never a nil *Overlay inside a non-nil Display
	}
	return screen, nil
}

// videoSize is the first video track's size, or zeros when unknown.
func videoSize(info media.Info) (int, int) {
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		return videos[0].Width, videos[0].Height
	}
	return 0, 0
}

// initialWindow opens the player inside the primary monitor's work area, so
// a 1080p movie on a 900-pixel screen does not start past its edges; the user
// resizes it or goes fullscreen from there.
func initialWindow(info media.Info) player.Placement {
	area, err := win32.WorkArea()
	if err != nil {
		return player.Placement{} // the player's own choice
	}
	width, height := videoSize(info)
	fit := layout.FitWindow(width, height, layout.Rect{
		X: float64(area.Left), Y: float64(area.Top),
		W: float64(area.Right - area.Left), H: float64(area.Bottom - area.Top),
	}, layout.InitialWindowFraction)
	return player.Placement{X: int(fit.X), Y: int(fit.Y), Width: int(fit.W), Height: int(fit.H)}
}

// screenOf shows both lines; simultaneous events stack, one per line.
func screenOf(visible session.Visible) session.Screen {
	return session.Screen{
		Primary:            stackText(visible.Primary),
		Translation:        stackText(visible.Translation),
		TranslationVisible: true,
	}
}

func stackText(events []subtitle.Event) string {
	texts := make([]string, 0, len(events))
	for _, event := range events {
		texts = append(texts, event.Text())
	}
	return strings.Join(texts, "\n")
}
