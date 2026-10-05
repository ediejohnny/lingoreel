package app

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

var (
	errUnknownTrack   = worded(func() string { return ptbr.T("error.track.unknown") })
	errSameTrack      = worded(func() string { return ptbr.T("error.track.sameAsOther") })
	errNeedsPrimary   = worded(func() string { return ptbr.T("error.track.needsPrimary") })
	errTrackLoading   = worded(func() string { return ptbr.T("error.track.loading") })
	errTrackNotLoaded = worded(func() string { return ptbr.T("error.track.notLoaded") })
	errSubtitleFile   = worded(func() string { return ptbr.T("error.subtitleFile.unreadable") })
	errSubtitleDialog = worded(func() string { return ptbr.T("error.subtitleFile.dialog") })
	errTooManyFiles   = worded(func() string {
		return ptbr.T("error.subtitleFile.tooMany", "max", project.MaxAddedFiles)
	})
)

// SwitchSubtitle puts another subtitle on a line while the video plays: a
// track of the video or a file found or added for it; zero takes the
// translation away. The track loads outside the locks (a track inside a long
// video can take seconds) with a notice on the video, one switch at a time,
// and the session takes it only if it is still the one watched.
func (a *API) SwitchSubtitle(line string, number uint64) error {
	which, err := lineOf(line)
	if err != nil {
		return err
	}
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	return a.switchLine(current, which, number)
}

func (a *API) switchLine(current *watching, line session.Line, number uint64) error {
	ctx, err := a.context()
	if err != nil {
		return err
	}
	a.mu.Lock()
	video := a.video
	a.mu.Unlock()
	if video == nil || video.Path != current.video {
		return errNotWatching
	}
	if err := lineRefusal(current.session.CheckLine(line, number)); err != nil {
		return err
	}
	if current.session.Tracks().Number(line) == number {
		return nil
	}
	if !current.switching.CompareAndSwap(false, true) {
		return errTrackLoading
	}
	defer current.switching.Store(false)

	var loaded analysis.LoadedTrack
	if number != 0 {
		current.session.Notify(ptbr.T("notice.subtitleLoading"))
		a.emit(EventPlayback, a.describe(current))
		all, err := a.deps.Service.LoadSubtitles(ctx, *video, []uint64{number}, nil)
		if err != nil {
			a.logger.Warn("the subtitle could not be loaded", "track", trackOf(video.Info, number), "error", err)
			return errTrackNotLoaded
		}
		loaded = all[number]
	}

	a.mu.Lock()
	if a.watching != current {
		a.mu.Unlock()
		return errNotWatching
	}
	var saved project.Track
	if p := a.projectOf(current); p != nil && number != 0 && p.keys[number] != "" {
		saved = p.value.Tracks[p.keys[number]]
	}
	a.mu.Unlock()
	if err := current.session.SetLine(line, number, loaded, saved); err != nil {
		a.logger.Warn("the subtitle could not be switched", "line", lineName(line), "error", err)
		return errTrackNotLoaded
	}
	a.renameLines(current, video.Info)
	a.keepChoice(current, current.session.Tracks().Current, current.session.Mode())
	a.logger.Info("subtitle switched", "line", lineName(line), "track", trackOf(video.Info, number), "file", fileOf(video.Info, number))
	a.emit(EventPlayback, a.describe(current))
	return nil
}

// lineRefusal words why the session refuses a track on a line.
func lineRefusal(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, watch.ErrSameTrack):
		return errSameTrack
	case errors.Is(err, watch.ErrNeedsPrimary):
		return errNeedsPrimary
	}
	return errUnknownTrack
}

// renameLines names the lines by the tracks they show now, for the panel
// and the sync notice.
func (a *API) renameLines(current *watching, info media.Info) {
	now := current.session.Tracks().Current
	names := ptbr.NewLineNames(info, now.Primary, now.Translation)
	current.names.Store(&names)
	current.session.SetSyncWording(names.Notice)
}

// SwitchAudio changes the audio track while the video plays, when the
// player can.
func (a *API) SwitchAudio(number uint64) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	tracks := current.session.Tracks()
	if !slices.ContainsFunc(tracks.Of(media.TrackAudio), func(track media.Track) bool { return track.Number == number }) {
		return errUnknownTrack
	}
	if tracks.Current.Audio == number {
		return nil
	}
	err := current.session.SelectAudio(number)
	if errors.Is(err, session.ErrNotSupported) {
		return errNotSupported
	}
	if err != nil {
		a.logger.Warn("the audio track could not be switched", "track", number, "error", err)
		return errors.New(ptbr.T("error.noResponse"))
	}
	a.keepChoice(current, current.session.Tracks().Current, current.session.Mode())
	a.logger.Info("audio switched", "track", number)
	a.emit(EventPlayback, a.describe(current))
	return nil
}

// AddSubtitleFile adds a subtitle file the user picks, from any folder, to
// the open video, and puts it on a line: on the session screen it becomes
// the line's suggestion; while the video plays, the line switches to it.
// The project remembers the file, so it comes back the next time the video
// opens. The video comes back described with the file; an empty path means
// the user cancelled.
func (a *API) AddSubtitleFile(line string) (Video, error) {
	which, err := lineOf(line)
	if err != nil {
		return Video{}, err
	}
	ctx, err := a.context()
	if err != nil {
		return Video{}, err
	}
	a.mu.Lock()
	ui, video := a.ui, a.video
	a.mu.Unlock()
	if video == nil {
		return Video{}, errNoVideo
	}
	path, err := ui.OpenSubtitleDialog()
	if err != nil {
		a.logger.Warn("subtitle file dialog failed", "error", err)
		return Video{}, errSubtitleDialog
	}
	if path == "" {
		return Video{}, nil
	}

	a.adding.Lock()
	defer a.adding.Unlock()
	a.mu.Lock()
	video = a.video
	a.mu.Unlock()
	if video == nil {
		return Video{}, errNoVideo
	}
	added, track, err := a.deps.Service.AddSubtitleFile(ctx, *video, path)
	switch {
	case errors.Is(err, analysis.ErrTooManyFiles):
		return Video{}, errTooManyFiles
	case err != nil:
		a.logger.Warn("the subtitle file could not be added", "name", filepath.Base(path), "error", err)
		return Video{}, errSubtitleFile
	}
	a.mu.Lock()
	if a.video == nil || a.video.Path != video.Path {
		a.mu.Unlock()
		return Video{}, errNoVideo
	}
	a.video = &added
	p := a.project
	if p != nil && p.video == video.Path {
		a.rememberFile(p, &added, track)
	}
	a.mu.Unlock()
	a.logger.Info("subtitle file added", "name", filepath.Base(path), "track", track.Number)

	if current := a.current(); current != nil && current.session != nil && current.video == video.Path {
		current.session.AddTrack(track)
		if err := a.switchLine(current, which, track.Number); err != nil {
			return Video{}, err
		}
		return a.describeVideo(added, p, current.session.Tracks().Current, current.session.Mode()), nil
	}
	// The screen keeps what the user picked on the other lists; the line
	// added to takes the file.
	a.mu.Lock()
	mode := a.profile().Study.Mode
	a.mu.Unlock()
	suggested := added.Suggested
	if p != nil {
		suggested, mode = p.remembered(added.Info, suggested, mode)
	}
	if which == session.LineTranslation {
		suggested.Translation = track.Number
	} else {
		suggested.Primary = track.Number
	}
	return a.describeVideo(added, p, suggested, mode), nil
}

// rememberFile names a track added to the video in its project and, for a
// file picked from elsewhere, keeps its path so it comes back. Called with
// a.mu held.
func (a *API) rememberFile(p *videoProject, video *analysis.Analysis, track media.Track) {
	if a.deps.Projects == nil {
		return
	}
	p.keys[track.Number] = a.deps.Projects.TrackKey(video.Path, track)
	if !track.Added || p.readOnly {
		return
	}
	written := a.deps.Projects.AddedPath(video.Path, track.Path)
	present := func(file string) bool { return a.loadedFile(video.Info, video.Path, file) }
	if !p.value.RememberFile(written, present) {
		return
	}
	if err := a.saveProject(); err != nil {
		a.logger.Warn("the added subtitle file was not remembered", "name", filepath.Base(track.Path))
	}
}

// loadedFile tells whether a path the project wrote names a file the
// analysis has.
func (a *API) loadedFile(info media.Info, video, written string) bool {
	path := a.deps.Projects.ResolveAdded(video, written)
	return slices.ContainsFunc(info.Tracks, func(track media.Track) bool {
		return track.Added && filepath.Clean(track.Path) == filepath.Clean(path)
	})
}

// addRemembered reads again the subtitle files picked for the video before;
// a file that is gone, or no longer reads, is left out (the log names it).
func (a *API) addRemembered(result analysis.Analysis, p *videoProject) analysis.Analysis {
	if a.deps.Projects == nil || p == nil {
		return result
	}
	ctx, err := a.context()
	if err != nil {
		return result
	}
	for _, written := range p.value.AddedFiles {
		path := a.deps.Projects.ResolveAdded(result.Path, written)
		next, track, err := a.deps.Service.AddSubtitleFile(ctx, result, path)
		if err != nil {
			a.logger.Warn("an added subtitle file was left out", "name", filepath.Base(path), "error", err)
			continue
		}
		result = next
		p.keys[track.Number] = a.deps.Projects.TrackKey(result.Path, track)
	}
	return result
}

// fileOf names a subtitle file for the log, by its name only; empty for a
// track inside the video.
func fileOf(info media.Info, number uint64) string {
	if track, found := info.Track(number); found && track.External() {
		return filepath.Base(track.Path)
	}
	return ""
}
