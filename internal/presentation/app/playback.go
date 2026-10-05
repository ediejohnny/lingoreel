package app

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// stateInterval is how often the window hears the playback position: often
// enough for a smooth progress bar, rare enough to cost nothing.
const stateInterval = 250 * time.Millisecond

// Lines as the window names them.
const (
	linePrimary     = "primary"
	lineTranslation = "translation"
)

var (
	errNoVideo      = errors.New("Abra um vídeo antes de assistir.")
	errNotWatching  = errors.New("Nenhum vídeo está tocando.")
	errNotSupported = errors.New("O player em uso não permite essa ação.")
	errUnknownLine  = errors.New("Legenda desconhecida.")
	errUnknownMode  = errors.New("Escolha um modo de estudo.")
)

// watching is the running session and what the window needs to describe it.
type watching struct {
	session *watch.Session
	names   ptbr.LineNames
	// stop ends the state reports.
	stop chan struct{}
	// video is the file watched; keys name its lines' tracks in the project.
	video string
	keys  [2]string
	// problem is the last project file failure, for the panel.
	problem atomic.Value // string
	// player, frameRate and duration (from the analysis) are for the
	// diagnostics.
	player    player.Installation
	frameRate float64
	duration  time.Duration
	// startSpeed and startKaraoke are what the session started with: the
	// video keeps only what the user changed from them.
	startSpeed   playback.Speed
	startKaraoke project.Karaoke
}

// Watch opens the video in the chosen player with the chosen tracks. A
// session already running ends first.
func (a *API) Watch(request WatchRequest) error {
	ctx, err := a.context()
	if err != nil {
		return err
	}
	a.mu.Lock()
	video := a.video
	a.mu.Unlock()
	if video == nil {
		return errNoVideo
	}
	if request.Mode != "" && !study.Mode(request.Mode).Valid() {
		return errUnknownMode
	}
	installation, err := a.installation(request.Player)
	if err != nil {
		return err
	}
	// A new video replaces the one playing; the window is not told it ended.
	a.stop(false)
	if request.AlwaysUse {
		if err := a.preferPlayer(request.Player); err != nil {
			return err
		}
	}
	tracks := media.Selection{Audio: request.Audio, Primary: request.Primary, Translation: request.Translation}
	a.mu.Lock()
	profile := a.profile()
	syncNotice := a.config.SyncNotice
	var saved map[session.Line]project.Track
	var keys [2]string
	var last project.Last
	var passages []project.Passage
	opened := a.project
	if opened != nil && opened.video == video.Path {
		saved = opened.saved(tracks)
		keys = [2]string{opened.keys[tracks.Primary], opened.keys[tracks.Translation]}
		last, passages = opened.value.Last, opened.value.Passages
	} else {
		opened = nil
	}
	a.mu.Unlock()

	mode := profile.Study.Mode
	if request.Mode != "" {
		mode = study.Mode(request.Mode)
	}
	names := ptbr.NewLineNames(video.Info, tracks.Primary, tracks.Translation)
	karaoke := profile.Study.Karaoke
	watchRequest := watch.Request{
		Analysis: *video, Tracks: tracks, Player: installation, Steps: profile.Sync.Nudge(),
		Karaoke:   session.Karaoke{Enabled: karaoke.Enabled, Rules: karaoke.Rules(), Fill: karaoke.Fill},
		Indicator: karaoke.Indicator, LineRepeats: karaoke.Repeat,
		Speed: profile.Speed, Shortcuts: profile.Shortcuts, Reveal: profile.Study.Reveal, NoSyncNotice: !syncNotice,
		Mode: mode, PrecisionStep: profile.Sync.PrecisionSeekStep.Std(), Saved: saved,
	}
	// What the user changed on this video comes back over the profile.
	if last.Speed != 0 {
		watchRequest.Speed = playback.Speed(last.Speed)
	}
	if k := last.Karaoke; k != nil {
		watchRequest.Karaoke = session.Karaoke{Enabled: k.Enabled, Rules: k.Rules(), Fill: k.Fill}
		watchRequest.Indicator, watchRequest.LineRepeats = k.Indicator, k.Repeat
	}
	if request.Passage != "" {
		i := project.IndexOf(passages, request.Passage)
		if i < 0 {
			return errUnknownPassage
		}
		passage := passages[i]
		if outside(passage, video.Info.Duration) {
			return errPassageOutside
		}
		watchRequest.Passage = &watch.PassageStart{Segment: passage.Segment(), Plan: passage.Practice()}
	}
	if a.deps.Window != nil {
		watchRequest.Window = a.deps.Window(video.Info)
	}
	a.mu.Lock()
	// A session the player lost goes back to where it was; else, asked to,
	// where the video was left.
	if resume := a.resume; request.Resume && resume != nil && resume.video == video.Path {
		watchRequest.Start = resume.at
	} else if offer := resumeOf(opened, video.Info.Duration); request.Resume && offer.Offered {
		watchRequest.Start = last.Position.Duration()
	}
	a.resume = nil
	a.mu.Unlock()
	current := &watching{
		names: names, stop: make(chan struct{}), video: video.Path, keys: keys,
		player: installation, frameRate: frameRateOf(video.Info), duration: video.Info.Duration,
		startSpeed: cmp.Or(watchRequest.Speed, playback.NormalSpeed),
	}
	started, err := watch.Start(ctx, a.deps.Ports, watchRequest, sessionListener{api: a, current: current}, names.Notice)
	if err != nil {
		a.logger.Warn("the session did not start", "player", request.Player, "error", err)
		return a.startFailure(err, installation)
	}
	current.session = started
	current.startKaraoke = karaokeOf(started)
	a.mu.Lock()
	a.watching = current
	a.tellPassages(current)
	a.mu.Unlock()
	a.logger.Info("session started", "player", installation.ID, "audio", trackOf(video.Info, tracks.Audio),
		"primary", trackOf(video.Info, tracks.Primary), "translation", trackOf(video.Info, tracks.Translation), "mode", mode)
	a.keepChoice(current, tracks, mode)
	go a.report(current)
	return nil
}

// startFailure words why a session did not start (PRD §52), with what the
// user can do.
func (a *API) startFailure(err error, installation player.Installation) error {
	name := string(installation.ID)
	if plugin, found := a.deps.Registry.Plugin(installation.ID); found {
		name = plugin.Descriptor().Name
	}
	switch {
	case errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound):
		return fmt.Errorf("O %s não foi encontrado em %s. Confira o caminho em Configurações › Player, ou instale-o de novo.", name, installation.Path)
	case errors.Is(err, watch.ErrSubtitles):
		return errors.New("Não foi possível ler as legendas escolhidas. Escolha outra legenda ou confira o arquivo dela.")
	case errors.Is(err, watch.ErrDisplay):
		return errors.New("Não foi possível abrir a janela do vídeo com as legendas. Tente de novo; se continuar, reinicie o LingoReel.")
	case errors.Is(err, watch.ErrLaunch):
		return fmt.Errorf("O %s não abriu a tempo. Clique em Assistir para tentar de novo; se continuar, feche outras janelas do %s.", name, name)
	}
	return errors.New("Não foi possível começar a sessão. Tente de novo.")
}

// healthText words the player's health for the panel and the video.
func healthText(health watch.Health) string {
	switch health {
	case watch.HealthReconnecting:
		return "O player não está respondendo. Reconectando…"
	case watch.HealthStalled:
		return "O player parece travado: as legendas esperam por ele."
	}
	return ""
}

// trackOf describes a track for the log: "4 en S_TEXT/UTF8", or "none".
func trackOf(info media.Info, number uint64) string {
	for _, track := range info.Tracks {
		if track.Number == number {
			return fmt.Sprintf("%d %s %s", track.Number, track.Language, track.Codec)
		}
	}
	return "none"
}

// preferPlayer makes a player the active profile's preferred one.
func (a *API) preferPlayer(id string) error {
	return a.updateProfile(func(profile *settings.Profile) { profile.PreferredPlayer = id })
}

// updateProfile changes the active profile and saves the profiles.
func (a *API) updateProfile(change func(*settings.Profile)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.profiles.Items {
		if a.profiles.Items[i].Name == a.profile().Name {
			change(&a.profiles.Items[i])
			return a.saveProfiles()
		}
	}
	return nil
}

// report sends the playback state until the session ends, then the end.
func (a *API) report(current *watching) {
	ticker := time.NewTicker(stateInterval)
	defer ticker.Stop()
	health := ""
	for {
		select {
		case <-ticker.C:
			state := a.describe(current)
			if state.Health != health {
				// A change of health also shows on the video, where the user looks.
				health = state.Health
				current.session.Notify(health)
				if health != "" {
					a.logger.Warn("player health", "state", health)
				}
			}
			a.emit(EventPlayback, state)
			a.keepSession(current, false)
		case <-current.session.Done():
			a.mu.Lock()
			if a.watching == current {
				a.watching = nil
			}
			a.mu.Unlock()
			a.flushSession(current)
			ended := Ended{}
			switch err := current.session.Err(); {
			case errors.Is(err, playback.ErrPlayerLost):
				a.logger.Warn("session ended: the player was lost", "error", err)
				a.mu.Lock()
				a.resume = &resumePoint{video: current.video, at: current.session.State().Position}
				a.mu.Unlock()
				ended.Error = "O player fechou ou parou de responder, e a sessão foi encerrada."
				ended.Resume = true
			case err != nil:
				a.logger.Warn("session ended with an error", "error", err)
			default:
				a.logger.Info("session ended")
			}
			a.emit(EventPlayback, Playback{})
			a.emit(EventEnded, ended)
			return
		case <-current.stop:
			return
		}
	}
}

// describe turns the session's state into what the panel shows.
func (a *API) describe(current *watching) Playback {
	state := current.session.State()
	capabilities := current.session.Controls().Capabilities()
	saved, note := a.savedPassages(current)
	if problem, _ := current.problem.Load().(string); problem != "" {
		note = problem
	}
	markedStart := ""
	if start, set := current.session.MarkedStart(); set {
		markedStart = ptbr.PassageClock(start)
	}
	plan, snap := current.session.PassageOptions()
	return Playback{
		MarkedStart:    markedStart,
		PassageOptions: PassageOptions{Snap: snap, Plan: planOf(plan)},
		Active:         true, Status: string(state.Status),
		Position: state.Position.Seconds(), Duration: state.Duration.Seconds(), Speed: float64(state.Speed),
		PositionText: ptbr.Clock(state.Position), DurationText: ptbr.Clock(state.Duration),
		PrimaryOffset:      ptbr.Offset(current.session.Offset(session.LinePrimary)),
		TranslationOffset:  ptbr.Offset(current.session.Offset(session.LineTranslation)),
		TranslationVisible: current.session.TranslationVisible(),
		Mode:               string(current.session.Mode()),
		Capabilities:       Capabilities{Pause: capabilities.Pause, Seek: capabilities.Seek, Speed: capabilities.Speed},
		Lines:              LineNames{Primary: current.names[session.LinePrimary], Translation: current.names[session.LineTranslation]},
		Karaoke:            describeKaraoke(current.session),
		Passage:            describePassage(current.session.Passage()),
		Editing:            describeEditing(current),
		SavedPassages:      saved,
		ProjectNote:        note,
		Health:             healthText(current.session.Health()),
	}
}

// sessionListener tells the window of sync changes and edits at once, and
// keeps them in the video's project.
type sessionListener struct {
	api     *API
	current *watching
}

func (l sessionListener) SubtitlesChanged(time.Duration, session.Visible) {}

func (l sessionListener) Synced(line session.Line, offset subtitle.Offset) {
	l.api.keepOffset(l.current, line, offset)
	if l.current.session != nil {
		l.api.emit(EventPlayback, l.api.describe(l.current))
	}
}

func (l sessionListener) Edited(line session.Line, edits map[string]project.Edit) {
	l.api.keepEdits(l.current, line, edits)
}

func (l sessionListener) PassageMarked(segment study.Segment, plan study.PracticePlan) {
	l.api.keepMarkedPassage(l.current, segment, plan)
}

func (a *API) current() *watching {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.watching
}

// control runs a command on the running session and reports the new state.
func (a *API) control(command func(*watching) error) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	err := command(current)
	if errors.Is(err, session.ErrNotSupported) {
		return errNotSupported
	}
	if err != nil {
		a.logger.Warn("playback command failed", "error", err)
		return errors.New("O player não respondeu ao comando.")
	}
	a.emit(EventPlayback, a.describe(current))
	return nil
}

// TogglePause pauses or resumes the video.
func (a *API) TogglePause() error {
	return a.control(func(w *watching) error { return w.session.Controls().TogglePause() })
}

// SeekBy moves the video by some seconds; negative goes back.
func (a *API) SeekBy(seconds float64) error {
	return a.control(func(w *watching) error { return w.session.Controls().SeekBy(secondsOf(seconds)) })
}

// SeekTo moves the video to a position in seconds.
func (a *API) SeekTo(seconds float64) error {
	return a.control(func(w *watching) error { return w.session.Controls().SeekTo(secondsOf(seconds)) })
}

// SetSpeed changes the playback speed.
func (a *API) SetSpeed(speed float64) error {
	return a.control(func(w *watching) error { return w.session.Controls().SetSpeed(playback.Speed(speed)) })
}

// Nudge moves one subtitle line ("primary" or "translation") one step.
func (a *API) Nudge(line string, later bool) error {
	which, err := lineOf(line)
	if err != nil {
		return err
	}
	return a.control(func(w *watching) error {
		w.session.Nudge(which, later)
		return nil
	})
}

// ResetSync puts a subtitle line back at its file's timing.
func (a *API) ResetSync(line string) error {
	which, err := lineOf(line)
	if err != nil {
		return err
	}
	return a.control(func(w *watching) error {
		w.session.ResetSync(which)
		return nil
	})
}

// SetTranslationVisible shows or hides the translation.
func (a *API) SetTranslationVisible(visible bool) error {
	return a.control(func(w *watching) error {
		w.session.SetTranslationVisible(visible)
		return nil
	})
}

// PlaybackState is the current state, for a window that just opened.
func (a *API) PlaybackState() Playback {
	if current := a.current(); current != nil && current.session != nil {
		return a.describe(current)
	}
	return Playback{}
}

// StopWatching closes the player and ends the session.
func (a *API) StopWatching() error {
	if !a.stop(true) {
		return errNotWatching
	}
	return nil
}

// stop ends the running session, if any, and reports whether there was one;
// tell sends the end to the window.
func (a *API) stop(tell bool) bool {
	a.mu.Lock()
	current := a.watching
	a.watching = nil
	a.mu.Unlock()
	if current == nil || current.session == nil {
		return false
	}
	close(current.stop)
	if err := current.session.Stop(); err != nil && !errors.Is(err, playback.ErrPlayerLost) {
		a.logger.Warn("the session did not end cleanly", "error", err)
	}
	a.logger.Info("session ended", "by", "the app")
	a.flushSession(current)
	if tell {
		a.emit(EventPlayback, Playback{})
		a.emit(EventEnded, Ended{})
	}
	return true
}

// lineName names a line as the window does.
func lineName(line session.Line) string {
	if line == session.LineTranslation {
		return lineTranslation
	}
	return linePrimary
}

func lineOf(name string) (session.Line, error) {
	switch name {
	case linePrimary:
		return session.LinePrimary, nil
	case lineTranslation:
		return session.LineTranslation, nil
	}
	return 0, errUnknownLine
}

// secondsOf converts the window's seconds, refusing what no video has.
func secondsOf(seconds float64) time.Duration {
	if !(seconds > -maxSeekSeconds && seconds < maxSeekSeconds) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// maxSeekSeconds bounds positions and jumps coming from the window (a day).
const maxSeekSeconds = 24 * 60 * 60
