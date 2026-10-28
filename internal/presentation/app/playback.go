package app

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/fault"
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
	errNoVideo      = worded(func() string { return ptbr.T("error.noVideo") })
	errNotWatching  = worded(func() string { return ptbr.T("error.notWatching") })
	errNotSupported = worded(func() string { return ptbr.T("error.notSupported") })
	errUnknownLine  = worded(func() string { return ptbr.T("error.unknownLine") })
	errUnknownMode  = worded(func() string { return ptbr.T("error.unknownMode") })
)

// watching is the running session and what the window needs to describe it.
type watching struct {
	session *watch.Session
	// names name the lines by their tracks, which a switch changes.
	names atomic.Pointer[ptbr.LineNames]
	// stop ends the state reports.
	stop chan struct{}
	// video is the file watched.
	video string
	// switching tells a subtitle is being loaded for a line: one at a time.
	switching atomic.Bool
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
	// embedded tells the video is drawn inside the app's window; a player
	// in a window of its own closes the app when the user closes it.
	embedded bool
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
		// Not kept, the preferred player waits for the next time; the video
		// plays anyway.
		if err := a.preferPlayer(request.Player); err != nil {
			a.logger.Warn("the preferred player was not kept", "player", request.Player, "error", err)
		}
	}
	tracks := media.Selection{Audio: request.Audio, Primary: request.Primary, Translation: request.Translation}
	a.mu.Lock()
	profile := a.profile()
	syncNotice := a.config.SyncNotice
	voicePreferences := a.config.Voice
	thumbnails := a.config.Thumbnails
	var saved map[session.Line]project.Track
	var last project.Last
	var passages []project.Passage
	opened := a.project
	if opened != nil && opened.video == video.Path {
		saved = opened.saved(tracks)
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
		TranslationOwnTiming: profile.Sync.TranslationOwnTiming, VolumeTouch: profile.VolumeTouch,
		Mode: mode, PrecisionStep: profile.Sync.PrecisionSeekStep.Std(), Saved: saved,
		Voice: voiceSetup(voicePreferences), Thumbnails: thumbnailSetup(thumbnails),
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
		if passage.Outside(video.Info.Duration) {
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
	} else if position, offered := last.ResumeAt(video.Info.Duration); request.Resume && offered {
		watchRequest.Start = position
	}
	a.resume = nil
	a.mu.Unlock()
	current := &watching{
		stop: make(chan struct{}), video: video.Path,
		player: installation, frameRate: frameRateOf(video.Info), duration: video.Info.Duration,
		startSpeed: cmp.Or(watchRequest.Speed, playback.NormalSpeed),
		embedded:   a.embeds(installation.ID),
	}
	current.names.Store(&names)
	ports := a.deps.Ports
	if a.deps.OpenSpeech != nil {
		ports.OpenWords = a.openWords
	}
	started, err := watch.Start(ctx, ports, watchRequest, sessionListener{api: a, current: current}, names.Notice)
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
		"primary", trackOf(video.Info, tracks.Primary), "translation", trackOf(video.Info, tracks.Translation), "mode", mode,
		"embedded", current.embedded)
	a.keepChoice(current, tracks, mode)
	a.setTitle(ptbr.WindowTitle(filepath.Base(video.Path)))
	go a.report(current)
	return nil
}

// embeds tells whether a player's video is drawn inside the app's window.
func (a *API) embeds(id player.ID) bool {
	if a.deps.Stage == nil || a.deps.Ports.OpenWindow == nil {
		return false
	}
	plugin, found := a.deps.Registry.Plugin(id)
	return found && plugin.Descriptor().DrawsInHost
}

// startFailure words why a session did not start, with what the
// user can do.
func (a *API) startFailure(err error, installation player.Installation) error {
	name := string(installation.ID)
	if plugin, found := a.deps.Registry.Plugin(installation.ID); found {
		name = plugin.Descriptor().Name
	}
	switch {
	// The players' small files in the temporary folder: a full disk is no
	// slow player.
	case a.deps.Faults != nil && a.deps.Faults(err) == fault.DiskFull:
		return a.because(errors.New(ptbr.T("error.start.failed")), err)
	case errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound):
		return errors.New(ptbr.T("error.start.notFound", "player", name, "path", installation.Path))
	case errors.Is(err, watch.ErrSubtitles):
		return errors.New(ptbr.T("error.start.subtitles"))
	case errors.Is(err, watch.ErrDisplay):
		return errors.New(ptbr.T("error.start.display"))
	case errors.Is(err, watch.ErrLaunch):
		return errors.New(ptbr.T("error.start.launch", "player", name))
	}
	return errors.New(ptbr.T("error.start.failed"))
}

// healthText words the player's health for the panel and the video.
func healthText(health watch.Health) string {
	switch health {
	case watch.HealthReconnecting:
		return ptbr.T("health.reconnecting")
	case watch.HealthStalled:
		return ptbr.T("health.stalled")
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
	return a.changeProfiles(func(next *settings.Profiles) error {
		active := a.profile().Name
		for i := range next.Items {
			if next.Items[i].Name == active {
				change(&next.Items[i])
			}
		}
		return nil
	})
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
			// Ended by the app (stop), the session was already let go: stop
			// tells the window, or not, itself.
			byPlayer := a.watching == current
			if byPlayer {
				a.watching = nil
			}
			a.mu.Unlock()
			if !byPlayer {
				return
			}
			unsaved := a.flushSession(current)
			a.setTitle(ptbr.WindowTitle(""))
			ended := Ended{}
			switch err := current.session.Err(); {
			case errors.Is(err, playback.ErrPlayerLost):
				a.logger.Warn("session ended: the player was lost", "error", err)
				a.mu.Lock()
				a.resume = &resumePoint{video: current.video, at: current.session.State().Position}
				a.mu.Unlock()
				ended.Error = ptbr.T("error.playerLost")
				ended.Resume = true
			case err != nil:
				a.logger.Warn("session ended with an error", "error", err)
			case !current.embedded:
				// The user closed the player's own window: the app, which is one
				// with it, closes too, its work saved above.
				a.logger.Info("session ended", "by", "the player's window")
				a.quit()
				return
			default:
				a.logger.Info("session ended")
			}
			if ended.Error == "" && unsaved != nil {
				ended.Error = unsaved.Error()
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
	voiceShift := ""
	if shift := current.session.VoiceShift(); shift != 0 {
		voiceShift = ptbr.Offset(subtitle.NewOffset(shift))
	}
	return Playback{
		PrimaryVoice:   voiceShift,
		MarkedStart:    markedStart,
		PassageOptions: PassageOptions{Snap: snap, Plan: planOf(plan)},
		Active:         true, Embedded: current.embedded, Status: string(state.Status),
		Position: state.Position.Seconds(), Duration: state.Duration.Seconds(), Speed: float64(state.Speed),
		PositionText: ptbr.Clock(state.Position), DurationText: ptbr.Clock(state.Duration),
		PrimaryOffset:      ptbr.Offset(current.session.Offset(session.LinePrimary)),
		TranslationOffset:  ptbr.Offset(current.session.Offset(session.LineTranslation)),
		TranslationVisible: current.session.TranslationVisible(),
		Mode:               string(current.session.Mode()),
		Capabilities:       Capabilities{Pause: capabilities.Pause, Seek: capabilities.Seek, Speed: capabilities.Speed},
		Lines:              LineNames{Primary: current.lineNames()[session.LinePrimary], Translation: current.lineNames()[session.LineTranslation]},
		Karaoke:            describeKaraoke(current.session),
		Passage:            describePassage(current.session.Passage()),
		Editing:            describeEditing(current),
		SavedPassages:      saved,
		ProjectNote:        note,
		Health:             healthText(current.session.Health()),
		Tracks:             describeTracks(current, capabilities.AudioTrackSelection, a.preferences()),
	}
}

// describeTracks lists the session's tracks for the panel.
func describeTracks(current *watching, audioSwitch bool, preferences media.Preferences) PlaybackTracks {
	tracks := current.session.Tracks()
	info := media.Info{Tracks: tracks.List}
	return PlaybackTracks{
		Audio: audioTracks(info), Subtitles: subtitleTracks(info, preferences),
		Current:     Selection{Audio: tracks.Current.Audio, Primary: tracks.Current.Primary, Translation: tracks.Current.Translation},
		AudioSwitch: audioSwitch, Loading: current.switching.Load(),
	}
}

// sessionListener tells the window of sync changes and edits at once, and
// keeps them in the video's project.
type sessionListener struct {
	api     *API
	current *watching
}

func (l sessionListener) SubtitlesChanged(time.Duration, session.Visible) {}

func (l sessionListener) Synced(_ session.Line, track uint64, offset subtitle.Offset) {
	l.api.keepOffset(l.current, track, offset)
	if l.current.session != nil {
		l.api.emit(EventPlayback, l.api.describe(l.current))
	}
}

func (l sessionListener) Edited(_ session.Line, track uint64, edits map[string]project.Edit) {
	l.api.keepEdits(l.current, track, edits)
}

func (l sessionListener) PassageMarked(segment study.Segment, plan study.PracticePlan) {
	l.api.keepMarkedPassage(l.current, segment, plan)
}

// lineNames name the lines by the tracks they show now.
func (w *watching) lineNames() ptbr.LineNames {
	return *w.names.Load()
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
		return errors.New(ptbr.T("error.noResponse"))
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
	unsaved := a.flushSession(current)
	a.setTitle(ptbr.WindowTitle(""))
	if tell {
		ended := Ended{}
		if unsaved != nil {
			ended.Error = unsaved.Error()
		}
		a.emit(EventPlayback, Playback{})
		a.emit(EventEnded, ended)
	}
	return true
}

// quit closes the app, unless it is closing already.
func (a *API) quit() {
	a.mu.Lock()
	ui, gone := a.ui, a.shutDown || a.closing
	a.mu.Unlock()
	if ui != nil && !gone {
		ui.Quit()
	}
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
