package lrdev

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// How a film's lines are told apart: an isolated line has a second with
// nothing said on each side, a dense one has another line within half a
// second; the film is also measured by quarters of an hour.
const (
	isolatedGap   = time.Second
	denseGap      = 500 * time.Millisecond
	checkStretch  = 15 * time.Minute
	reportEvery   = 15 * time.Minute
	marginSlack   = 200 * time.Millisecond // a subtitle's start and end are a little off the voice
	speechVersion = "silero-v6.2.1/analyzer-1"
	speechMagic   = "SPEECH1\n"
	// maxSpeechCache bounds a cache file: 4 h of frames.
	maxSpeechCache = 4 * 3600 * 100 * 4
)

// The measured line contexts, in the order they are printed.
var lineContexts = []string{"isolated", "dense", "other"}

// voiceCheck measures, over a whole film, how the voice the speech model
// finds fits the lines of a subtitle that matches the film. It prints only
// numbers, never a line's text nor the film's name, so its output can be
// kept with the measurements.
func voiceCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	flags := flag.NewFlagSet("voice-check", flag.ContinueOnError)
	flags.SetOutput(stdout)
	label := flags.String("subtitle", "", "the subtitle whose lines are measured, as inspect shows it (#3, #A1); the suggested top line by default")
	shift := flags.Duration("shift", 0, "moves every line by this much (300ms, -1.2s), after the correction of a file that fits")
	tool := flags.String("ffmpeg", "", "the FFmpeg to decode with; found as the app finds it by default")
	cache := flags.String("cache", "", "a folder to keep the speech heard in, so a film is decoded only once")
	length := flags.Duration("length", 0, "measures only the film's start, this long (10m); the whole film by default")
	clips := flags.Int("clips", 0, "cuts this many lines into sound clips in --out, to mark by hand where the voice begins")
	clipsOut := flags.String("out", "", "the folder for the clips")
	labels := flags.String("labels", "", "a file of lines marked by hand, to measure the start against")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *clips > 0 && *clipsOut == "" {
		return errors.New("voice-check: --clips needs --out")
	}
	film, err := filmKey(path)
	if err != nil {
		return err
	}

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
		return errors.New("voice-check: no subtitle to measure; choose one with --subtitle")
	}
	loaded, err := service.LoadSubtitles(ctx, result, []uint64{number}, nil)
	if err != nil {
		return err
	}
	subtitles, found := loaded[number]
	if !found {
		return fmt.Errorf("voice-check: the subtitle %s cannot be read", trackLabel(number))
	}
	track, position, found := media.VoiceTrack(result.Info, lines.Language)
	if !found {
		return errors.New("voice-check: the video has no audio track to hear")
	}

	out := &printer{w: stdout}
	out.line("Film:      %s", film)
	out.line("Voice:     %s (%s, %d channels)", trackLabel(track.Number), track.Codec, track.Channels)
	correction := "none"
	if subtitles.Corrected {
		correction = describeCorrection(subtitles.Correction)
	}
	out.line("Lines:     %s, correction %s, shift %+.3f s", trackLabel(number), correction, shift.Seconds())
	stretch := voice.Stretch{File: path, Track: position, Channels: track.Channels, Codec: track.Codec,
		Length: result.Info.Duration + time.Minute}
	if *length > 0 {
		stretch.Length = *length
	}
	frames, err := heardSpeech(ctx, out, stretch, *tool, *cache)
	if err != nil {
		return err
	}
	sound := speech.Track{Frames: frames}
	events := subtitles.Timeline.Events()
	var marked map[int64]*int64
	if *labels != "" {
		if marked, err = readLabels(*labels, film); err != nil {
			return err
		}
	}
	for _, sensitivity := range []speech.Sensitivity{speech.SensitivityLow, speech.SensitivityNormal, speech.SensitivityHigh} {
		out.line("")
		reportVoice(out, sound, events, *shift, sensitivity, marked)
	}
	if *clips > 0 {
		heard := time.Duration(len(frames)) * speech.FrameLength
		picked := pickLines(film, events, *shift, heard, *clips)
		if err := writeClips(ctx, *tool, stretch, film, events, picked, *shift, *clipsOut); err != nil {
			return err
		}
		out.line("")
		out.line("Clips:     %d lines cut", len(picked))
	}
	out.line("")
	out.line("Targets: start within 80 ms (median) and 200 ms (P90), 60%% of the lines trusted, voice in under 15%% of the isolated margins.")
	return out.err
}

// parseTrackLabel reads a track as trackLabel shows it: "#3" or "#A1".
func parseTrackLabel(label string) (uint64, error) {
	text, external := strings.TrimPrefix(label, "#"), false
	if rest, ok := strings.CutPrefix(text, "A"); ok {
		text, external = rest, true
	}
	n, err := strconv.ParseUint(text, 10, 32)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("voice-check: %q is not a track (#3, #A1)", label)
	}
	if external {
		return media.ExternalTrackBase + n - 1, nil
	}
	return n, nil
}

// heardSpeech is the speech of a stretch frame by frame: from the cache, or
// decoded and judged by the model in one go (then kept in the cache).
func heardSpeech(ctx context.Context, out *printer, stretch voice.Stretch, tool, cacheDir string) ([]speech.Frame, error) {
	cachePath := ""
	if cacheDir != "" {
		key, err := speechCacheKey(stretch)
		if err != nil {
			return nil, err
		}
		cachePath = filepath.Join(cacheDir, key+".speech")
		if frames, err := readSpeechCache(cachePath); err == nil {
			out.line("Speech:    %s of sound, read from the cache", formatClock(time.Duration(len(frames))*speech.FrameLength))
			return frames, nil
		}
	}
	path, err := ffmpeg.Find(ctx, tool)
	if err != nil {
		return nil, err
	}
	stream, err := silero.Embedded{}.NewStream()
	if err != nil {
		return nil, err
	}
	var analyzer *speech.Analyzer
	var frames []speech.Frame
	var analyzerErr error
	next, started := reportEvery, time.Now()
	err = ffmpeg.Decoder{Path: path}.Decode(ctx, stretch, func(samples []float32, channels int) {
		if analyzer == nil && analyzerErr == nil {
			analyzer, analyzerErr = speech.NewAnalyzer(channels, stream)
		}
		if analyzer == nil {
			return
		}
		frames = analyzer.Write(samples, frames)
		if heard := time.Duration(len(frames)) * speech.FrameLength; heard >= next {
			out.line("           %s heard", formatClock(heard))
			next += reportEvery
		}
	})
	if err = errors.Join(err, analyzerErr); err != nil {
		return nil, err
	}
	if analyzer != nil {
		frames = analyzer.Flush(frames)
	}
	heard, took := time.Duration(len(frames))*speech.FrameLength, time.Since(started)
	out.line("Speech:    %s of sound in %s (%.0f× real time)", formatClock(heard), took.Round(time.Second), heard.Seconds()/max(took.Seconds(), 1e-3))
	if cachePath != "" {
		if err := writeSpeechCache(cachePath, frames); err != nil {
			return nil, err
		}
	}
	return frames, nil
}

// speechCacheKey names a stretch's cache by the file (path, size, time) and
// the track, never by the film's name.
func speechCacheKey(stretch voice.Stretch) (string, error) {
	abs, err := filepath.Abs(stretch.File)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%d|%d|%d|%s|%d|%d", speechVersion, strings.ToLower(abs), info.Size(),
		info.ModTime().UnixNano(), stretch.Track, stretch.Codec, stretch.Channels, stretch.Length))
	return hex.EncodeToString(sum[:8]), nil
}

func readSpeechCache(path string) ([]speech.Frame, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > int64(len(speechMagic)+maxSpeechCache) {
		return nil, errors.New("voice-check: the cache file is too big")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body, ok := strings.CutPrefix(string(data), speechMagic)
	if !ok || len(body)%4 != 0 {
		return nil, errors.New("voice-check: not a speech cache")
	}
	frames := make([]speech.Frame, len(body)/4)
	for i := range frames {
		frames[i].Speech = math.Float32frombits(binary.LittleEndian.Uint32([]byte(body[4*i : 4*i+4])))
	}
	return frames, nil
}

func writeSpeechCache(path string, frames []speech.Frame) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data := make([]byte, 0, len(speechMagic)+4*len(frames))
	data = append(data, speechMagic...)
	for _, f := range frames {
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(f.Speech))
	}
	return os.WriteFile(path, data, 0o644)
}

// measuredLine is how the voice found fits one line.
type measuredLine struct {
	context       string
	stretch       int
	verdict       speech.Verdict
	onset, finish time.Duration // first voice start - line start; last voice end - line end
}

// reportVoice prints how the voice found at one sensitivity fits the lines.
func reportVoice(out *printer, sound speech.Track, events []subtitle.Event, shift time.Duration, sensitivity speech.Sensitivity, marked map[int64]*int64) {
	heard := time.Duration(len(sound.Frames)) * speech.FrameLength
	starts, ends := make([]time.Duration, len(events)), make([]time.Duration, len(events))
	for i, event := range events {
		starts[i], ends[i] = event.Start+shift, event.End+shift
	}
	var lines []measuredLine
	var subtitleErrs, foundErrs, paintedErrs, liveErrs []time.Duration
	trustedMarked := 0
	var marginVoice, marginAll, lineVoice, lineAll int
	voiced := listened(sound, sensitivity)
	for i, event := range events {
		units := study.SpeechUnits(event.Text())
		if !units.Spoken() || units.Lyrics || starts[i] < 0 || ends[i] > heard-time.Second {
			continue
		}
		weight := 0.0
		for _, word := range units.Words {
			weight += word.Weight
		}
		line := measuredLine{context: "other", stretch: int(starts[i] / checkStretch)}
		before, after := gap(starts, ends, i, -1), gap(starts, ends, i, 1)
		switch {
		case before >= isolatedGap && after >= isolatedGap:
			line.context = "isolated"
			v, a := countVoice(voiced, starts[i]-isolatedGap, starts[i]-marginSlack)
			w, b := countVoice(voiced, ends[i]+marginSlack, ends[i]+isolatedGap)
			marginVoice, marginAll = marginVoice+v+w, marginAll+a+b
			v, a = countVoice(voiced, starts[i], ends[i])
			lineVoice, lineAll = lineVoice+v, lineAll+a
		case before < denseGap || after < denseGap:
			line.context = "dense"
		}
		found := speech.DetectLine(sound, starts[i], ends[i], weight, sensitivity)
		line.verdict = found.Verdict
		painted := starts[i] // the estimate starts with the line
		if found.Verdict == speech.Trusted {
			line.onset = found.Spans[0].Start - starts[i]
			line.finish = found.Spans[len(found.Spans)-1].End - ends[i]
			painted = found.Spans[0].Start
		}
		if onset, ok := marked[starts[i].Milliseconds()]; ok && onset != nil {
			voiceAt := time.Duration(*onset) * time.Millisecond
			subtitleErrs = append(subtitleErrs, (starts[i] - voiceAt).Abs())
			paintedErrs = append(paintedErrs, (painted - voiceAt).Abs())
			if found.Verdict == speech.Trusted {
				trustedMarked++
				foundErrs = append(foundErrs, (painted - voiceAt).Abs())
			}
			// Live, the fill hears the line from its start (study.LiveFill).
			if heardAt, ok := firstVoice(voiced, starts[i], ends[i]); ok {
				liveErrs = append(liveErrs, (heardAt - voiceAt).Abs())
			}
		}
		lines = append(lines, line)
	}

	out.line("Sensitivity %s", sensitivity)
	out.line("  %-12s %5s %8s | %25s | %18s | %25s", "", "lines", "trusted", "start P10 / median / P90", "|start| med / P90", "end P10 / median / P90")
	reportLines(out, "whole film", lines, func(measuredLine) bool { return true })
	for _, context := range lineContexts {
		reportLines(out, "  "+context, lines, func(l measuredLine) bool { return l.context == context })
	}
	counts := map[speech.Verdict]int{}
	for _, l := range lines {
		counts[l.verdict]++
	}
	out.line("  not trusted: %d missing, %d no voice, %d steady, %d wrong pace",
		counts[speech.Missing], counts[speech.NoVoice], counts[speech.Steady], counts[speech.WrongPace])
	out.line("  isolated lines: voice in %s of the margins, %s of their time", share(marginVoice, marginAll), share(lineVoice, lineAll))
	if len(subtitleErrs) > 0 {
		out.line("  marked by hand: %d lines; how far from the voice's start, median / P90:", len(subtitleErrs))
		out.line("    the subtitle's start (the estimate)  %s / %s", quantile(subtitleErrs, 0.5), quantile(subtitleErrs, 0.9))
		out.line("    the voice found (%s trusted)        %s / %s", share(trustedMarked, len(subtitleErrs)), quantile(foundErrs, 0.5), quantile(foundErrs, 0.9))
		out.line("    what the app paints from            %s / %s", quantile(paintedErrs, 0.5), quantile(paintedErrs, 0.9))
		out.line("    live, the player heard as it plays  %s / %s (%d lines; up to 32 ms more until a chunk is judged)",
			quantile(liveErrs, 0.5), quantile(liveErrs, 0.9), len(liveErrs))
	}
	var quarters []string
	for stretch := 0; time.Duration(stretch)*checkStretch < heard; stretch++ {
		total, trusted := 0, 0
		for _, l := range lines {
			if l.stretch == stretch {
				total++
				if l.verdict == speech.Trusted {
					trusted++
				}
			}
		}
		quarters = append(quarters, share(trusted, total))
	}
	out.line("  trusted by quarter of an hour: %s", strings.Join(quarters, " "))
}

// listened tells, frame by frame, whether the Listener hears voice.
func listened(sound speech.Track, sensitivity speech.Sensitivity) []bool {
	listener := speech.NewListener(sensitivity)
	voiced := make([]bool, len(sound.Frames))
	for i, frame := range sound.Frames {
		voiced[i], _ = listener.Hear(frame)
	}
	return voiced
}

// firstVoice is the first frame heard as voice from one time to another.
func firstVoice(voiced []bool, from, to time.Duration) (time.Duration, bool) {
	first, last := max(int(from/speech.FrameLength), 0), min(int(to/speech.FrameLength), len(voiced))
	for i := first; i < last; i++ {
		if voiced[i] {
			return time.Duration(i) * speech.FrameLength, true
		}
	}
	return 0, false
}

// countVoice counts the frames of voice from one time to another, and all
// the frames there.
func countVoice(voiced []bool, from, to time.Duration) (voice, all int) {
	first, last := max(int(from/speech.FrameLength), 0), min(int(to/speech.FrameLength), len(voiced))
	for _, v := range voiced[min(first, last):last] {
		all++
		if v {
			voice++
		}
	}
	return voice, all
}

// gap is the time between a line and the one before (side -1) or after it
// (side 1); an hour when there is none.
func gap(starts, ends []time.Duration, i, side int) time.Duration {
	j := i + side
	switch {
	case j < 0 || j >= len(starts):
		return time.Hour
	case side < 0:
		return starts[i] - ends[j]
	}
	return starts[j] - ends[i]
}

func reportLines(out *printer, title string, lines []measuredLine, keep func(measuredLine) bool) {
	var onsets, finishes, errs []time.Duration
	total := 0
	for _, l := range lines {
		if !keep(l) {
			continue
		}
		total++
		if l.verdict == speech.Trusted {
			onsets, finishes, errs = append(onsets, l.onset), append(finishes, l.finish), append(errs, l.onset.Abs())
		}
	}
	if total == 0 {
		return
	}
	out.line("  %-12s %5d %8s | %7s / %7s / %7s | %8s / %7s | %7s / %7s / %7s", title, total, share(len(onsets), total),
		quantile(onsets, 0.1), quantile(onsets, 0.5), quantile(onsets, 0.9), quantile(errs, 0.5), quantile(errs, 0.9),
		quantile(finishes, 0.1), quantile(finishes, 0.5), quantile(finishes, 0.9))
}

// quantile shows a share of the way through durations, in milliseconds.
func quantile(list []time.Duration, q float64) string {
	if len(list) == 0 {
		return "-"
	}
	return fmt.Sprintf("%+d ms", quantileOf(list, q).Milliseconds())
}

func share(part, total int) string {
	if total == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(part)/float64(total))
}
