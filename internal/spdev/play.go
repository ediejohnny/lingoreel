package spdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"lingoreel/internal/application/analysis"
	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/media/matroska"
	"lingoreel/internal/infrastructure/player/discovery"
	"lingoreel/internal/infrastructure/player/vlc"
	"lingoreel/internal/presentation/overlay"
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

	// maxStart keeps --inicio within a sane range before the video duration
	// is known; the player itself clamps to the real duration.
	maxStart = 24 * time.Hour
)

// runningPlayer is a launched player: the controls the session uses, its
// observations and a way to end it.
type runningPlayer interface {
	session.Player
	Samples() <-chan playback.Sample
	ProcessID() int
	Close() error
}

type launcher func(ctx context.Context, executable string, request player.LaunchRequest) (runningPlayer, error)

// launchers holds the players spdev can drive; the others are detected but
// not offered yet.
var launchers = map[player.ID]launcher{
	player.VLC: func(ctx context.Context, executable string, request player.LaunchRequest) (runningPlayer, error) {
		return vlc.Launch(ctx, executable, request)
	},
}

type playOptions struct {
	path      string
	preferred player.ID
	start     time.Duration
	paused    bool
}

// play opens a video in the chosen player and prints the subtitles as they
// change, until the player exits or the context ends; withOverlay also draws
// them over the video.
func play(ctx context.Context, args []string, stdout io.Writer, withOverlay bool) error {
	options, err := parsePlayOptions(args)
	if err != nil {
		return err
	}

	service := analysis.NewService(matroska.FileReader{})
	result, err := service.Analyze(options.path, defaultPreferences())
	if err != nil {
		return err
	}
	if result.Info.Duration > 0 && options.start >= result.Info.Duration {
		return fmt.Errorf("%w: %s %s está além do fim do vídeo (%s)", ErrInvalidOption, startFlag,
			formatClock(options.start), formatClock(result.Info.Duration))
	}
	chosen := []uint64{result.Suggested.Primary, result.Suggested.Translation}
	loaded, err := service.LoadSubtitles(ctx, options.path, chosen, nil)
	if err != nil {
		return err
	}

	choice, found := player.Choose(discovery.Detect(discovery.SystemEnvironment{}, nil), isDrivable, options.preferred)
	if !found {
		return fmt.Errorf("%w (suportados agora: VLC)", ErrNoPlayer)
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

	request := player.LaunchRequest{
		File:        options.path,
		AudioIndex:  audioIndex(result.Info, result.Suggested.Audio),
		StartPaused: options.paused,
		Start:       options.start,
	}
	running, err := launchers[choice.Installation.ID](ctx, choice.Installation.Path, request)
	if err != nil {
		return err
	}

	precision := running.Capabilities().Precision
	out.line("Precisão:  %s", describePrecision(precision))
	var screen *overlay.Overlay
	if withOverlay {
		if screen, err = openOverlay(ctx, running, result.Info); err != nil {
			return errors.Join(fmt.Errorf("abrindo o overlay: %w", err), running.Close())
		}
		out.line("Overlay:   legendas desenhadas sobre o vídeo")
	}
	out.line("Feche o player ou pressione Ctrl+C para encerrar.")
	out.line("")

	labels := [2]string{
		lineLabel(result.Info, result.Suggested.Primary, "Legenda"),
		lineLabel(result.Info, result.Suggested.Translation, "Tradução"),
	}
	clock := session.NewClock(precision, time.Now)
	subtitles := session.NewSubtitles(trackOf(loaded, result.Suggested.Primary), trackOf(loaded, result.Suggested.Translation))
	// The runner calls back from its own loop, so the printer needs no lock.
	runner := session.NewRunner(clock, subtitles, func(position time.Duration, visible session.Visible) {
		out.line("[%s] %s", formatClock(position.Truncate(time.Millisecond)), describeVisible(visible, labels))
		if screen != nil {
			if err := screen.Show(overlayContent(visible)); err != nil {
				out.line("Aviso: o overlay não foi atualizado: %v", err)
			}
		}
	}, session.NewRealTimer)

	err = runner.Run(ctx, running.Samples())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		err = nil
	}
	var overlayErr error
	if screen != nil {
		if overlayErr = screen.Close(); overlayErr != nil {
			overlayErr = fmt.Errorf("ao fechar o overlay: %w", overlayErr)
		}
	}
	closeErr := running.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("ao fechar o %s: %w", choice.Installation.ID.DisplayName(), closeErr)
	}
	if err == nil {
		out.line("Sessão encerrada.")
	}
	return errors.Join(err, overlayErr, closeErr, out.err)
}

func parsePlayOptions(args []string) (playOptions, error) {
	var options playOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case playerFlag, startFlag:
			if i+1 >= len(args) {
				return playOptions{}, fmt.Errorf("%w: %s precisa de um valor", ErrInvalidOption, arg)
			}
			i++
			if arg == playerFlag {
				options.preferred = player.ID(strings.ToLower(args[i]))
				continue
			}
			start, err := parseSeconds(args[i])
			if err != nil {
				return playOptions{}, err
			}
			options.start = start
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

// parseSeconds reads a start position in seconds; a comma works as the
// decimal separator too.
func parseSeconds(value string) (time.Duration, error) {
	seconds, err := strconv.ParseFloat(strings.Replace(value, ",", ".", 1), 64)
	if err != nil || !(seconds >= 0 && seconds <= maxStart.Seconds()) {
		return 0, fmt.Errorf("%w: %s %q (use segundos, de 0 a %d)", ErrInvalidOption, startFlag, value, int(maxStart.Seconds()))
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func isDrivable(id player.ID) bool {
	_, found := launchers[id]
	return found
}

// audioIndex converts a track number into the position among audio tracks
// that players expect; -1 keeps the player's own choice.
func audioIndex(info media.Info, number uint64) int {
	for index, track := range info.TracksOf(media.TrackAudio) {
		if track.Number == number {
			return index
		}
	}
	return -1
}

func trackOf(loaded map[uint64]analysis.LoadedTrack, number uint64) *session.Track {
	track, found := loaded[number]
	if !found {
		return nil
	}
	return &session.Track{Timeline: track.Timeline}
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
