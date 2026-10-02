package app

import (
	"errors"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
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
)

// watching is the running session and what the window needs to describe it.
type watching struct {
	session *watch.Session
	names   ptbr.LineNames
	// stop ends the state reports.
	stop chan struct{}
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
	a.mu.Lock()
	profile := a.profile()
	syncNotice := a.config.SyncNotice
	a.mu.Unlock()

	tracks := media.Selection{Audio: request.Audio, Primary: request.Primary, Translation: request.Translation}
	names := ptbr.NewLineNames(video.Info, tracks.Primary, tracks.Translation)
	karaoke := profile.Study.Karaoke
	watchRequest := watch.Request{
		Analysis: *video, Tracks: tracks, Player: installation, Steps: profile.Sync.Nudge(),
		Karaoke:   session.Karaoke{Enabled: karaoke.Enabled, Rules: karaoke.Rules(), Fill: karaoke.Fill},
		Indicator: karaoke.Indicator, LineRepeats: karaoke.Repeat,
		Speed: profile.Speed, Shortcuts: profile.Shortcuts, Reveal: profile.Study.Reveal, NoSyncNotice: !syncNotice,
	}
	if a.deps.Window != nil {
		watchRequest.Window = a.deps.Window(video.Info)
	}
	current := &watching{names: names, stop: make(chan struct{})}
	started, err := watch.Start(ctx, a.deps.Ports, watchRequest, sessionListener{api: a}, names.Notice)
	if err != nil {
		a.logger.Warn("the session did not start", "player", request.Player, "error", err)
		return errors.New("Não foi possível iniciar o player. Confira se ele está instalado e tente de novo.")
	}
	current.session = started
	a.mu.Lock()
	a.watching = current
	a.mu.Unlock()
	go a.report(current)
	return nil
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
	for {
		select {
		case <-ticker.C:
			a.emit(EventPlayback, a.describe(current))
		case <-current.session.Done():
			a.mu.Lock()
			if a.watching == current {
				a.watching = nil
			}
			a.mu.Unlock()
			ended := Ended{}
			if errors.Is(current.session.Err(), playback.ErrPlayerLost) {
				ended.Error = "O player parou de responder e a sessão foi encerrada."
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
	return Playback{
		Active: true, Status: string(state.Status),
		Position: state.Position.Seconds(), Duration: state.Duration.Seconds(), Speed: float64(state.Speed),
		PositionText: ptbr.Clock(state.Position), DurationText: ptbr.Clock(state.Duration),
		PrimaryOffset:      ptbr.Offset(current.session.Offset(session.LinePrimary)),
		TranslationOffset:  ptbr.Offset(current.session.Offset(session.LineTranslation)),
		TranslationVisible: current.session.TranslationVisible(),
		Capabilities:       Capabilities{Pause: capabilities.Pause, Seek: capabilities.Seek, Speed: capabilities.Speed},
		Lines:              LineNames{Primary: current.names[session.LinePrimary], Translation: current.names[session.LineTranslation]},
		Karaoke:            describeKaraoke(current.session),
		Passage:            describePassage(current.session.Passage()),
	}
}

// sessionListener sends sync changes to the window at once.
type sessionListener struct {
	api *API
}

func (l sessionListener) SubtitlesChanged(time.Duration, session.Visible) {}

func (l sessionListener) Synced(session.Line, subtitle.Offset) {
	if current := l.api.current(); current != nil {
		l.api.emit(EventPlayback, l.api.describe(current))
	}
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
	if tell {
		a.emit(EventPlayback, Playback{})
		a.emit(EventEnded, Ended{})
	}
	return true
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
