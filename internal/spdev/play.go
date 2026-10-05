package spdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/plugins"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/hotkeys"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/placement"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

var (
	// ErrNoPlayer reports that no supported player is installed.
	ErrNoPlayer = errors.New("nenhum player compatível encontrado")
	// ErrInvalidOption reports an unknown option or a malformed value.
	ErrInvalidOption = errors.New("opção inválida")
)

const (
	playerFlag = "--player"
	startFlag  = "--inicio"
	pausedFlag = "--pausado"
	stepFlag   = "--passo"

	// maxStart keeps --inicio within a sane range before the video duration
	// is known; the player itself clamps to the real duration.
	maxStart = 24 * time.Hour
)

type playOptions struct {
	path      string
	preferred player.ID
	start     time.Duration
	paused    bool
	steps     subtitle.NudgeSteps
}

// play opens a video in the chosen player and prints the subtitles as they
// change, until the player exits or the context ends; withOverlay also draws
// them over the video.
func play(ctx context.Context, args []string, stdout io.Writer, withOverlay bool) error {
	options, err := parsePlayOptions(args)
	if err != nil {
		return err
	}

	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	result, err := service.Analyze(ctx, options.path, defaultPreferences())
	if err != nil {
		return err
	}
	if result.Info.Duration > 0 && options.start >= result.Info.Duration {
		return fmt.Errorf("%w: %s %s está além do fim do vídeo (%s)", ErrInvalidOption, startFlag,
			formatClock(options.start), formatClock(result.Info.Duration))
	}
	registry, hints, err := plugins.Registry(stderrLogger())
	if err != nil {
		return err
	}
	installed := discovery.Detect(discovery.SystemEnvironment{}, hints, nil)
	choice, found := player.Choose(installed, registry.Supported, options.preferred)
	if !found {
		return fmt.Errorf("%w (suportados agora: %s)", ErrNoPlayer, supportedNames(registry))
	}

	out := &printer{w: stdout}
	if choice.PreferredMissing {
		out.line("Aviso: %s não está instalado ou ainda não é suportado.", options.preferred.DisplayName())
	}
	out.line("Player:    %s  %s", describeInstallation(choice.Installation), choice.Installation.Path)
	out.line("Legenda:   %s", describeChoice(result.Info, result.Suggested.Primary))
	out.line("Tradução:  %s", describeChoice(result.Info, result.Suggested.Translation))
	if out.err != nil {
		return out.err
	}

	ports := watch.Ports{
		Service: service, Registry: registry,
		StartShortcuts: func(target watch.ShortcutTarget, chords []shortcut.Chord) (watch.Shortcuts, error) {
			return hotkeys.StartGlobal(target, chords, stderrLogger())
		},
	}
	if withOverlay {
		ports.OpenWindow, ports.OpenDisplay = openPlayerWindow, openOverlay
	}
	names := ptbr.NewLineNames(result.Info, result.Suggested.Primary, result.Suggested.Translation)
	listener := &consoleListener{out: out, names: names, labels: [2]string{
		lineLabel(result.Info, result.Suggested.Primary, "Legenda"),
		lineLabel(result.Info, result.Suggested.Translation, "Tradução"),
	}}
	watching, err := watch.Start(ctx, ports, watch.Request{
		Analysis: result, Tracks: result.Suggested, Player: choice.Installation,
		Start: options.start, Paused: options.paused, Window: placement.ForVideo(result.Info), Steps: options.steps,
	}, listener, names.Notice)
	if err != nil {
		return err
	}
	for _, number := range []uint64{result.Suggested.Primary, result.Suggested.Translation} {
		if track, found := watching.Loaded()[number]; found && track.Corrected {
			out.line("Sincronia: %s ajustada automaticamente para casar com o vídeo: %s", trackLabel(number), describeCorrection(track.Correction))
		}
	}
	out.line("Precisão:  %s", describePrecision(watching.Controls().Capabilities().Precision))
	if withOverlay {
		out.line("Overlay:   legendas desenhadas sobre o vídeo")
	}
	out.line("Sincronia: Shift+, / Shift+. movem a legenda de cima, Ctrl+, / Ctrl+. a de baixo (passo %s); z + Enter zera.",
		strings.TrimPrefix(ptbr.Offset(subtitle.Offset(options.steps.Step)), "+"))
	out.line("Feche o player ou pressione Ctrl+C para encerrar.")
	out.line("")
	go readConsole(ctx, os.Stdin, func(command string) {
		if command == resetCommand {
			watching.ResetSync(session.LinePrimary)
			watching.ResetSync(session.LineTranslation)
		}
	})

	select {
	case <-watching.Done():
	case <-ctx.Done():
	}
	err = watching.Stop()
	if errors.Is(err, playback.ErrPlayerLost) {
		err = fmt.Errorf("o %s parou de responder: %w", choice.Installation.ID.DisplayName(), err)
	}
	if err == nil {
		out.line("Sessão encerrada.")
	}
	return errors.Join(err, out.err)
}

// consoleListener prints what happens in the session.
type consoleListener struct {
	out    *printer
	names  ptbr.LineNames
	labels [2]string
}

func (l *consoleListener) SubtitlesChanged(position time.Duration, visible session.Visible) {
	l.out.line("[%s] %s", formatClock(position.Truncate(time.Millisecond)), describeVisible(visible, l.labels))
}

func (l *consoleListener) Synced(line session.Line, offset subtitle.Offset) {
	l.out.line("Sincronia: %s", l.names.Notice(line, offset))
}

// Edited is never called: the development tool does not edit lines.
func (l *consoleListener) Edited(session.Line, map[string]project.Edit) {}

// PassageMarked prints a passage closed by the mark shortcut; the
// development tool keeps no project to save it in.
func (l *consoleListener) PassageMarked(segment study.Segment, _ study.PracticePlan) {
	l.out.line("Trecho marcado: %s", ptbr.PassageName(segment.Start, segment.End))
}

func parsePlayOptions(args []string) (playOptions, error) {
	options := playOptions{steps: subtitle.DefaultNudgeSteps()}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case playerFlag, startFlag, stepFlag:
			if i+1 >= len(args) {
				return playOptions{}, fmt.Errorf("%w: %s precisa de um valor", ErrInvalidOption, arg)
			}
			i++
			switch arg {
			case playerFlag:
				options.preferred = player.ID(strings.ToLower(args[i]))
			case stepFlag:
				steps, err := parseStep(args[i])
				if err != nil {
					return playOptions{}, err
				}
				options.steps = steps
			default:
				start, err := parseSeconds(args[i])
				if err != nil {
					return playOptions{}, err
				}
				options.start = start
			}
		case pausedFlag:
			options.paused = true
		default:
			if strings.HasPrefix(arg, "--") || options.path != "" {
				return playOptions{}, fmt.Errorf("%w: %q", ErrInvalidOption, arg)
			}
			options.path = arg
		}
	}
	if options.path == "" {
		return playOptions{}, fmt.Errorf("%w: caminho do vídeo", ErrMissingArgument)
	}
	return options, nil
}

// parseStep reads the sync step in milliseconds; holding a shortcut moves
// by the default fast step, or by the step when it is larger.
func parseStep(value string) (subtitle.NudgeSteps, error) {
	milliseconds, err := strconv.Atoi(value)
	steps := subtitle.NudgeSteps{Step: time.Duration(milliseconds) * time.Millisecond}
	steps.FastStep = max(subtitle.DefaultFastNudgeStep, steps.Step)
	if err != nil || steps.Validate() != nil {
		return subtitle.NudgeSteps{}, fmt.Errorf("%w: %s %q (use milissegundos, de %d a %d)", ErrInvalidOption, stepFlag, value,
			subtitle.MinNudgeStep.Milliseconds(), subtitle.MaxNudgeStep.Milliseconds())
	}
	return steps, nil
}

// parseSeconds reads a start position in seconds; a comma works as the
// decimal separator too.
func parseSeconds(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(strings.Replace(value, ",", ".", 1), 64)
	if err != nil || !(seconds >= 0 && seconds <= maxStart.Seconds()) {
		return 0, fmt.Errorf("%w: %s %q (use segundos, de 0 a %d)", ErrInvalidOption, startFlag, value, int(maxStart.Seconds()))
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func describeInstallation(installation player.Installation) string {
	if installation.Version == "" {
		return installation.ID.DisplayName()
	}
	return installation.ID.DisplayName() + " " + installation.Version
}

func describePrecision(precision playback.Precision) string {
	if precision == playback.PrecisionExact {
		return "exata"
	}
	return "estimada (o player informa o tempo algumas vezes por segundo)"
}

// lineLabel names a subtitle line by its language, or by its role when the
// language is unknown.
func lineLabel(info media.Info, number uint64, role string) string {
	if track, found := info.Track(number); found && track.Language != "" {
		return string(track.Language)
	}
	return role
}

func describeVisible(visible session.Visible, labels [2]string) string {
	primary, translation := joinText(visible.Primary), joinText(visible.Translation)
	if primary == "" && translation == "" {
		return "—"
	}
	return fmt.Sprintf("%s: %s  │  %s: %s", labels[0], orDash(primary), labels[1], orDash(translation))
}

func joinText(events []subtitle.Event) string {
	texts := make([]string, 0, len(events))
	for _, event := range events {
		texts = append(texts, strings.ReplaceAll(event.Text(), "\n", " / "))
	}
	return strings.Join(texts, " / ")
}

func orDash(text string) string {
	if text == "" {
		return "—"
	}
	return text
}
