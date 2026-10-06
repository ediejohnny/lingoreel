package app

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// errBusy refuses a video opened while another one plays.
var errBusy = worded(func() string { return ptbr.T("error.video.busy") })

// ChooseVideo asks for a video file; an empty path means the user cancelled.
func (a *API) ChooseVideo() (string, error) {
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenVideoDialog()
	if err != nil {
		a.logger.Warn("file dialog failed", "error", err)
		return "", errors.New(ptbr.T("error.video.dialog"))
	}
	return path, nil
}

// OpenVideo reads a video's tracks and the subtitle files next to it, and
// suggests what to watch it with.
func (a *API) OpenVideo(path string) (Video, error) {
	ctx, err := a.context()
	if err != nil {
		return Video{}, err
	}
	if a.current() != nil {
		return Video{}, errBusy
	}
	if !analysis.IsSupported(path) {
		return Video{}, errors.New(ptbr.T("error.video.unsupported", "extension", extensionOf(path)))
	}
	a.mu.Lock()
	languages := a.profile().Languages
	mode := a.profile().Study.Mode
	a.mu.Unlock()
	preferences := media.Preferences{
		Audio: media.LanguageTag(languages.Audio), Primary: media.LanguageTag(languages.Primary),
		Translation: media.LanguageTag(languages.Translation),
	}
	result, err := a.deps.Service.Analyze(ctx, path, preferences)
	if err != nil {
		a.logger.Warn("video analysis failed", "path", path, "error", err)
		if errors.Is(err, fs.ErrNotExist) {
			return Video{}, errors.New(ptbr.T("error.video.notFound", "name", filepath.Base(path)))
		}
		return Video{}, errors.New(ptbr.T("error.video.unreadable", "name", filepath.Base(path)))
	}
	a.logger.Info("video opened", "name", filepath.Base(path), "duration", result.Info.Duration.Round(time.Second),
		"audio", len(result.Info.TracksOf(media.TrackAudio)), "subtitles", len(result.Info.TracksOf(media.TrackSubtitle)))
	opened := a.loadProject(path, result.Info)
	result = a.addRemembered(result, opened)
	result = withKeptFits(result, opened, preferences)
	suggested, mode := opened.remembered(result.Info, result.Suggested, mode)
	a.mu.Lock()
	a.stopListening() // a video opened before
	a.video, a.project = &result, opened
	a.startListening(ctx, result, preferences)
	a.mu.Unlock()
	return a.describeVideo(result, opened, suggested, mode), nil
}

// describeVideo is a video as the session screen shows it. The project is
// read under a.mu: a session playing it saves passages and where it is.
func (a *API) describeVideo(result analysis.Analysis, opened *videoProject, suggested media.Selection, mode study.Mode) Video {
	preferences := a.preferences()
	video := Video{
		Path: result.Path, Name: filepath.Base(result.Path), Duration: result.Info.Duration.Seconds(),
		Suggested: Selection{Audio: suggested.Audio, Primary: suggested.Primary, Translation: suggested.Translation},
		Players:   a.players(),
		Audio:     audioTracks(result.Info), Subtitles: subtitleTracks(result.Info, preferences),
		Mode: string(mode), Modes: modeOptions(),
	}
	a.mu.Lock()
	video.SavedPassages, video.Resume = passagesOf(opened, result.Info.Duration), resumeOf(opened, result.Info.Duration)
	video.Listening = a.listeningTo(result.Path)
	if opened != nil {
		video.ProjectNote = opened.note
	}
	a.mu.Unlock()
	video.LanguageNote = languageNote(result.Info, suggested, preferences)
	return video
}

// preferences are the active profile's languages, as the track choice takes
// them.
func (a *API) preferences() media.Preferences {
	a.mu.Lock()
	languages := a.profile().Languages
	a.mu.Unlock()
	return media.Preferences{
		Audio: media.LanguageTag(languages.Audio), Primary: media.LanguageTag(languages.Primary),
		Translation: media.LanguageTag(languages.Translation),
	}
}

// audioTracks and subtitleTracks describe a video's tracks for the lists;
// the subtitles come in the app's order, each line's best choice first.
func audioTracks(info media.Info) []Track {
	list := []Track{}
	for _, track := range info.TracksOf(media.TrackAudio) {
		list = append(list, audioTrack(track))
	}
	return list
}

func subtitleTracks(info media.Info, preferences media.Preferences) []Track {
	list := []Track{}
	for _, track := range media.RankSubtitles(info.TracksOf(media.TrackSubtitle), preferences) {
		list = append(list, subtitleTrack(track))
	}
	return list
}

// languageNote tells when the video has subtitles but none in a language
// the profile asks for.
func languageNote(info media.Info, suggested media.Selection, preferences media.Preferences) string {
	readable := 0
	for _, track := range info.TracksOf(media.TrackSubtitle) {
		if track.Readable {
			readable++
		}
	}
	if readable == 0 {
		return ""
	}
	var missing []string
	if suggested.Primary == 0 && preferences.Primary != "" {
		missing = append(missing, ptbr.LanguageName(preferences.Primary))
	}
	if suggested.Translation == 0 && preferences.Translation != "" {
		missing = append(missing, ptbr.LanguageName(preferences.Translation))
	}
	switch len(missing) {
	case 0:
		return ""
	case 1:
		return ptbr.T("video.missingLanguage", "language", missing[0])
	}
	return ptbr.T("video.missingLanguages", "first", missing[0], "second", missing[1])
}

func extensionOf(path string) string {
	if extension := strings.ToLower(filepath.Ext(path)); extension != "" {
		return extension
	}
	return ptbr.T("video.noExtension")
}

func audioTrack(track media.Track) Track {
	return Track{Number: track.Number, Label: ptbr.AudioLabel(track), Language: string(track.Language), Readable: true}
}

func subtitleTrack(track media.Track) Track {
	result := Track{Number: track.Number, Label: ptbr.SubtitleLabel(track), Language: string(track.Language), Readable: track.Readable}
	result.Note, result.Warning = subtitleNote(track)
	return result
}

// subtitleNote explains what the app found about a subtitle track.
func subtitleNote(track media.Track) (string, bool) {
	if !track.Readable {
		return ptbr.T("track.note.unreadable"), true
	}
	switch track.Alignment.Verdict {
	case subtitle.VerdictMatch:
		if correction, moved := analysis.Correction(track); moved {
			return ptbr.T("track.note.adjusted", "offset", ptbr.Offset(subtitle.Offset(correction.Offset))), false
		}
		if track.Alignment.Heard {
			return ptbr.T("track.note.heard"), false
		}
		return ptbr.T("track.note.match"), false
	case subtitle.VerdictOtherCut:
		return ptbr.T("track.note.otherCut"), true
	case subtitle.VerdictNoMatch:
		return ptbr.T("track.note.noMatch"), true
	}
	if track.Candidate {
		return ptbr.T("track.note.candidate"), true
	}
	return "", false
}
