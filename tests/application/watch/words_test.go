package watch_test

import (
	"context"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// lateLineReader is the rig's film with its line after 6 s of silence, long
// enough for the karaoke mode to show it ahead of time.
type lateLineReader struct{ fakeReader }

func (lateLineReader) ReadSubtitles(context.Context, string, []uint64, func(int64, int64)) (map[uint64]media.SubtitleContent, error) {
	return map[uint64]media.SubtitleContent{
		4: {Events: []subtitle.Event{{ID: "e1", Start: 6 * time.Second, End: 8 * time.Second, OriginalText: "Hello there."}}},
		5: {Events: []subtitle.Event{{ID: "p1", Start: 6 * time.Second, End: 8 * time.Second, OriginalText: "Olá."}}},
	}, nil
}

// quietDecoder decodes a film of silence, a minute long.
type quietDecoder struct{}

func (quietDecoder) Decode(ctx context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	end := min(stretch.From+stretch.Length, time.Minute)
	piece := make([]float32, speech.SampleRate/10)
	for at := stretch.From; at < end && ctx.Err() == nil; at += 100 * time.Millisecond {
		hear(piece, 1)
	}
	return ctx.Err()
}

// earlyWords places "Hello" from 5.7 s, before the line shows at 6 s, in the
// sound handed from the margin before the line.
type earlyWords struct{}

func (earlyWords) Align([]float32, []string) ([]speech.Word, error) {
	from := 6*time.Second - speech.LineMargin
	return []speech.Word{
		{Span: speech.Span{Start: 5700*time.Millisecond - from, End: 5900*time.Millisecond - from}, Heard: true, Confidence: 0.9},
		{Span: speech.Span{Start: 6000*time.Millisecond - from, End: 6300*time.Millisecond - from}, Heard: true, Confidence: 0.9},
	}, nil
}
func (earlyWords) Close() {}

// An actor who begins before the line's time ends its preview then: with the
// line's words placed in the sound, the bar is gone at 5.8 s and the line is
// said, its fill going, though it shows from 6 s; before the voice, the
// preview counts down.
func TestThePreviewEndsAsTheLinesFirstWordIsSaid(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	service := analysis.NewService(lateLineReader{}, nil)
	result, err := service.Analyze(t.Context(), `C:\v.mkv`, media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	r.ports.Service, r.request.Analysis, r.request.Tracks = service, result, result.Suggested
	r.ports.Model = testkit.LoudModel{}
	r.ports.FindDecoder = func(context.Context, string) (voice.Decoder, error) { return quietDecoder{}, nil }
	r.ports.OpenWords = func(string) (voice.WordModel, error) { return earlyWords{}, nil }
	r.request.Karaoke = session.Karaoke{Enabled: true, Rules: study.CueRules{MinGap: 3 * time.Second, Lead: 2 * time.Second}, Fill: true}
	r.request.Voice = watch.VoiceSetup{Model: true, Follow: true}
	r.request.Tracks.Audio = 3
	r.start(t)

	r.running.samples <- playback.Sample{Position: 5800 * time.Millisecond, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	testkit.Eventually(t, "the line said at 5.8 s, its preview over", func() bool {
		screen, _, _, _ := r.display.last()
		return screen.Cue == nil && screen.Primary == "Hello there." && screen.Fill > 0
	})
	r.running.samples <- playback.Sample{Position: 5 * time.Second, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	testkit.Eventually(t, "the preview at 5 s, before the voice", func() bool {
		screen, _, _, _ := r.display.last()
		return screen.Cue != nil && screen.Primary == "Hello there."
	})
}
