package voice

import (
	"context"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// Hear decodes a stretch of a film's voice and has the model judge it, at
// the pace of decoding ahead, for choosing the subtitle that fits the film
// (subtitle.AlignToVoice). Like a window ahead, it starts aheadWarmUp before
// the stretch, so the model has heard what leads into it, and runs
// aheadTail past it; only the stretch's own frames come back. progress, when
// not nil, hears the share of the stretch done, from 0 to 1, as it goes.
func Hear(ctx context.Context, decoder Decoder, model speech.Model, stretch Stretch, progress func(done float64)) (speech.Track, error) {
	stream, err := model.NewStream()
	if err != nil {
		return speech.Track{}, err
	}
	decode := stretch
	decode.From = max(stretch.From-aheadWarmUp, 0)
	decode.Length = stretch.From + stretch.Length + aheadTail - decode.From
	var analyzer *speech.Analyzer
	var analyzerErr error
	var frames []speech.Frame
	paced := pace{duty: aheadDuty}
	err = decoder.Decode(ctx, decode, func(samples []float32, channels int) {
		began := time.Now()
		if analyzer == nil && analyzerErr == nil {
			analyzer, analyzerErr = speech.NewAnalyzer(channels, stream)
		}
		if analyzer != nil {
			frames = analyzer.Write(samples, frames)
		}
		paced.worked(ctx, time.Since(began))
		if progress != nil {
			progress(min(float64(time.Duration(len(frames))*speech.FrameLength)/float64(decode.Length), 1))
		}
	})
	if err == nil {
		err = analyzerErr
	}
	if err != nil {
		return speech.Track{}, err
	}
	if analyzer != nil {
		frames = analyzer.Flush(frames)
	}
	first := min(framesUntil(stretch.From-decode.From), len(frames))
	last := max(min(framesUntil(stretch.From+stretch.Length-decode.From), len(frames)), first)
	return speech.Track{Start: decode.From + time.Duration(first)*speech.FrameLength, Frames: frames[first:last]}, nil
}
