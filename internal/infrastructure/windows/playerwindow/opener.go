//go:build windows && amd64

package playerwindow

import (
	"context"
	"log/slog"

	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/overlay"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/placement"
)

// Opener returns what a watch session uses to open its player window: the
// subtitles drawn with the look look returns at that moment, the bar worded
// by labels and the title worded by title.
func Opener(look func() overlay.Look, labels Labels, title func(video string) string, logger *slog.Logger) func(context.Context, watch.WindowRequest) (watch.Window, error) {
	return opener(nil, look, labels, title, logger)
}

// opener opens the window on a stage, or as a window of its own for none.
func opener(stage *Stage, look func() overlay.Look, labels Labels, title func(video string) string, logger *slog.Logger) func(context.Context, watch.WindowRequest) (watch.Window, error) {
	return func(ctx context.Context, request watch.WindowRequest) (watch.Window, error) {
		options := Options{
			Title: title(request.Title), Placement: request.Placement,
			Look: look(), Labels: labels, Logger: logger, Stage: stage,
		}
		options.VideoWidth, options.VideoHeight = placement.VideoSize(request.Info)
		window, err := Open(ctx, options)
		if err != nil {
			return nil, err // never a nil *Window inside a non-nil watch.Window
		}
		return window, nil
	}
}
