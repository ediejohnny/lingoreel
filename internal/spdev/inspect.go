package spdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/infrastructure/media/matroska"
	"lingoreel/internal/infrastructure/subtitlefile"
)

// ErrMissingArgument reports a command called without a required argument.
var ErrMissingArgument = errors.New("argumento obrigatório ausente")

const (
	subtitlesFlag     = "--legendas"
	previewEventCount = 3
)

var trackTypeNames = map[media.TrackType]string{
	media.TrackVideo:    "vídeo",
	media.TrackAudio:    "áudio",
	media.TrackSubtitle: "legenda",
	media.TrackOther:    "outra",
}

// bitmapSubtitleCodecs are image based subtitles, which would need OCR.
var bitmapSubtitleCodecs = []string{"S_HDMV/PGS", "S_VOBSUB", "S_DVBSUB", "S_IMAGE/BMP"}

// inspect prints the tracks of a video, the suggested study tracks and, with
// --legendas, the first lines of the suggested subtitles.
func inspect(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: caminho do vídeo", ErrMissingArgument)
	}
	path := args[0]
	withSubtitles := len(args) > 1 && args[1] == subtitlesFlag

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(path, defaultPreferences())
	if err != nil {
		return err
	}

	out := &printer{w: stdout}
	out.line("Arquivo:  %s", path)
	out.line("Duração:  %s", formatClock(result.Info.Duration))
	out.line("")
	out.line("Faixas:")
	for _, track := range result.Info.Tracks {
		out.line("  %s", describeTrack(track))
	}
	out.line("")
	out.line("Sugestão:")
	out.line("  Áudio:     %s", describeChoice(result.Info, result.Suggested.Audio))
	out.line("  Legenda:   %s", describeChoice(result.Info, result.Suggested.Primary))
	out.line("  Tradução:  %s", describeChoice(result.Info, result.Suggested.Translation))
	if out.err != nil || !withSubtitles {
		return out.err
	}

	loaded, err := service.LoadSubtitles(context.Background(), result, []uint64{result.Suggested.Primary, result.Suggested.Translation}, nil)
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
		out.line("Legenda %s: %d falas (%d descartadas)", trackLabel(number), len(events), track.Skipped)
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
			parts = append(parts, strings.Replace(fmt.Sprintf("%.3f fps", track.FrameRate), ".", ",", 1))
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
		parts = append(parts, "[padrão]")
	}
	if track.Forced {
		parts = append(parts, "[forçada]")
	}
	if track.HearingImpaired {
		parts = append(parts, "[SDH]")
	}
	if track.External() {
		parts = append(parts, "[arquivo]")
	}
	return strings.Join(parts, "  ")
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
		return "(não suportada: legenda em imagem, precisaria de OCR)"
	}
	return "(não suportada)"
}

func describeChoice(info media.Info, number uint64) string {
	track, found := info.Track(number)
	if number == 0 || !found {
		return "nenhuma"
	}
	language := string(track.Language)
	if language == "" {
		language = "idioma desconhecido"
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

// printer writes lines and keeps the first write error.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) line(format string, args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format+"\n", args...)
	}
}
