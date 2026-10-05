package app

import (
	"slices"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// EventVideo sends the video described again, with a subtitle file the
// menu over the video added to it, for the session screen to list.
const EventVideo = "video:changed"

// settingsPages are the settings pages the menu opens; any other opens the
// first one.
var settingsPages = []string{"general", "subtitles-primary", "subtitles-translation", "study", "shortcuts"}

// The menu the right button opens over the video asks these of the app (the
// player window's AppActions). Each runs off the window's thread and tells
// on the video what went wrong.

// OpenSettings opens the panel beside the video on a page of the settings,
// out of the fullscreen first, where the panel does not show.
func (l Lifecycle) OpenSettings(page string) {
	a := l.api
	if !slices.Contains(settingsPages, page) {
		page = settingsPages[0]
	}
	if a.deps.Stage != nil {
		a.deps.Stage.SetFullscreen(false)
	}
	a.emit(EventPanel, PanelRequest{Section: PanelSettings, Page: page})
}

// SwitchSubtitle puts another subtitle on a line.
func (l Lifecycle) SwitchSubtitle(line session.Line, track uint64) {
	l.api.tell(l.api.SwitchSubtitle(lineName(line), track))
}

// SwitchAudio changes the audio track.
func (l Lifecycle) SwitchAudio(track uint64) {
	l.api.tell(l.api.SwitchAudio(track))
}

// AddSubtitle asks for a subtitle file and puts it on a line; the screens
// hear of the file, so the session screen lists it once the video ends.
func (l Lifecycle) AddSubtitle(line session.Line) {
	video, err := l.api.AddSubtitleFile(lineName(line))
	if err != nil {
		l.api.tell(err)
		return
	}
	if video.Path != "" {
		l.api.emit(EventVideo, video)
	}
}

// ExportSubtitle saves a line as a subtitle file, asking where.
func (l Lifecycle) ExportSubtitle(line session.Line) {
	l.api.tell(l.api.ExportSubtitle(lineName(line)))
}

// CopyText puts a line's text on the clipboard, and says so on the video.
func (l Lifecycle) CopyText(text string) {
	a := l.api
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return
	}
	if err := ui.CopyText(text); err != nil {
		a.logger.Warn("the line was not copied", "error", err)
		a.tell(errNotCopied)
		return
	}
	if current := a.current(); current != nil && current.session != nil {
		current.session.Notify(ptbr.T("notice.copied"))
	}
}

// OpenVideo asks for another video and, unless the user cancels, ends this
// one and opens that, as if it had been dropped on the window.
func (l Lifecycle) OpenVideo() {
	a := l.api
	path, err := a.ChooseVideo()
	if err != nil {
		a.tell(err)
		return
	}
	if path == "" {
		return
	}
	a.stop(true)
	a.emit(EventVideoDropped, path)
}

// StopVideo ends the video, as the panel's "Encerrar" does.
func (l Lifecycle) StopVideo() {
	_ = l.api.StopWatching() // no video: nothing to end
}

var errNotCopied = worded(func() string { return ptbr.T("error.copy") })

// tell shows on the video what went wrong with something the menu asked.
func (a *API) tell(err error) {
	if err == nil {
		return
	}
	if current := a.current(); current != nil && current.session != nil {
		current.session.Notify(err.Error())
	}
}
