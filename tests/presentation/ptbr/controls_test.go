package ptbr_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
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
		{on(controlbar.ActionTranslation), nil, "Ocultar a tradução (Alt direito)"},
		{on(controlbar.ActionTranslation), func(s *controlbar.State) { s.TranslationVisible = false }, "Mostrar a tradução (Alt direito)"},
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
		if got := ptbr.BarTip(c.item, s, nil, nil); got != c.want {
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

func TestBarTipsShowTheProfileKeys(t *testing.T) {
	keys := shortcut.Map{
		shortcut.PlayPause: {Modifiers: shortcut.Ctrl, Key: "KeyP"},
		shortcut.VolumeUp:  {Key: "NumpadAdd"}, shortcut.VolumeDown: {Key: "NumpadSubtract"},
		shortcut.Mute: {Key: "Semicolon"},
	}
	play := controlbar.Item{Action: controlbar.ActionPlayPause, Enabled: true}
	if got := ptbr.BarTip(play, controlbar.State{}, keys, nil); got != "Pausar (Ctrl+P)" {
		t.Errorf("tip = %q", got)
	}
	volume := controlbar.Item{Action: controlbar.ActionVolume, Enabled: true}
	if got := ptbr.BarTip(volume, controlbar.State{Volume: 0.5}, keys, nil); got != "Volume: 50% (+ (teclado numérico) - (teclado numérico))" {
		t.Errorf("tip = %q", got)
	}
	// The key right of L types "ç" on ABNT2: the layout's character wins.
	abnt2 := func(key shortcut.Key) string {
		if key == "Semicolon" {
			return "ç"
		}
		return ""
	}
	mute := controlbar.Item{Action: controlbar.ActionMute, Enabled: true}
	if got := ptbr.BarTips(abnt2)(mute, controlbar.State{}, keys); got != "Sem som (Ç)" {
		t.Errorf("tip = %q", got)
	}
	if got := ptbr.BarTip(mute, controlbar.State{}, keys, nil); got != "Sem som (;)" {
		t.Errorf("tip without the layout = %q", got)
	}
}

func TestBarTipsWithoutAKey(t *testing.T) {
	// Commands without a shortcut leave the parentheses out.
	keys := shortcut.Map{shortcut.PlayPause: {}, shortcut.VolumeUp: {}, shortcut.Translation: {}}
	play := controlbar.Item{Action: controlbar.ActionPlayPause, Enabled: true}
	if got := ptbr.BarTip(play, controlbar.State{}, keys, nil); got != "Pausar" {
		t.Errorf("tip = %q", got)
	}
	volume := controlbar.Item{Action: controlbar.ActionVolume, Enabled: true}
	if got := ptbr.BarTip(volume, controlbar.State{Volume: 0.5}, keys, nil); got != "Volume: 50% (↓)" {
		t.Errorf("tip with one volume key = %q", got)
	}
	keys[shortcut.VolumeDown] = shortcut.Chord{}
	if got := ptbr.BarTip(volume, controlbar.State{Volume: 0.5}, keys, nil); got != "Volume: 50%" {
		t.Errorf("tip without volume keys = %q", got)
	}
	translation := controlbar.Item{Action: controlbar.ActionTranslation, Enabled: true}
	if got := ptbr.BarTip(translation, controlbar.State{HasTranslation: true}, keys, nil); got != "Mostrar a tradução" {
		t.Errorf("tip = %q", got)
	}
	// A mode that always shows the translation explains the off button,
	// without blaming the player.
	locked := controlbar.Item{Action: controlbar.ActionTranslation}
	state := controlbar.State{HasTranslation: true, TranslationVisible: true, TranslationLocked: true}
	if got := ptbr.BarTip(locked, state, nil, nil); got != "A tradução fica sempre visível neste modo de estudo" {
		t.Errorf("locked tip = %q", got)
	}
}
