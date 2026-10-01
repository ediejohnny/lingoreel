package ptbr_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/presentation/ptbr"
)

func TestSpeedAndTime(t *testing.T) {
	for speed, want := range map[playback.Speed]string{1: "1,00×", 0.75: "0,75×", 2: "2,00×", 1.25: "1,25×"} {
		if got := ptbr.Speed(speed); got != want {
			t.Errorf("Speed(%v) = %q, want %q", speed, got, want)
		}
	}
	if got := ptbr.BarTime(65*time.Second, 90*time.Minute); got != "1:05 / 1:30:00" {
		t.Errorf("BarTime = %q", got)
	}
	if got := ptbr.BarTime(5*time.Second, 0); got != "0:05" {
		t.Errorf("BarTime without a duration = %q", got)
	}
}

func TestBarItemWritesTheSpeeds(t *testing.T) {
	cases := []struct {
		item controlbar.Item
		want string
	}{
		{controlbar.Item{Action: controlbar.ActionSpeed, Speed: 0.75}, "0,75×"},
		{controlbar.Item{Action: controlbar.ActionSpeedChoice, Speed: 1.5}, "1,50×"},
		// A speed the player has not reported yet shows as normal.
		{controlbar.Item{Action: controlbar.ActionSpeed}, "1,00×"},
		{controlbar.Item{Action: controlbar.ActionSpeed, Speed: playback.Speed(math.NaN())}, "1,00×"},
		{controlbar.Item{Action: controlbar.ActionPlayPause}, ""},
	}
	for _, c := range cases {
		if got := ptbr.BarItem(c.item); got != c.want {
			t.Errorf("BarItem(%+v) = %q, want %q", c.item, got, c.want)
		}
	}
}

func TestBarTips(t *testing.T) {
	on := func(action controlbar.Action) controlbar.Item { return controlbar.Item{Action: action, Enabled: true} }
	state := controlbar.State{Volume: 0.8, HasTranslation: true, TranslationVisible: true}
	cases := []struct {
		item  controlbar.Item
		state func(*controlbar.State)
		want  string
	}{
		{on(controlbar.ActionPlayPause), nil, "Pausar (Espaço)"},
		{on(controlbar.ActionPlayPause), func(s *controlbar.State) { s.Paused = true }, "Reproduzir (Espaço)"},
		{on(controlbar.ActionBack), nil, "Voltar 5 s (←)"},
		{on(controlbar.ActionForward), nil, "Avançar 5 s (→)"},
		{on(controlbar.ActionMute), nil, "Sem som (M)"},
		{on(controlbar.ActionMute), func(s *controlbar.State) { s.Muted = true }, "Ativar o som (M)"},
		{on(controlbar.ActionVolume), nil, "Volume: 80% (↑ ↓)"},
		{on(controlbar.ActionVolume), func(s *controlbar.State) { s.Muted = true }, "Volume: 0% (↑ ↓)"},
		{on(controlbar.ActionTranslation), nil, "Ocultar a tradução"},
		{on(controlbar.ActionTranslation), func(s *controlbar.State) { s.TranslationVisible = false }, "Mostrar a tradução"},
		{controlbar.Item{Action: controlbar.ActionTranslation}, func(s *controlbar.State) { s.HasTranslation = false }, "Nenhuma tradução escolhida"},
		{on(controlbar.ActionFullscreen), nil, "Tela cheia (F)"},
		{on(controlbar.ActionFullscreen), func(s *controlbar.State) { s.Fullscreen = true }, "Sair da tela cheia (Esc)"},
		{on(controlbar.ActionSpeed), nil, "Velocidade"},
		{controlbar.Item{Action: controlbar.ActionSpeed}, nil, "Velocidade (indisponível neste player)"},
		{on(controlbar.ActionNone), nil, ""},
	}
	for _, c := range cases {
		s := state
		if c.state != nil {
			c.state(&s)
		}
		if got := ptbr.BarTip(c.item, s); got != c.want {
			t.Errorf("BarTip(%d) = %q, want %q", c.item.Action, got, c.want)
		}
	}
}

func TestWindowTitle(t *testing.T) {
	if got := ptbr.WindowTitle("Filme.mkv"); got != "Filme.mkv — LingoReel" {
		t.Errorf("WindowTitle = %q", got)
	}
	if got := ptbr.WindowTitle("  "); got != "LingoReel" || strings.Contains(got, "—") {
		t.Errorf("WindowTitle of nothing = %q", got)
	}
}
