package app

import (
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// The video inside the app's window: the screens tell where their controls
// leave room for it, ask for the fullscreen, and keep the panel beside it.

// Panel sections the app asks the screens to open.
const (
	PanelEditor   = "editor"
	PanelSettings = "settings"
)

var (
	errBadVideoLayout = worded(func() string { return ptbr.T("error.videoLayout") })
	errBadPanel       = worded(func() string { return ptbr.T("error.panelWidth") })
)

// VideoLayout is the room the screen's controls leave the video, in pixels
// at 100% scaling: the screen shows the video (Visible), and its header,
// panel and other controls take Top, Right, Bottom and Left.
type VideoLayout struct {
	Visible bool    `json:"visible"`
	Top     float64 `json:"top"`
	Right   float64 `json:"right"`
	Bottom  float64 `json:"bottom"`
	Left    float64 `json:"left"`
}

// PanelRequest asks the screens to open the panel beside the video on a
// section; for the settings, Page names the page (empty for the first).
type PanelRequest struct {
	Section string `json:"section"`
	Page    string `json:"page,omitempty"`
}

// PanelState is how the panel beside the video was left: open or closed,
// and its width (zero for the default).
type PanelState struct {
	Open  bool    `json:"open"`
	Width float64 `json:"width"`
}

// SetVideoLayout places the video inside the app's window. Without a video
// drawn there it only keeps the place for the next one.
func (a *API) SetVideoLayout(place VideoLayout) error {
	insets := layout.Insets{Top: place.Top, Right: place.Right, Bottom: place.Bottom, Left: place.Left}
	if !insets.Valid() {
		return errBadVideoLayout
	}
	if a.deps.Stage != nil {
		a.deps.Stage.Place(place.Visible, insets)
	}
	return nil
}

// SetFullscreen puts the video in fullscreen, or back.
func (a *API) SetFullscreen(on bool) {
	if a.deps.Stage != nil {
		a.deps.Stage.SetFullscreen(on)
	}
}

// SetPanel keeps how the panel beside the video is: open or closed, and its
// width in pixels at 100% (zero for the default).
func (a *API) SetPanel(state PanelState) error {
	panel := settings.PanelBounds{Open: state.Open, Width: state.Width}
	if !panel.Valid() {
		return errBadPanel
	}
	a.mu.Lock()
	same := a.config.Panel == panel
	a.mu.Unlock()
	if same {
		return nil
	}
	return a.changeConfig(func(next *settings.Config) error {
		next.Panel = panel
		return nil
	})
}
