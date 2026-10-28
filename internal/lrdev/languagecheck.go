package lrdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/speechpack"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// The language check hears one audio track as the app hears a track whose
// file does not tell its language: each language given places some lines of
// a subtitle in that language in the track, and the check prints each
// language's score and what the app would decide. It measures the rule's
// thresholds on films whose language is known.

// pairs is a repeated "key=value" flag.
type pairs map[string]string

func (p pairs) String() string { return fmt.Sprint(map[string]string(p)) }

func (p pairs) Set(value string) error {
	key, rest, ok := strings.Cut(value, "=")
	if !ok || key == "" || rest == "" {
		return fmt.Errorf("%q is not key=value", value)
	}
	p[key] = rest
	return nil
}

func languageCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	models, subtitles := pairs{}, pairs{}
	flags := flag.NewFlagSet("language-check", flag.ContinueOnError)
	flags.SetOutput(stdout)
	runtimePath := flags.String("runtime", "", "the onnxruntime.dll to run the models with")
	flags.Var(models, "pack", "a language and its model, en=<model.onnx> (vocab.json and meta.json beside it); repeat for each")
	flags.Var(subtitles, "subtitle", "a language's subtitle as inspect shows it, pt=#4; the first readable one in it by default")
	audio := flags.Int("audio", 1, "the audio track heard, by its place among the audio tracks (1 is the first)")
	threads := flags.Int("threads", 1, "how many processors each model runs on (the app uses 1)")
	tool := flags.String("ffmpeg", "", "the FFmpeg to decode with; found as the app finds it by default")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *runtimePath == "" || len(models) == 0 {
		return errors.New("language-check: give --runtime and at least one --pack")
	}
	dll, err := filepath.Abs(*runtimePath)
	if err != nil {
		return err
	}

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, path, defaultPreferences())
	if err != nil {
		return err
	}
	tracks := result.Info.TracksOf(media.TrackAudio)
	if *audio < 1 || *audio > len(tracks) {
		return fmt.Errorf("language-check: the video has %d audio tracks, not a %d", len(tracks), *audio)
	}
	track := tracks[*audio-1]
	languages := make([]string, 0, len(models))
	for language := range models {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	numbers := map[string]uint64{}
	for _, language := range languages {
		number, err := subtitleIn(result.Info, language, subtitles[language])
		if err != nil {
			return err
		}
		numbers[language] = number
	}
	wanted := make([]uint64, 0, len(numbers))
	for _, number := range numbers {
		wanted = append(wanted, number)
	}
	loaded, err := service.LoadSubtitles(ctx, result, wanted, nil)
	if err != nil {
		return err
	}
	decoderPath, err := ffmpeg.Find(ctx, *tool)
	if err != nil {
		return err
	}
	decoder := ffmpeg.Decoder{Path: decoderPath}
	stretch := voice.Stretch{File: path, Track: *audio - 1, Channels: track.Channels, Codec: track.Codec}

	out := &printer{w: stdout}
	out.line("Audio:     %s (%s%s), the track heard", trackLabel(track.Number), orUnknown(string(track.Language)), guessedMark(track))
	var scores []speech.LanguageScore
	for _, language := range languages {
		model, err := speechpack.Open(dll, models[language], *threads)
		if err != nil {
			return err
		}
		lines := voice.LinesToHear(loaded[numbers[language]].Timeline.Events())
		found, err := voice.ScoreLanguages(ctx, decoder, stretch, []voice.LanguageCandidate{{Language: language, Model: model, Lines: lines}}, nil)
		model.Close()
		if err != nil {
			return err
		}
		scores = append(scores, found...)
		out.line("%-4s subtitle %s, %d lines heard of %d: score %.2f", language, trackLabel(numbers[language]), found[0].Lines, len(lines), found[0].Score)
	}
	if language, ok := speech.HeardLanguage(scores); ok {
		out.line("Decided:   %s", language)
	} else {
		out.line("Decided:   nothing (the sound does not tell)")
	}
	return out.err
}

// subtitleIn is the subtitle whose lines are heard for a language: the one
// named, or the first readable one in it.
func subtitleIn(info media.Info, language, label string) (uint64, error) {
	if label != "" {
		return parseTrackLabel(label)
	}
	for _, track := range info.TracksOf(media.TrackSubtitle) {
		if track.Readable && track.Language.Base() == language {
			return track.Number, nil
		}
	}
	return 0, fmt.Errorf("language-check: no readable subtitle in %q; name one with --subtitle %s=#n", language, language)
}

func orUnknown(language string) string {
	if language == "" {
		return "no language"
	}
	return language
}

func guessedMark(track media.Track) string {
	if track.LanguageGuessed {
		return ", guessed"
	}
	return ""
}
