package lrdev

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/onnx"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// The align check runs a CTC speech model (wav2vec2 exported to ONNX) over
// a film's lines through ONNX Runtime, places each line's words in its
// sound, and tells how fast and how sure it was. The model's folder holds
// the model, its vocab.json and a meta.json naming its blank and word
// delimiter tokens and whether the sound is normalized.

// ctcFrame is how much sound one output of a wav2vec2 model covers: its
// convolutions step 320 samples at 16 kHz.
const ctcFrame = 20 * time.Millisecond

// markup is what a line's text carries besides its words: ASS overrides
// ({\i1}) and HTML-like tags (<i>).
var markup = regexp.MustCompile(`\{[^}]*\}|<[^>]*>`)

type ctcMeta struct {
	Pad         string `json:"pad_token"`
	Delimiter   string `json:"word_delimiter_token"`
	DoNormalize bool   `json:"do_normalize"`
}

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
	alphabet, meta, err := readCTCModel(*folder)
	if err != nil {
		return err
	}
	dll, err := filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}
	started := time.Now()
	runtime, err := onnx.Load(dll)
	if err != nil {
		return err
	}
	defer runtime.Close()
	session, err := runtime.Open(filepath.Join(*folder, *file), "input_values", "logits", *threads)
	if err != nil {
		return err
	}
	defer session.Close()
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
	out.line("Runtime:   ONNX Runtime %s, %s, %d threads, loaded in %d ms", runtime.Version(), *file, *threads, loaded.Milliseconds())
	var heard, spent time.Duration
	var words, placed int
	var confidences []float64
	done := 0
	for _, event := range subtitles[number].Timeline.Events() {
		if done >= *count || ctx.Err() != nil {
			break
		}
		text := strings.Fields(markup.ReplaceAllString(strings.ReplaceAll(event.Text(), `\N`, " "), " "))
		if len(text) == 0 {
			continue
		}
		stretch := voice.Stretch{File: path, Track: position, Channels: track.Channels, Codec: track.Codec,
			From: max(event.Start-*margin, 0), Length: event.End - event.Start + 2**margin}
		sound, err := decodeMono(ctx, decoder, stretch)
		if err != nil {
			return err
		}
		if meta.DoNormalize {
			normalize(sound)
		}
		began := time.Now()
		logits, shape, err := session.Run(sound, []int64{1, int64(len(sound))})
		took := time.Since(began)
		if err != nil {
			return err
		}
		if len(shape) != 3 || shape[2] != int64(alphabet.Size()) {
			return fmt.Errorf("align-check: the model answered %v, not frames of %d tokens", shape, alphabet.Size())
		}
		rows := make([][]float32, shape[1])
		for i := range rows {
			rows[i] = logits[i*int(shape[2]) : (i+1)*int(shape[2])]
		}
		found, err := alphabet.Align(rows, ctcFrame, text)
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

// readCTCModel reads a model folder's vocabulary and meta.
func readCTCModel(folder string) (speech.Alphabet, ctcMeta, error) {
	var meta ctcMeta
	var vocabulary map[string]int
	for name, into := range map[string]any{"meta.json": &meta, "vocab.json": &vocabulary} {
		data, err := readBounded(filepath.Join(folder, name), 1<<20)
		if err != nil {
			return speech.Alphabet{}, meta, err
		}
		if err := json.Unmarshal(data, into); err != nil {
			return speech.Alphabet{}, meta, fmt.Errorf("align-check: %s: %w", name, err)
		}
	}
	alphabet, err := speech.NewAlphabet(vocabulary, meta.Pad, meta.Delimiter)
	return alphabet, meta, err
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("%s is larger than %d bytes", path, limit)
	}
	return data, err
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

// normalize gives the sound zero mean and unit variance, as wav2vec2's
// feature extractor does.
func normalize(sound []float32) {
	mean := 0.0
	for _, s := range sound {
		mean += float64(s)
	}
	mean /= float64(len(sound))
	variance := 0.0
	for _, s := range sound {
		variance += (float64(s) - mean) * (float64(s) - mean)
	}
	scale := 1 / math.Sqrt(variance/float64(len(sound))+1e-7)
	for i, s := range sound {
		sound[i] = float32((float64(s) - mean) * scale)
	}
}

func quantileOf(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(int(q*float64(len(sorted))), len(sorted)-1)]
}
