package app

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// videoProject is the open video's project and what keeping it needs.
type videoProject struct {
	video string
	value project.Project
	// keys name the video's tracks by number, as the project knows them.
	keys map[uint64]string
	// readOnly keeps a file the app could not read whole (written by a
	// newer version, or damaged) from being overwritten; note tells why, in
	// the language of the moment.
	readOnly bool
	note     func() string
	// inherited are the lines whose file was chosen in an earlier opening
	// and not chosen again in this one: chosen before anyone knew whether it
	// fits, so a hearing may still replace it (correctInherited).
	inherited map[session.Line]bool
	// dirty tells something changed that is saved a moment later rather
	// than at once: the offsets of a held shortcut, the position, the speed,
	// the karaoke mode, the mode. flush is that pending save.
	dirty bool
	flush func() bool
	// positionAt is when the position was last kept.
	positionAt time.Time
}

// Timing of the saves that wait.
const (
	// defaultSaveDelay is how long after the last change a waiting save
	// happens: a held sync key saves once, not at every step.
	defaultSaveDelay = 2 * time.Second
	// positionEvery is how often the position is kept while playing; the
	// end of the session keeps the last one.
	positionEvery = 10 * time.Second
)

var (
	errNoPassageToSave  = worded(func() string { return ptbr.T("error.passage.noneToSave") })
	errPassageName      = worded(func() string { return ptbr.T("error.passage.name") })
	errTooManySaved     = worded(func() string { return ptbr.T("error.passage.tooMany") })
	errUnknownPassage   = worded(func() string { return ptbr.T("error.passage.unknown") })
	errDuplicatePassage = worded(func() string { return ptbr.T("error.passage.duplicate") })
	errPassageOutside   = worded(func() string { return ptbr.T("error.passage.outside") })
	errNotKept          = worded(func() string { return ptbr.T("error.project.notSaved") })
	errPassageReadOnly  = worded(func() string { return ptbr.T("error.passage.readOnly") })
	errResetWatching    = worded(func() string { return ptbr.T("error.project.resetWhileWatching") })
	errResetFailed      = worded(func() string { return ptbr.T("error.project.resetFailed") })
)

// loadProject reads the project of a video just analyzed; a file the app
// cannot read leaves an empty project that is never written over it.
func (a *API) loadProject(video string, info media.Info) *videoProject {
	p := &videoProject{video: video, value: project.New(), keys: map[uint64]string{}}
	if a.deps.Projects == nil {
		return p
	}
	for _, track := range info.Tracks {
		p.keys[track.Number] = a.deps.Projects.TrackKey(video, track)
	}
	loaded, err := a.deps.Projects.Load(video)
	switch {
	case errors.Is(err, project.ErrNewerVersion):
		p.readOnly = true
		p.note = func() string { return ptbr.T("project.newerVersion") }
	case err != nil:
		a.logger.Warn("the video's project could not be read", "video", video, "error", err)
		p.readOnly = true
		p.note = func() string { return ptbr.T("project.unreadable") }
	default:
		p.value = loaded
		p.inherited = map[session.Line]bool{
			session.LinePrimary: loaded.Choice.Primary != "", session.LineTranslation: loaded.Choice.Translation != "",
		}
	}
	return p
}

// numberOf finds the track a project key names, of the kind given and, for
// subtitles, readable.
func (p *videoProject) numberOf(key string, info media.Info, kind media.TrackType) (uint64, bool) {
	if key == "" {
		return 0, false
	}
	for _, track := range info.TracksOf(kind) {
		if p.keys[track.Number] == key && (kind != media.TrackSubtitle || track.Readable) {
			return track.Number, true
		}
	}
	return 0, false
}

// remembered puts the project's last choice over the suggestion: the tracks
// still in the video, and the study mode.
func (p *videoProject) remembered(info media.Info, suggested media.Selection, mode study.Mode) (media.Selection, study.Mode) {
	choice := p.value.Choice
	if number, found := p.numberOf(choice.Audio, info, media.TrackAudio); found {
		suggested.Audio = number
	}
	if number, found := p.numberOf(choice.Primary, info, media.TrackSubtitle); found {
		suggested.Primary = number
	}
	if number, found := p.numberOf(choice.Translation, info, media.TrackSubtitle); found {
		suggested.Translation = number
	}
	if choice.Mode.Valid() {
		mode = choice.Mode
	}
	return suggested, mode
}

// correctInherited puts, once, the file that fits in place of a line's file
// from an earlier opening when the hearing that just ended (heard) found
// that file does not fit and another in its language does. A file chosen in
// this opening stays, and so does one that no file can replace.
func (p *videoProject) correctInherited(info media.Info, heard map[uint64]subtitle.Alignment) {
	choice := &p.value.Choice
	for _, line := range []session.Line{session.LinePrimary, session.LineTranslation} {
		key, other := &choice.Primary, choice.Translation
		if line == session.LineTranslation {
			key, other = &choice.Translation, choice.Primary
		}
		kept, found := p.numberOf(*key, info, media.TrackSubtitle)
		fit, fresh := heard[kept]
		if !p.inherited[line] || !found || !fresh ||
			(fit.Verdict != subtitle.VerdictNoMatch && fit.Verdict != subtitle.VerdictOtherCut) {
			continue
		}
		keptTrack, _ := info.Track(kept)
		best := media.SelectTracks(info, media.Preferences{Primary: keptTrack.Language}).Primary
		bestTrack, found := info.Track(best)
		if !found || bestTrack.Alignment.Verdict != subtitle.VerdictMatch || p.keys[best] == "" || p.keys[best] == other ||
			bestTrack.Language.Base() != keptTrack.Language.Base() {
			continue
		}
		*key = p.keys[best]
		delete(p.inherited, line)
	}
}

// saved is what the project keeps for the lines of a session.
func (p *videoProject) saved(tracks media.Selection) map[session.Line]project.Track {
	saved := map[session.Line]project.Track{}
	for line, number := range map[session.Line]uint64{session.LinePrimary: tracks.Primary, session.LineTranslation: tracks.Translation} {
		if key := p.keys[number]; key != "" && number != 0 {
			saved[line] = p.value.Tracks[key]
		}
	}
	return saved
}

// track returns the project's track of a key, ready to be changed.
func (p *videoProject) track(key string) project.Track {
	track := p.value.Tracks[key]
	if track.Edits == nil {
		track.Edits = map[string]project.Edit{}
	}
	return track
}

// saveProject writes the project; a read-only one is kept in memory only.
// A failed save stays due, so the next one (at the latest as the session
// ends) tries again, and the panel of the video's session tells why until a
// save works. Called with a.mu held.
func (a *API) saveProject() error {
	p := a.project
	if p == nil || p.readOnly || a.deps.Projects == nil {
		return nil
	}
	var watching *watching
	if a.watching != nil && a.watching.video == p.video {
		watching = a.watching
	}
	if err := a.deps.Projects.Save(p.video, p.value); err != nil {
		a.logger.Warn("the video's project was not saved", "video", p.video, "error", err)
		p.dirty = true
		failed := a.because(errNotKept, err)
		if watching != nil {
			watching.problem.Store(failed.Error())
		}
		return failed
	}
	p.dirty = false
	if watching != nil {
		watching.problem.Store("")
	}
	return nil
}

// markDirty schedules a save of the project a moment from now, if none is
// pending; changes until then go in the same save. Called with a.mu held.
func (a *API) markDirty(p *videoProject) {
	if p.readOnly {
		return
	}
	p.dirty = true
	if p.flush != nil {
		return
	}
	delay := a.deps.SaveDelay
	if delay <= 0 {
		delay = defaultSaveDelay
	}
	run := func() { a.flushProject(p) }
	if a.deps.Later != nil {
		p.flush = a.deps.Later(delay, run)
		return
	}
	p.flush = time.AfterFunc(delay, run).Stop
}

// flushProject runs the pending save, unless the project is no longer the
// open one (another video, or forgotten in the meantime).
func (a *API) flushProject(p *videoProject) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p.flush = nil
	if a.project != p || !p.dirty {
		return
	}
	_ = a.saveProject() // logged; the next change tries again
}

// cancelFlush drops a pending save. Called with a.mu held.
func (p *videoProject) cancelFlush() {
	if p.flush != nil {
		p.flush()
		p.flush = nil
	}
}

// projectOf is the open project when it belongs to the session's video.
// Called with a.mu held.
func (a *API) projectOf(current *watching) *videoProject {
	if a.project == nil || current == nil || a.project.video != current.video {
		return nil
	}
	return a.project
}

// keepEdits takes a track's edits into the project and saves it.
func (a *API) keepEdits(current *watching, number uint64, edits map[string]project.Edit) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	key := ""
	if p != nil && number != 0 {
		key = p.keys[number]
	}
	if key == "" {
		return
	}
	track := p.track(key)
	track.Edits = edits
	p.value.Tracks[key] = track
	if err := a.saveProject(); err != nil {
		current.problem.Store(err.Error())
	}
}

// keepOffset takes a track's offset into the project, saved a moment after
// the last step: a held sync key saves once.
func (a *API) keepOffset(current *watching, number uint64, offset subtitle.Offset) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	key := ""
	if p != nil && number != 0 {
		key = p.keys[number]
	}
	if key == "" {
		return
	}
	track := p.track(key)
	track.Offset = project.MillisOf(offset.Duration())
	p.value.Tracks[key] = track
	a.markDirty(p)
}

// keepChoice remembers what the user chose to watch with, and saves.
func (a *API) keepChoice(current *watching, tracks media.Selection, mode study.Mode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil {
		return
	}
	// A line watched with another file than the one kept was chosen now,
	// knowing what fits: no hearing replaces it.
	if p.keys[tracks.Primary] != p.value.Choice.Primary {
		delete(p.inherited, session.LinePrimary)
	}
	if p.keys[tracks.Translation] != p.value.Choice.Translation {
		delete(p.inherited, session.LineTranslation)
	}
	p.value.Choice = project.Choice{
		Audio: p.keys[tracks.Audio], Primary: p.keys[tracks.Primary], Translation: p.keys[tracks.Translation], Mode: mode,
	}
	if err := a.saveProject(); err != nil {
		current.problem.Store(err.Error())
	}
}

// keepSession takes into the project how the video is being watched, at
// every state report: where it is (every positionEvery while playing),
// the speed, the karaoke mode and the study mode, saved a moment after they
// change. Values the session started with are not written: a video only
// remembers what the user changed on it. It reads the session before
// taking a.mu, which the session's listener calls take too.
func (a *API) keepSession(current *watching, final bool) {
	if current.session == nil {
		return
	}
	state := current.session.State()
	status := current.session.Passage()
	karaoke := karaokeOf(current.session)
	mode := current.session.Mode()
	now := time.Now()

	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil || p.readOnly {
		return
	}
	last := &p.value.Last
	if final || state.Status == playback.StatusPlaying && now.Sub(p.positionAt) >= positionEvery {
		if position := project.MillisOf(state.Position); position != last.Position {
			last.Position = position
			a.markDirty(p)
		}
		p.positionAt = now
	}
	// The slower laps of a progressive passage are not the video's speed.
	progressive := status.Active && status.Plan.Progressive
	if speed := float64(state.Speed); !progressive && state.Speed.Valid() && speed != last.Speed && (last.Speed != 0 || state.Speed != current.startSpeed) {
		last.Speed = speed
		a.markDirty(p)
	}
	if (last.Karaoke != nil && *last.Karaoke != karaoke) || (last.Karaoke == nil && karaoke != current.startKaraoke) {
		last.Karaoke = &karaoke
		a.markDirty(p)
	}
	if mode.Valid() && mode != p.value.Choice.Mode {
		p.value.Choice.Mode = mode
		a.markDirty(p)
	}
}

// flushSession keeps how the session ended (the last position included)
// and saves at once what waits, as the session ends; a failure stays due
// and is told with the session's end.
func (a *API) flushSession(current *watching) error {
	a.keepSession(current, true)
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.projectOf(current); p != nil {
		p.cancelFlush()
		if p.dirty {
			return a.saveProject()
		}
	}
	return nil
}

// flushProjectAtExit tries once more a save that failed, as the app closes:
// the disk may have room again. Nobody is left to tell.
func (a *API) flushProjectAtExit() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.project; p != nil && p.dirty {
		p.cancelFlush()
		_ = a.saveProject()
	}
}

// karaokeOf is the session's karaoke mode in the project's form.
func karaokeOf(s *watch.Session) project.Karaoke {
	karaoke := s.Karaoke()
	return project.Karaoke{
		Enabled: karaoke.Enabled, MinGap: project.MillisOf(karaoke.Rules.MinGap), Lead: project.MillisOf(karaoke.Rules.Lead),
		Indicator: s.Indicator(), Fill: karaoke.Fill, Repeat: s.LineRepeats(),
	}
}

// resumeOf offers to go on where the video was left (project.Last.ResumeAt).
func resumeOf(p *videoProject, duration time.Duration) Resume {
	if p == nil {
		return Resume{}
	}
	position, offered := p.value.Last.ResumeAt(duration)
	if !offered {
		return Resume{}
	}
	return Resume{Offered: true, Position: position.Seconds(), Text: ptbr.Clock(position)}
}

// SavePassage keeps the passage being studied under a name, with how it is
// studied; a passage of the same name is replaced. An empty name is the
// passage's times, as the end key names it.
func (a *API) SavePassage(name string) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	status := current.session.Passage()
	if !status.Active {
		return errNoPassageToSave
	}
	plan := project.PlanOf(status.Plan)
	passage := project.Passage{
		Name: strings.TrimSpace(name), Start: project.MillisOf(status.Segment.Start), End: project.MillisOf(status.Segment.End),
		Plan: &plan,
	}
	a.mu.Lock()
	p := a.projectOf(current)
	if p == nil {
		a.mu.Unlock()
		return errNotWatching
	}
	if passage.Name == "" {
		passage.Name = project.UniqueName(p.value.Passages, ptbr.PassageName(status.Segment.Start, status.Segment.End))
	}
	if passage.Validate() != nil {
		a.mu.Unlock()
		return errPassageName
	}
	// A project kept as it is would only seem to save the passage.
	if p.readOnly {
		a.mu.Unlock()
		return errPassageReadOnly
	}
	passages := p.value.Passages
	if i := project.IndexOf(passages, passage.Name); i >= 0 {
		passages[i] = passage
	} else if len(passages) >= project.MaxPassages {
		a.mu.Unlock()
		return errTooManySaved
	} else {
		p.value.Passages = append(passages, passage)
	}
	a.tellPassages(current)
	err := a.saveProject()
	a.mu.Unlock()
	a.emit(EventPlayback, a.describe(current))
	return err
}

// keepMarkedPassage saves a passage closed by the mark shortcut, named by its
// times ("Trecho 10:38–11:30", with "(2)" when the name is taken), and says
// so on the video. It runs on the session's shortcut goroutine.
func (a *API) keepMarkedPassage(current *watching, segment study.Segment, plan study.PracticePlan) {
	a.mu.Lock()
	p := a.projectOf(current)
	if p == nil {
		a.mu.Unlock()
		return
	}
	if len(p.value.Passages) >= project.MaxPassages || p.readOnly {
		refusal := errTooManySaved
		if p.readOnly {
			refusal = errPassageReadOnly
		}
		a.mu.Unlock()
		current.session.Notify(refusal.Error())
		return
	}
	practice := project.PlanOf(plan)
	passage := project.Passage{
		Name:  project.UniqueName(p.value.Passages, ptbr.PassageName(segment.Start, segment.End)),
		Start: project.MillisOf(segment.Start), End: project.MillisOf(segment.End), Plan: &practice,
	}
	p.value.Passages = append(p.value.Passages, passage)
	a.tellPassages(current)
	err := a.saveProject()
	a.mu.Unlock()
	if err != nil {
		current.problem.Store(err.Error())
	} else {
		a.logger.Info("passage saved", "start", segment.Start.Round(time.Millisecond), "end", segment.End.Round(time.Millisecond))
		current.session.Notify(ptbr.PassageSavedNotice(passage.Name))
	}
	a.emit(EventPlayback, a.describe(current))
}

// DeletePassage forgets a saved passage, while watching or on the session
// screen.
func (a *API) DeletePassage(name string) error {
	return a.editPassages(func(passages []project.Passage) ([]project.Passage, error) {
		i := project.IndexOf(passages, name)
		if i < 0 {
			return nil, errUnknownPassage
		}
		return append(passages[:i:i], passages[i+1:]...), nil
	})
}

// RenamePassage gives a saved passage another name, unique in the video
// whatever the case; the same name in another case is allowed.
func (a *API) RenamePassage(from, to string) error {
	name := strings.TrimSpace(to)
	return a.editPassages(func(passages []project.Passage) ([]project.Passage, error) {
		i := project.IndexOf(passages, from)
		if i < 0 {
			return nil, errUnknownPassage
		}
		renamed := passages[i]
		renamed.Name = name
		if renamed.Validate() != nil {
			return nil, errPassageName
		}
		if other := project.IndexOf(passages, name); other >= 0 && other != i {
			return nil, errDuplicatePassage
		}
		changed := append([]project.Passage(nil), passages...)
		changed[i] = renamed
		return changed, nil
	})
}

// editPassages changes the open video's passages and saves them, then
// shows the change on the panel when a session runs.
func (a *API) editPassages(change func([]project.Passage) ([]project.Passage, error)) error {
	current := a.current()
	a.mu.Lock()
	p := a.project
	if p == nil {
		a.mu.Unlock()
		return errNoVideo
	}
	passages, err := change(p.value.Passages)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	p.value.Passages = passages
	if current != nil {
		a.tellPassages(current)
	}
	err = a.saveProject()
	a.mu.Unlock()
	if current != nil {
		a.emit(EventPlayback, a.describe(current))
	}
	return err
}

// SavedPassages lists the open video's passages, for the session screen.
func (a *API) SavedPassages() []SavedPassage {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.video == nil {
		return []SavedPassage{}
	}
	return passagesOf(a.project, a.video.Info.Duration)
}

// savedPassages lists the open video's passages for the panel.
func (a *API) savedPassages(current *watching) ([]SavedPassage, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil {
		return []SavedPassage{}, ""
	}
	return passagesOf(p, current.duration), p.noteText()
}

// noteText is why the project is only read, empty when it is not.
func (p *videoProject) noteText() string {
	if p.note == nil {
		return ""
	}
	return p.note()
}

// tellPassages gives the session the video's saved passages, for the player
// window's menu. Called with a.mu held; the session only stores them.
func (a *API) tellPassages(current *watching) {
	p := a.projectOf(current)
	if p == nil || current.session == nil {
		return
	}
	list := make([]watch.SavedPassage, len(p.value.Passages))
	for i, passage := range p.value.Passages {
		list[i] = watch.SavedPassage{Name: passage.Name, Segment: passage.Segment(), Plan: passage.Practice()}
	}
	current.session.SetSavedPassages(list)
}

// passagesOf describes a project's passages; duration (zero when unknown)
// tells which ones lie past the end of the video.
func passagesOf(p *videoProject, duration time.Duration) []SavedPassage {
	list := []SavedPassage{}
	if p == nil {
		return list
	}
	for _, passage := range p.value.Passages {
		plan := passage.Practice()
		list = append(list, SavedPassage{
			Name: passage.Name, Start: ptbr.PassageClock(passage.Start.Duration()), End: ptbr.PassageClock(passage.End.Duration()),
			Plan: planOf(plan), Summary: ptbr.PlanSummary(plan), Outside: passage.Outside(duration),
		})
	}
	return list
}

// planOf describes a practice plan for the window.
func planOf(plan study.PracticePlan) PassagePlan {
	return PassagePlan{Loop: plan.Mode == study.EndLoop, Gap: plan.Gap.Seconds(), Progressive: plan.Progressive, Ladder: plan.Ladder, Goal: plan.Goal}
}

// ResetProject forgets everything the app kept about the open video: its
// corrections, sync, saved passages, choices and where it was left. Only
// with no video playing; the video then opens again, as new.
func (a *API) ResetProject() (Video, error) {
	if a.current() != nil {
		return Video{}, errResetWatching
	}
	a.mu.Lock()
	video, p := a.video, a.project
	if video == nil {
		a.mu.Unlock()
		return Video{}, errNoVideo
	}
	wasReadOnly := false
	if p != nil {
		// Nothing pending may write the old project back after it is gone.
		p.cancelFlush()
		wasReadOnly, p.readOnly = p.readOnly, true
	}
	a.resume = nil
	a.mu.Unlock()
	if a.deps.Projects != nil {
		if err := a.deps.Projects.Remove(video.Path); err != nil {
			a.logger.Warn("the video's project was not removed", "video", video.Path, "error", err)
			// Still there, the project keeps being kept: what changes next is
			// saved as before, and what waited is saved again.
			if p != nil {
				a.mu.Lock()
				p.readOnly = wasReadOnly
				if p.dirty {
					a.markDirty(p)
				}
				a.mu.Unlock()
			}
			return Video{}, a.because(errResetFailed, err)
		}
	}
	a.logger.Info("video's project forgotten", "name", filepath.Base(video.Path))
	return a.OpenVideo(video.Path)
}
