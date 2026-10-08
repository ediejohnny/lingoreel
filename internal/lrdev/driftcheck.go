package lrdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// The drift check plays a film at 1× without the screen: every few seconds
// it reads the voice around the playhead (as the karaoke follower keeps it)
// against the lines shown, the way the app would, and moves them as the
// drift tracker says. --cut and --shift make a version that slips: every
// line from the cut on is shown that much off, so the tracker should bring
// it back. Numbers only: no text, no film name.

const (
	driftTick     = 5 * time.Second
	driftBehind   = 100 * time.Second
	driftAheadOf  = 20 * time.Second
	driftReportAt = time.Minute
)

func driftCheck(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	flags := flag.NewFlagSet("drift-check", flag.ContinueOnError)
	flags.SetOutput(stdout)
	label := flags.String("subtitle", "", "the subtitle followed, as inspect shows it (#3, #A1); the suggested top line by default")
	cut := flags.Duration("cut", 0, "where the version slips (45m); no slip by default")
	shift := flags.Duration("shift", 0, "how far every line from --cut on is off (800ms, -1.5s)")
	tool := flags.String("ffmpeg", "", "the FFmpeg to decode with; found as the app finds it by default")
	cache := flags.String("cache", "", "a folder to keep the speech heard in, so a film is decoded only once")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if (*cut == 0) != (*shift == 0) {
		return errors.New("drift-check: give --cut and --shift together")
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
		return errors.New("drift-check: no subtitle to follow; choose one with --subtitle")
	}
	loaded, err := service.LoadSubtitles(ctx, result, []uint64{number}, nil)
	if err != nil {
		return err
	}
	track, position, found := media.VoiceTrack(result.Info, lines.Language)
	if !found {
		return errors.New("drift-check: the video has no audio track to hear")
	}
	out := &printer{w: stdout}
	stretch := voice.Stretch{File: path, Track: position, Channels: track.Channels, Codec: track.Codec,
		Length: result.Info.Duration + time.Minute}
	frames, err := heardSpeech(ctx, out, stretch, *tool, *cache)
	if err != nil {
		return err
	}
	var shown []speech.Span
	for _, e := range loaded[number].Timeline.Events() {
		if e.Text() == "" {
			continue
		}
		span := speech.Span{Start: e.Start, End: e.End}
		if *cut > 0 && e.Start >= *cut {
			span.Start, span.End = span.Start-*shift, span.End-*shift
		}
		shown = append(shown, span)
	}
	out.line("Lines:     %d, slip %+.3f s from %s", len(shown), shift.Seconds(), formatClock(*cut))

	var tracker subtitle.DriftTracker
	start := time.Now()
	length := time.Duration(len(frames)) * speech.FrameLength
	trusted, read, moves := 0, 0, 0
	var firstMove time.Duration = -1
	var spent time.Duration
	for at := time.Duration(0); at < length && ctx.Err() == nil; at += driftTick {
		from, to := max(at-driftBehind, 0), min(at+driftAheadOf, length)
		window := speech.Track{Start: from, Frames: frames[from/speech.FrameLength : to/speech.FrameLength]}
		began := time.Now()
		moved := make([]speech.Span, len(shown))
		for i, s := range shown {
			moved[i] = speech.Span{Start: s.Start + tracker.Auto(), End: s.End + tracker.Auto()}
		}
		reading := subtitle.ReadDrift(window, moved)
		now := start.Add(at)
		tracker.Heard(reading, at, to, now)
		step, stepped := tracker.Step(now)
		spent += time.Since(began)
		read++
		if reading.Trusted() {
			trusted++
		}
		if stepped {
			moves++
			if firstMove < 0 {
				firstMove = at
			}
			out.line("%s  move %+5d ms  auto %+5d ms  lines %2d  score %.2f  sharp %.2f  voiced %.2f",
				formatClock(at), step.Milliseconds(), tracker.Auto().Milliseconds(), reading.Lines, reading.Score, reading.Sharpness, reading.Voiced)
		} else if at%driftReportAt == 0 {
			out.line("%s  residual %+5d ms  lines %2d  score %.2f  sharp %.2f  voiced %.2f  trusted %v",
				formatClock(at), reading.Residual.Milliseconds(), reading.Lines, reading.Score, reading.Sharpness, reading.Voiced, reading.Trusted())
		}
	}
	out.line("")
	out.line("Windows %d, trusted %d (%.0f%%), moves %d, first at %s, final auto %+d ms (want %+d ms after the cut)",
		read, trusted, 100*float64(trusted)/float64(max(read, 1)), moves, formatClock(max(firstMove, 0)), tracker.Auto().Milliseconds(), shift.Milliseconds())
	for _, a := range tracker.Anchors() {
		out.line("Anchor from %s: %+d ms", formatClock(a.From), a.Auto.Milliseconds())
	}
	out.line("Checker time: %s for %s of film (%.1f µs per film second)", spent.Round(time.Millisecond), formatClock(length),
		float64(spent.Microseconds())/max(length.Seconds(), 1))
	return out.err
}
