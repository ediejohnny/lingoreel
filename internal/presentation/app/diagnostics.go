package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/presentation/ptbr"
)

// Diagnostics is the state of the app for a problem report (RNF-032). It
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
	add("LingoReel", cmpOr(a.deps.About.Version, "desconhecida"))
	add("Windows", cmpOr(a.deps.About.Windows, "desconhecido"))
	for i, monitor := range a.monitors() {
		add(fmt.Sprintf("Monitor %d", i+1), monitor)
	}
	if current := a.current(); current != nil && current.session != nil {
		a.describeSession(current, add)
	} else {
		add("Sessão", "nenhum vídeo tocando")
	}
	add("Pasta dos registros", cmpOr(a.deps.LogDir, "sem registros"))
	add("Pasta das configurações", cmpOr(a.deps.SettingsDir, "desconhecida"))

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
	add("Vídeo", filepath.Base(current.video))
	add("Player", fmt.Sprintf("%s (%s)", current.player.ID, current.player.Path))
	add("Conexão com o player", connectionText(s.Health()))
	add("Estado", fmt.Sprintf("%s a %s, posição %s de %s", statusText(state.Status),
		speedText(float64(state.Speed)), ptbr.EditClock(state.Position), ptbr.Clock(state.Duration)))
	add("Quadros por segundo", fpsText(current.frameRate))
	shown := s.Shown()
	add(current.names[session.LinePrimary], fmt.Sprintf("offset %s, fala %s", ptbr.Offset(s.Offset(session.LinePrimary)), eventIDs(shown.Primary)))
	add(current.names[session.LineTranslation], fmt.Sprintf("offset %s, fala %s", ptbr.Offset(s.Offset(session.LineTranslation)), eventIDs(shown.Translation)))
	overlay := "ativa"
	if !s.HasDisplay() {
		overlay = "inativa"
	}
	add("Camada das legendas", overlay)
}

// monitors words each screen: "1920×1080 em (0, 0), escala 100% (principal)".
func (a *API) monitors() []string {
	if a.deps.Monitors == nil {
		return nil
	}
	found, err := a.deps.Monitors()
	if err != nil {
		a.logger.Warn("monitors could not be listed", "error", err)
		return []string{"não foi possível listar"}
	}
	words := make([]string, len(found))
	for i, m := range found {
		words[i] = fmt.Sprintf("%d×%d em (%d, %d), escala %d%%", m.Width, m.Height, m.X, m.Y, m.DPI*100/layout.BaseDPI)
		if m.Primary {
			words[i] += " (principal)"
		}
	}
	return words
}

func connectionText(health watch.Health) string {
	switch health {
	case watch.HealthReconnecting:
		return "reconectando"
	case watch.HealthStalled:
		return "conectado, mas a posição não anda (travado?)"
	}
	return "conectado"
}

func statusText(status playback.Status) string {
	switch status {
	case playback.StatusPlaying:
		return "tocando"
	case playback.StatusPaused:
		return "pausado"
	case playback.StatusStopped:
		return "parado"
	}
	return "desconhecido"
}

// speedText shows a speed as "1,00×".
func speedText(speed float64) string {
	return strings.Replace(strconv.FormatFloat(speed, 'f', 2, 64), ".", ",", 1) + "×"
}

// eventIDs names the lines on screen by their IDs ("#483"), never by text.
func eventIDs(events []subtitle.Event) string {
	if len(events) == 0 {
		return "nenhuma na tela"
	}
	ids := make([]string, len(events))
	for i, event := range events {
		ids[i] = "#" + event.ID
	}
	return strings.Join(ids, ", ")
}

func fpsText(rate float64) string {
	if rate <= 0 {
		return "desconhecido"
	}
	return strings.Replace(strconv.FormatFloat(rate, 'f', 3, 64), ".", ",", 1)
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
