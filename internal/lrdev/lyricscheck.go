package lrdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	cappedfile "github.com/ediejohnny/lingoreel/internal/infrastructure/capped"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

// The lyrics check places a song's lyrics, a text file with one line of the
// song per line and no times, in the song's sound with a CTC speech model,
// all the lines at once and in their order, the solos and refrains not
// written left between them. It prints where each line went and how sure
// the model was, never the words (lyrics are someone's work), and --out
// writes the lines as a SubRip subtitle the app opens beside the video.

const (
	// lyricsLimit bounds what is read of a lyrics file.
	lyricsLimit = 1 << 20
	// lyricsHold keeps a line on screen after its last word, unless the next
	// one comes first.
	lyricsHold = 1500 * time.Millisecond
)

func lyricsCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	flags := flag.NewFlagSet("lyrics-check", flag.ContinueOnError)
	flags.SetOutput(stdout)
	lyrics := flags.String("lyrics", "", "the lyrics: a text file, one line of the song per line")
	runtimePath := flags.String("runtime", "", "the onnxruntime.dll to run the model with")
	folder := flags.String("model", "", "the model's folder (model, vocab.json, meta.json)")
	file := flags.String("file", "model.onnx", "the model's file inside its folder")
	threads := flags.Int("threads", 2, "how many processors the model runs on")
	language := flags.String("language", "en", "the song's language, to pick its audio track")
	tool := flags.String("ffmpeg", "", "the FFmpeg to decode with; found as the app finds it by default")
	subtitleOut := flags.String("out", "", "writes the lines placed to this SubRip file")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *lyrics == "" || *runtimePath == "" || *folder == "" {
		return errors.New("lyrics-check: give --lyrics, --runtime and --model")
	}
	data, err := cappedfile.ReadFile(*lyrics, lyricsLimit)
	if err != nil {
		return err
	}
	var texts []string
	var words [][]string
	for line := range strings.Lines(string(data)) {
		text := strings.TrimSpace(line)
		if said := subtitle.SpokenWords(text); len(said) > 0 {
			texts, words = append(texts, text), append(words, said)
		}
	}
	if len(words) == 0 {
		return errors.New("lyrics-check: the lyrics have no words")
	}

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, path, defaultPreferences())
	if err != nil {
		return err
	}
	track, position, found := media.VoiceTrack(result.Info, media.LanguageTag(*language))
	if !found {
		return errors.New("lyrics-check: the video has no audio track to hear")
	}
	decoderPath, err := ffmpeg.Find(ctx, *tool)
	if err != nil {
		return err
	}
	dll, err := filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}
	model, err := speechpack.Open(dll, filepath.Join(*folder, *file), *threads)
	if err != nil {
		return err
	}
	defer model.Close()

	out := &printer{w: stdout}
	started := time.Now()
	sound, err := voice.DecodeMono(ctx, ffmpeg.Decoder{Path: decoderPath}, voice.Stretch{File: path, Track: position,
		Channels: track.Channels, Codec: track.Codec, Length: result.Info.Duration + time.Minute})
	if err != nil {
		return err
	}
	length := time.Duration(len(sound)) * time.Second / speech.SampleRate
	out.line("Song:      %s of sound, %d lines of lyrics, decoded in %s", formatClock(length), len(words), time.Since(started).Round(time.Second))
	began := time.Now()
	placed, err := model.AlignLines(sound, words)
	if err != nil {
		return err
	}
	out.line("Model:     %s, %d threads, %s", model.Version(), *threads, time.Since(began).Round(time.Second))
	out.line("")

	var events []subtitle.Event
	var medians []float64
	for i, line := range placed {
		var heard []speech.Word
		var sureness []float64
		for _, w := range line {
			if w.Heard {
				heard, sureness = append(heard, w), append(sureness, w.Confidence)
			}
		}
		if len(heard) == 0 {
			out.line("line %3d  not placed", i+1)
			continue
		}
		slices.Sort(sureness)
		median := quantileOf(sureness, 0.5)
		medians = append(medians, median)
		start, end := heard[0].Start, heard[len(heard)-1].End
		out.line("line %3d  %s - %s  words %d/%d  confidence median %.2f, lowest %.2f", i+1,
			formatMoment(start), formatMoment(end), len(heard), len(line), median, sureness[0])
		events = append(events, subtitle.Event{ID: strconv.Itoa(len(events) + 1), Start: start, End: end + lyricsHold, OriginalText: texts[i]})
	}
	for i := range len(events) - 1 {
		events[i].End = min(events[i].End, events[i+1].Start)
	}
	slices.Sort(medians)
	out.line("")
	out.line("Lines placed %d of %d, confidence median %.2f, p10 %.2f", len(events), len(words), quantileOf(medians, 0.5), quantileOf(medians, 0.1))
	if *subtitleOut != "" {
		written, err := subtitleformat.WriteFile(*subtitleOut, events)
		if err != nil {
			return err
		}
		out.line("Written:   %s", written)
	}
	return out.err
}
