package spdev

import (
	"context"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/overlay"
)

// openOverlay puts the subtitle window over the player, with the default
// profile's look.
func openOverlay(ctx context.Context, running runningPlayer, info media.Info) (*overlay.Overlay, error) {
	profile := settings.DefaultProfile()
	options := overlay.Options{
		ProcessID: running.ProcessID(),
		Layout:    profile.Layout, Primary: profile.Primary, Translation: profile.Translation,
	}
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		options.VideoWidth, options.VideoHeight = videos[0].Width, videos[0].Height
	}
	return overlay.Open(ctx, options)
}

// overlayContent shows both lines; simultaneous events stack, one per line.
func overlayContent(visible session.Visible) overlay.Content {
	return overlay.Content{
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
