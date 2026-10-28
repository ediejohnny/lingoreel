package lrdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/silero"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// ErrMissingArgument reports a command called without a required argument.
var ErrMissingArgument = errors.New("missing argument")

const (
	subtitlesFlag     = "--subtitles"
	hearFlag          = "--hear"
	ffmpegFlag        = "--ffmpeg"
	previewEventCount = 3
	// rateShownFrom is the least speed change worth printing (0.001%).
	rateShownFrom = 1e-5
)

var trackTypeNames = map[media.TrackType]string{
	media.TrackVideo:    "video",
	media.TrackAudio:    "audio",
	media.TrackSubtitle: "subtitle",
	media.TrackOther:    "other",
}

// bitmapSubtitleCodecs are image based subtitles, which would need OCR.
var bitmapSubtitleCodecs = []string{"S_HDMV/PGS", "S_VOBSUB", "S_DVBSUB", "S_IMAGE/BMP"}

// inspect prints the tracks of a video, the suggested study tracks and, with
// --subtitles, the first lines of the suggested subtitles. With --hear, the
// files nothing in the video could measure are lined up with its voice
// first, as the app does when the video opens (--ffmpeg picks the FFmpeg).
func inspect(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	withSubtitles, hearing, tool := false, false, ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case subtitlesFlag:
			withSubtitles = true
		case hearFlag:
			hearing = true
		case ffmpegFlag:
			if i+1 == len(args) {
				return fmt.Errorf("%w: the FFmpeg's path after %s", ErrMissingArgument, ffmpegFlag)
			}
			i++
			tool = args[i]
		default:
			return fmt.Errorf("inspect: unknown option %q", args[i])
		}
	}

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, path, defaultPreferences())
	if err != nil {
		return err
	}
	if hearing {
		if result, err = heardAnalysis(ctx, result, tool, stdout); err != nil {
			return err
		}
	}

	out := &printer{w: stdout}
	out.line("File:      %s", path)
	out.line("Duration:  %s", formatClock(result.Info.Duration))
	out.line("")
	out.line("Tracks:")
	for _, track := range result.Info.Tracks {
		out.line("  %s", describeTrack(track))
		if track.External() && track.Readable {
			out.line("       timing: %s", describeAlignment(track))
		}
	}
	out.line("")
	out.line("Suggestion:")
	out.line("  Audio:        %s", describeChoice(result.Info, result.Suggested.Audio))
	out.line("  Top line:     %s", describeChoice(result.Info, result.Suggested.Primary))
	out.line("  Bottom line:  %s", describeChoice(result.Info, result.Suggested.Translation))
	if out.err != nil || !withSubtitles {
		return out.err
	}

	loaded, err := service.LoadSubtitles(ctx, result, []uint64{result.Suggested.Primary, result.Suggested.Translation}, nil)
	if err != nil {
		return err
	}
	for _, number := range []uint64{result.Suggested.Primary, result.Suggested.Translation} {
		track, found := loaded[number]
		if !found {
			continue
		}
		events := track.Timeline.Events()
		out.line("")
		out.line("Subtitle %s: %d lines (%d left out)", trackLabel(number), len(events), track.Skipped)
		for _, event := range events[:min(previewEventCount, len(events))] {
			out.line("  %s → %s  %s", formatClock(event.Start), formatClock(event.End), strings.ReplaceAll(event.Text(), "\n", " / "))
		}
	}
	return out.err
}

func defaultPreferences() media.Preferences {
	languages := settings.DefaultProfile().Languages
	return media.Preferences{
		Audio:       media.LanguageTag(languages.Audio),
		Primary:     media.LanguageTag(languages.Primary),
		Translation: media.LanguageTag(languages.Translation),
	}
}

func describeTrack(track media.Track) string {
	language := string(track.Language)
	if language == "" {
		language = "?"
	}
	parts := []string{fmt.Sprintf("%-4s %-8s %-6s %s", trackLabel(track.Number), trackTypeNames[track.Type], language, track.Codec)}
	switch track.Type {
	case media.TrackVideo:
		if track.Width > 0 {
			parts = append(parts, fmt.Sprintf("%d×%d", track.Width, track.Height))
		}
		if track.FrameRate > 0 {
			parts = append(parts, fmt.Sprintf("%.3f fps", track.FrameRate))
		}
	case media.TrackSubtitle:
		if !track.Readable {
			parts = append(parts, unreadableReason(track.Codec))
		}
	}
	if track.Name != "" {
		parts = append(parts, fmt.Sprintf("%q", track.Name))
	}
	if track.Default {
		parts = append(parts, "[default]")
	}
	if track.Forced {
		parts = append(parts, "[forced]")
	}
	if track.HearingImpaired {
		parts = append(parts, "[SDH]")
	}
	switch {
	case track.Candidate:
		parts = append(parts, fmt.Sprintf("[candidate in %s]", filepath.Base(filepath.Dir(track.Path))))
	case track.External():
		parts = append(parts, "[file]")
	}
	return strings.Join(parts, "  ")
}

// describeAlignment tells how a subtitle file fits the video.
func describeAlignment(track media.Track) string {
	alignment := track.Alignment
	share := fmt.Sprintf("%.0f%% of the lines", alignment.Score*100)
	notChosen := ""
	if !track.AutoSelectable() {
		notChosen = "; never picked on its own"
	}
	if alignment.Heard {
		share = fmt.Sprintf("%.0f%% of the lines heard fall in speech", alignment.Score*100)
	}
	switch alignment.Verdict {
	case subtitle.VerdictMatch:
		if _, moved := analysis.Correction(track); !moved {
			return fmt.Sprintf("fits the video (%s); kept as timed (found %s)", share, describeCorrection(alignment))
		}
		return fmt.Sprintf("fits the video (%s); correction %s", share, describeCorrection(alignment))
	case subtitle.VerdictOtherCut:
		return fmt.Sprintf("made for another cut of the film (an extended or director's cut?), drifting at every scene (%s)%s", share, notChosen)
	case subtitle.VerdictNoMatch:
		return fmt.Sprintf("does not fit this video (%s)%s", share, notChosen)
	}
	return "no subtitle inside the video to compare with" + notChosen
}

// describeCorrection shows the move a matching file gets, as "+0.036 s" and,
// when it runs at another speed, "speed × 0.99996".
func describeCorrection(alignment subtitle.Alignment) string {
	text := fmt.Sprintf("%+.3f s", alignment.Offset.Seconds())
	if math.Abs(alignment.Rate-1) >= rateShownFrom {
		text += fmt.Sprintf(", speed × %.5f", alignment.Rate)
	}
	return text
}

// trackLabel shows container tracks as "#3" and subtitle files as "#A1",
// "#A2"… in the order they were found.
func trackLabel(number uint64) string {
	if number >= media.ExternalTrackBase {
		return fmt.Sprintf("#A%d", number-media.ExternalTrackBase+1)
	}
	return fmt.Sprintf("#%d", number)
}

func unreadableReason(codec string) string {
	if slices.Contains(bitmapSubtitleCodecs, codec) {
		return "(not supported: a subtitle in pictures, which would need OCR)"
	}
	return "(not supported)"
}

func describeChoice(info media.Info, number uint64) string {
	track, found := info.Track(number)
	if number == 0 || !found {
		return "none"
	}
	language := string(track.Language)
	if language == "" {
		language = "unknown language"
	}
	return fmt.Sprintf("%s (%s)", trackLabel(number), language)
}

// formatClock shows a duration as h:mm:ss.mmm, or h:mm:ss for whole seconds.
func formatClock(d time.Duration) string {
	d = max(d, 0)
	hours := d / time.Hour
	minutes := (d % time.Hour) / time.Minute
	seconds := (d % time.Minute) / time.Second
	milliseconds := (d % time.Second) / time.Millisecond
	if milliseconds == 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

// printer writes lines, keeping the first error. Lines may come from several
// goroutines (the session and the shortcuts); err is read once they stopped.
type printer struct {
	mu  sync.Mutex
	w   io.Writer
	err error
}

func (p *printer) line(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format+"\n", args...)
	}
}

// heardAnalysis lines the unheard files of a video up with its voice, heard
// as the app hears it: the same stretches and audio track, at its pace.
func heardAnalysis(ctx context.Context, result analysis.Analysis, tool string, stdout io.Writer) (analysis.Analysis, error) {
	if len(result.Unheard()) == 0 {
		return result, nil
	}
	path, err := ffmpeg.Find(ctx, tool)
	if err != nil {
		return result, err
	}
	preferences := defaultPreferences()
	decoder := ffmpeg.Decoder{Path: path}
	started := time.Now()
	telling := func(stretches []subtitle.HearingStretch, label string) func(int, float64) {
		told := -1
		return func(stretch int, _ float64) {
			if stretch != told && stretch < len(stretches) {
				told = stretch
				fmt.Fprintf(stdout, "Hearing %sstretch %d of %d (%s)...\n", label, stretch+1, len(stretches), formatClock(stretches[stretch].From))
			}
		}
	}
	heard, err := analysis.HearVoice(ctx, result, decoder, silero.Embedded{}, preferences.Primary,
		telling(subtitle.HearingStretches(result.HearingLength()), ""))
	if err != nil {
		return result, err
	}
	var more []speech.Track
	if between := subtitle.HearingStretchesBetween(result.HearingLength()); len(between) > 0 && analysis.Undecided(result, heard) {
		if more, err = analysis.HearMoreVoice(ctx, result, decoder, silero.Embedded{}, preferences.Primary, telling(between, "the undecided file's ")); err != nil {
			return result, err
		}
	}
	fmt.Fprintf(stdout, "Heard in %s.\n\n", time.Since(started).Round(time.Second))
	fitted, _ := analysis.FitByVoice(result, heard, more, preferences)
	return fitted, nil
}
