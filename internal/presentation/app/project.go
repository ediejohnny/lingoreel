package app

import (
	"errors"
	"strings"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/project"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
)

// videoProject is the open video's project and what keeping it needs.
type videoProject struct {
	video string
	value project.Project
	// keys name the video's tracks by number, as the project knows them.
	keys map[uint64]string
	// readOnly keeps a file the app could not read whole (written by a
	// newer version, or damaged) from being overwritten; note tells why.
	readOnly bool
	note     string
	// offsetsChanged tells offsets moved since the last save; they are
	// saved when the session ends, not at every step of a held shortcut.
	offsetsChanged bool
}

var (
	errNoPassageToSave = errors.New("Comece a estudar um trecho antes de salvá-lo.")
	errPassageName     = errors.New("Dê um nome ao trecho, com até 80 letras.")
	errTooManySaved    = errors.New("Já há trechos salvos demais neste vídeo. Exclua algum antes.")
	errUnknownPassage  = errors.New("Trecho salvo não encontrado.")
	errNotKept         = errors.New("Não foi possível gravar no arquivo de correções deste vídeo.")
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
		p.note = "As correções deste vídeo foram gravadas por uma versão mais nova do LingoReel. Elas não serão alteradas."
	case err != nil:
		a.logger.Warn("the video's project could not be read", "video", video, "error", err)
		p.readOnly = true
		p.note = "O arquivo de correções deste vídeo não pôde ser lido. O que você mudar agora não será gravado."
	default:
		p.value = loaded
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
// Called with a.mu held.
func (a *API) saveProject() error {
	p := a.project
	if p == nil || p.readOnly || a.deps.Projects == nil {
		return nil
	}
	if err := a.deps.Projects.Save(p.video, p.value); err != nil {
		a.logger.Warn("the video's project was not saved", "video", p.video, "error", err)
		return errNotKept
	}
	p.offsetsChanged = false
	return nil
}

// projectOf is the open project when it belongs to the session's video.
// Called with a.mu held.
func (a *API) projectOf(current *watching) *videoProject {
	if a.project == nil || current == nil || a.project.video != current.video {
		return nil
	}
	return a.project
}

// keepEdits takes a line's edits into the project and saves it.
func (a *API) keepEdits(current *watching, line session.Line, edits map[string]project.Edit) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil || current.keys[line] == "" {
		return
	}
	track := p.track(current.keys[line])
	track.Edits = edits
	p.value.Tracks[current.keys[line]] = track
	if err := a.saveProject(); err != nil {
		current.problem.Store(err.Error())
	}
}

// keepOffset takes a line's offset into the project, saved when the
// session ends.
func (a *API) keepOffset(current *watching, line session.Line, offset subtitle.Offset) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil || current.keys[line] == "" {
		return
	}
	track := p.track(current.keys[line])
	track.Offset = project.MillisOf(offset.Duration())
	p.value.Tracks[current.keys[line]] = track
	p.offsetsChanged = true
}

// keepChoice remembers what the user chose to watch with, and saves.
func (a *API) keepChoice(current *watching, tracks media.Selection, mode study.Mode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.projectOf(current)
	if p == nil {
		return
	}
	p.value.Choice = project.Choice{
		Audio: p.keys[tracks.Audio], Primary: p.keys[tracks.Primary], Translation: p.keys[tracks.Translation], Mode: mode,
	}
	if err := a.saveProject(); err != nil {
		current.problem.Store(err.Error())
	}
}

// saveOffsets writes the offsets moved during a session, as it ends.
func (a *API) saveOffsets(current *watching) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.projectOf(current); p != nil && p.offsetsChanged {
		_ = a.saveProject() // logged; the session is over
	}
}

// SavePassage keeps the passage being studied under a name; a passage of
// the same name is replaced.
func (a *API) SavePassage(name string) error {
	current := a.current()
	if current == nil || current.session == nil {
		return errNotWatching
	}
	status := current.session.Passage()
	if !status.Active {
		return errNoPassageToSave
	}
	passage := project.Passage{
		Name: strings.TrimSpace(name), Start: project.MillisOf(status.Segment.Start), End: project.MillisOf(status.Segment.End),
	}
	if passage.Validate() != nil {
		return errPassageName
	}
	a.mu.Lock()
	p := a.projectOf(current)
	if p == nil {
		a.mu.Unlock()
		return errNotWatching
	}
	passages := p.value.Passages
	if i := indexOfPassage(passages, passage.Name); i >= 0 {
		passages[i] = passage
	} else if len(passages) >= project.MaxPassages {
		a.mu.Unlock()
		return errTooManySaved
	} else {
		p.value.Passages = append(passages, passage)
	}
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
	if len(p.value.Passages) >= project.MaxPassages {
		a.mu.Unlock()
		current.session.Notify(errTooManySaved.Error())
		return
	}
	practice := project.PlanOf(plan)
	passage := project.Passage{
		Name:  project.UniqueName(p.value.Passages, ptbr.PassageName(segment.Start, segment.End)),
		Start: project.MillisOf(segment.Start), End: project.MillisOf(segment.End), Plan: &practice,
	}
	p.value.Passages = append(p.value.Passages, passage)
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

// DeletePassage forgets a saved passage.
func (a *API) DeletePassage(name string) error {
	current := a.current()
	a.mu.Lock()
	p := a.projectOf(current)
	if p == nil {
		a.mu.Unlock()
		return errNotWatching
	}
	i := indexOfPassage(p.value.Passages, name)
	if i < 0 {
		a.mu.Unlock()
		return errUnknownPassage
	}
	p.value.Passages = append(p.value.Passages[:i:i], p.value.Passages[i+1:]...)
	err := a.saveProject()
	a.mu.Unlock()
	a.emit(EventPlayback, a.describe(current))
	return err
}

func indexOfPassage(passages []project.Passage, name string) int {
	for i, passage := range passages {
		if strings.EqualFold(passage.Name, strings.TrimSpace(name)) {
			return i
		}
	}
	return -1
}

// savedPassages lists the open video's passages for the panel.
func (a *API) savedPassages(current *watching) ([]SavedPassage, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	list := []SavedPassage{}
	p := a.projectOf(current)
	if p == nil {
		return list, ""
	}
	for _, passage := range p.value.Passages {
		list = append(list, SavedPassage{
			Name: passage.Name, Start: ptbr.PassageClock(passage.Start.Duration()), End: ptbr.PassageClock(passage.End.Duration()),
		})
	}
	return list, p.note
}
