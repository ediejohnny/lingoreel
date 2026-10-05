package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// Diagnostics is the state of the app for a problem report. It
// never holds the text of a line and never leaves the computer by itself:
// the user copies it.
type Diagnostics struct {
	Items []Fact `json:"items"`
	// Text is the same, one "label: value" per line, to copy.
	Text string `json:"text"`
}

// Fact is one line of the diagnostics.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Diagnostics describes the app, the computer and the session, if any.
func (a *API) Diagnostics() Diagnostics {
	var facts []Fact
	add := func(label, value string) { facts = append(facts, Fact{label, value}) }
	add("LingoReel", cmpOr(a.deps.About.Version, ptbr.T("diagnostics.unknownVersion")))
	add("Windows", cmpOr(a.deps.About.Windows, ptbr.T("diagnostics.unknownWindows")))
	for i, monitor := range a.monitors() {
		add(ptbr.T("diagnostics.monitor", "number", i+1), monitor)
	}
	if current := a.current(); current != nil && current.session != nil {
		a.describeSession(current, add)
	} else {
		add(ptbr.T("diagnostics.session"), ptbr.T("diagnostics.noSession"))
	}
	add(ptbr.T("diagnostics.logDir"), cmpOr(a.deps.LogDir, ptbr.T("diagnostics.noLogs")))
	add(ptbr.T("diagnostics.settingsDir"), cmpOr(a.deps.SettingsDir, ptbr.T("diagnostics.unknownFolder")))

	lines := make([]string, len(facts))
	for i, fact := range facts {
		lines[i] = fact.Label + ": " + fact.Value
	}
	return Diagnostics{Items: facts, Text: strings.Join(lines, "\n")}
}

func (a *API) describeSession(current *watching, add func(label, value string)) {
	s := current.session
	state := s.State()
	if state.Duration <= 0 {
		state.Duration = current.duration // the player has not told it yet
	}
	add(ptbr.T("diagnostics.video"), filepath.Base(current.video))
	add(ptbr.T("diagnostics.player"), fmt.Sprintf("%s (%s)", current.player.ID, current.player.Path))
	add(ptbr.T("diagnostics.connection"), connectionText(s.Health()))
	add(ptbr.T("diagnostics.state"), ptbr.T("diagnostics.stateValue", "status", statusText(state.Status),
		"speed", ptbr.Speed(state.Speed), "position", ptbr.EditClock(state.Position), "duration", ptbr.Clock(state.Duration)))
	add(ptbr.T("diagnostics.frameRate"), fpsText(current.frameRate))
	shown := s.Shown()
	add(current.lineNames()[session.LinePrimary], ptbr.T("diagnostics.line",
		"offset", ptbr.Offset(s.Offset(session.LinePrimary)), "lines", eventIDs(shown.Primary)))
	add(current.lineNames()[session.LineTranslation], ptbr.T("diagnostics.line",
		"offset", ptbr.Offset(s.Offset(session.LineTranslation)), "lines", eventIDs(shown.Translation)))
	overlay := ptbr.T("diagnostics.overlayOn")
	if !s.HasDisplay() {
		overlay = ptbr.T("diagnostics.overlayOff")
	}
	add(ptbr.T("diagnostics.overlay"), overlay)
}

// monitors words each screen: "1920×1080 em (0, 0), escala 100% (principal)".
func (a *API) monitors() []string {
	if a.deps.Monitors == nil {
		return nil
	}
	found, err := a.deps.Monitors()
	if err != nil {
		a.logger.Warn("monitors could not be listed", "error", err)
		return []string{ptbr.T("diagnostics.monitorsFailed")}
	}
	words := make([]string, len(found))
	for i, m := range found {
		words[i] = ptbr.T("diagnostics.monitorValue", "width", m.Width, "height", m.Height,
			"x", m.X, "y", m.Y, "scale", m.DPI*100/layout.BaseDPI)
		if m.Primary {
			words[i] = ptbr.T("diagnostics.primaryMonitor", "monitor", words[i])
		}
	}
	return words
}

func connectionText(health watch.Health) string {
	switch health {
	case watch.HealthReconnecting:
		return ptbr.T("diagnostics.reconnecting")
	case watch.HealthStalled:
		return ptbr.T("diagnostics.stalled")
	}
	return ptbr.T("diagnostics.connected")
}

func statusText(status playback.Status) string {
	switch status {
	case playback.StatusPlaying:
		return ptbr.T("diagnostics.playing")
	case playback.StatusPaused:
		return ptbr.T("diagnostics.paused")
	case playback.StatusStopped:
		return ptbr.T("diagnostics.stopped")
	}
	return ptbr.T("diagnostics.unknownStatus")
}

// eventIDs names the lines on screen by their IDs ("#483"), never by text.
func eventIDs(events []subtitle.Event) string {
	if len(events) == 0 {
		return ptbr.T("diagnostics.noLines")
	}
	ids := make([]string, len(events))
	for i, event := range events {
		ids[i] = "#" + event.ID
	}
	return strings.Join(ids, ", ")
}

func fpsText(rate float64) string {
	if rate <= 0 {
		return ptbr.T("diagnostics.unknownFrameRate")
	}
	return ptbr.Number(rate, 3)
}

func frameRateOf(info media.Info) float64 {
	if videos := info.TracksOf(media.TrackVideo); len(videos) > 0 {
		return videos[0].FrameRate
	}
	return 0
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
