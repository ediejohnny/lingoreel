package lrdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// The align check runs a CTC speech model (wav2vec2 exported to ONNX) over
// a film's lines through ONNX Runtime, places each line's words in its
// sound, and tells how fast and how sure it was. The model's folder holds
// the model, its vocab.json and a meta.json naming its blank and word
// delimiter tokens and whether the sound is normalized.

func alignCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	flags := flag.NewFlagSet("align-check", flag.ContinueOnError)
	flags.SetOutput(stdout)
	runtimePath := flags.String("runtime", "", "the onnxruntime.dll to run the model with")
	folder := flags.String("model", "", "the model's folder (model, vocab.json, meta.json)")
	file := flags.String("file", "model.onnx", "the model's file inside its folder")
	threads := flags.Int("threads", 2, "how many processors the model runs on")
	label := flags.String("subtitle", "", "the subtitle whose lines are aligned, as inspect shows it (#3, #A1); the suggested top line by default")
	count := flags.Int("lines", 20, "how many lines, from the first spoken one")
	margin := flags.Duration("margin", 300*time.Millisecond, "sound taken before and after each line")
	tool := flags.String("ffmpeg", "", "the FFmpeg to decode with; found as the app finds it by default")
	showText := flags.Bool("text", false, "prints each line's words; only for a free film, never a private one")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *runtimePath == "" || *folder == "" {
		return errors.New("align-check: give --runtime and --model")
	}
	dll, err := filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}
	started := time.Now()
	model, err := speechpack.Open(dll, filepath.Join(*folder, *file), *threads)
	if err != nil {
		return err
	}
	defer model.Close()
	loaded := time.Since(started)

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, path, defaultPreferences())
	if err != nil {
		return err
	}
	number := result.Suggested.Primary
	if *label != "" {
		if number, err = parseTrackLabel(*label); err != nil {
			return err
		}
	}
	lines, found := result.Info.Track(number)
	if number == 0 || !found || lines.Type != media.TrackSubtitle {
		return errors.New("align-check: no subtitle to align; choose one with --subtitle")
	}
	subtitles, err := service.LoadSubtitles(ctx, result, []uint64{number}, nil)
	if err != nil {
		return err
	}
	track, position, found := media.VoiceTrack(result.Info, lines.Language)
	if !found {
		return errors.New("align-check: the video has no audio track to hear")
	}
	decoderPath, err := ffmpeg.Find(ctx, *tool)
	if err != nil {
		return err
	}
	decoder := ffmpeg.Decoder{Path: decoderPath}

	out := &printer{w: stdout}
	out.line("Runtime:   ONNX Runtime %s, %s, %d threads, loaded in %d ms", model.Version(), *file, *threads, loaded.Milliseconds())
	var heard, spent time.Duration
	var words, placed int
	var confidences []float64
	done := 0
	for _, event := range subtitles[number].Timeline.Events() {
		if done >= *count || ctx.Err() != nil {
			break
		}
		text := subtitle.SpokenWords(event.Text())
		if len(text) == 0 {
			continue
		}
		stretch := voice.Stretch{File: path, Track: position, Channels: track.Channels, Codec: track.Codec,
			From: max(event.Start-*margin, 0), Length: event.End - event.Start + 2**margin}
		sound, err := decodeMono(ctx, decoder, stretch)
		if err != nil {
			return err
		}
		began := time.Now()
		found, err := model.Align(sound, text)
		took := time.Since(began)
		if errors.Is(err, speech.ErrNothingToAlign) {
			continue
		}
		if err != nil {
			return err
		}
		done++
		length := time.Duration(len(sound)) * time.Second / speech.SampleRate
		heard += length
		spent += took
		line := fmt.Sprintf("line %3d  %s  sound %5d ms  model %4d ms", done, formatClock(event.Start), length.Milliseconds(), took.Milliseconds())
		var parts []string
		for i, w := range found {
			words++
			if !w.Heard {
				parts = append(parts, text[i]+"=?")
				continue
			}
			placed++
			confidences = append(confidences, w.Confidence)
			at := stretch.From + w.Start
			parts = append(parts, fmt.Sprintf("%s@%s+%d(%.2f)", text[i], formatClock(at), (w.End-w.Start).Milliseconds(), w.Confidence))
		}
		if *showText {
			line += "  " + strings.Join(parts, " ")
		}
		out.line("%s", line)
	}
	if done == 0 {
		out.line("No line was aligned.")
		return out.err
	}
	slices.Sort(confidences)
	out.line("")
	out.line("Lines %d, words %d, placed %d (%.0f%%), confidence median %.2f, p10 %.2f",
		done, words, placed, 100*float64(placed)/float64(words), quantileOf(confidences, 0.5), quantileOf(confidences, 0.1))
	out.line("Speed: %s of sound in %s of model time, real-time factor %.3f on %d threads",
		heard.Round(time.Millisecond), spent.Round(time.Millisecond), spent.Seconds()/heard.Seconds(), *threads)
	return out.err
}

// decodeMono decodes a stretch of sound at 16 kHz, its channels averaged.
func decodeMono(ctx context.Context, decoder ffmpeg.Decoder, stretch voice.Stretch) ([]float32, error) {
	var sound []float32
	err := decoder.Decode(ctx, stretch, func(samples []float32, channels int) {
		for i := 0; i+channels <= len(samples); i += channels {
			sum := float32(0)
			for _, s := range samples[i : i+channels] {
				sum += s
			}
			sound = append(sound, sum/float32(channels))
		}
	})
	if err == nil && len(sound) == 0 {
		err = errors.New("align-check: no sound decoded")
	}
	return sound, err
}

func quantileOf(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(int(q*float64(len(sorted))), len(sorted)-1)]
}
