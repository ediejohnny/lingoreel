package spdev

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

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/media/matroska"
	"lingoreel/internal/infrastructure/subtitlefile"
)

// ErrMissingArgument reports a command called without a required argument.
var ErrMissingArgument = errors.New("argumento obrigatório ausente")

const (
	subtitlesFlag     = "--legendas"
	previewEventCount = 3
	// rateShownFrom is the least speed change worth printing (0.001%).
	rateShownFrom = 1e-5
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
func inspect(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: caminho do vídeo", ErrMissingArgument)
	}
	path := args[0]
	withSubtitles := len(args) > 1 && args[1] == subtitlesFlag

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, path, defaultPreferences())
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
		if track.External() && track.Readable {
			out.line("       sincronia: %s", describeAlignment(track))
		}
	}
	out.line("")
	out.line("Sugestão:")
	out.line("  Áudio:     %s", describeChoice(result.Info, result.Suggested.Audio))
	out.line("  Legenda:   %s", describeChoice(result.Info, result.Suggested.Primary))
	out.line("  Tradução:  %s", describeChoice(result.Info, result.Suggested.Translation))
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
	switch {
	case track.Candidate:
		parts = append(parts, fmt.Sprintf("[candidata em %s]", filepath.Base(filepath.Dir(track.Path))))
	case track.External():
		parts = append(parts, "[arquivo]")
	}
	return strings.Join(parts, "  ")
}

// describeAlignment tells how a subtitle file fits the video.
func describeAlignment(track media.Track) string {
	alignment := track.Alignment
	share := fmt.Sprintf("%.0f%% das falas", alignment.Score*100)
	notChosen := ""
	if !track.AutoSelectable() {
		notChosen = "; não é escolhida sozinha"
	}
	switch alignment.Verdict {
	case subtitle.VerdictMatch:
		return fmt.Sprintf("casa com o vídeo (%s); correção %s", share, describeCorrection(alignment))
	case subtitle.VerdictOtherCut:
		return fmt.Sprintf("feita para outra versão do filme (estendida? Director's Cut?), se afasta a cada cena (%s)%s", share, notChosen)
	case subtitle.VerdictNoMatch:
		return fmt.Sprintf("não casa com este vídeo (%s)%s", share, notChosen)
	}
	return "sem legenda interna para comparar" + notChosen
}

// describeCorrection shows the move a matching file gets, as "+0,036 s" and,
// when it runs at another speed, "× 0,99996".
func describeCorrection(alignment subtitle.Alignment) string {
	text := strings.Replace(fmt.Sprintf("%+.3f s", alignment.Offset.Seconds()), ".", ",", 1)
	if math.Abs(alignment.Rate-1) >= rateShownFrom {
		text += strings.Replace(fmt.Sprintf(", velocidade × %.5f", alignment.Rate), ".", ",", 1)
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
