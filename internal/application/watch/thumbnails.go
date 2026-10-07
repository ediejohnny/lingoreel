package watch

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// A video in the app's own window gets stills for its seek bar: taken in the
// background while it plays, the moment under the pointer first, until the
// session ends (which never waits for them).

// startStills begins the video's stills when the app's window shows it and
// stills can be taken; the film's FFmpeg is the one chosen for the voice.
func (s *Session) startStills(ports Ports, request Request, info media.Info) {
	if ports.OpenStills == nil {
		return
	}
	chosen := request.Voice.FFmpegPath
	open := func(ctx context.Context, file string) (thumbnail.Film, error) {
		return ports.OpenStills(ctx, chosen, file)
	}
	duration := func() time.Duration {
		if info.Duration > 0 {
			return info.Duration
		}
		return s.State().Duration // MP4, AVI: the player tells
	}
	s.stills = thumbnail.Start(request.Analysis.Path, open, duration, thumbnail.Options{OnStop: ports.StillsStopped})
	s.stills.Rest(ports.Power != nil && ports.Power.OnBattery())
}

// Thumbnail is what the seek bar shows for a moment under the pointer.
func (s *Session) Thumbnail(at time.Duration) thumbnail.View { return s.stills.Thumbnail(at) }

// WantThumbnail tells the moment the pointer is on, whose still goes first.
func (s *Session) WantThumbnail(at time.Duration) { s.stills.Want(at) }
