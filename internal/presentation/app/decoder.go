package app

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
)

// findTimeout bounds each search for the user's FFmpeg before hearing.
const findTimeout = 10 * time.Second

// defaultBusyRest is how long hearing waits, after the computer was too busy
// to start FFmpeg, before looking for it again.
const defaultBusyRest = 30 * time.Second

// decoderFor is the user's FFmpeg for hearing a video. A computer too busy to
// start it is no FFmpeg missing: the hearing rests and looks again for as
// long as the video is open. Nil when there is none, or once the video
// closed; what names the hearing in the log.
func (a *API) decoderFor(ctx context.Context, ffmpegPath, what string) voice.Decoder {
	rest := cmp.Or(a.deps.BusyRest, defaultBusyRest)
	waiting := false
	for {
		found, cancel := context.WithTimeout(ctx, findTimeout)
		decoder, err := a.deps.Ports.FindDecoder(found, ffmpegPath)
		cancel()
		switch {
		case ctx.Err() != nil:
			return nil
		case err == nil && decoder != nil:
			return decoder
		case !errors.Is(err, voice.ErrDecoderBusy):
			a.logger.Info(what + " was not heard: no FFmpeg found")
			return nil
		}
		if !waiting {
			a.logger.Info(what+" waits: the computer is too busy to start FFmpeg", "rest", rest)
			waiting = true
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(rest):
		}
	}
}
