package ptbr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
)

// unavailable ends the tip of a control the player lacks.
const unavailable = " (indisponível neste player)"

// Speed shows a speed as "1,25×".
func Speed(speed playback.Speed) string {
	return strings.Replace(fmt.Sprintf("%.2f×", float64(speed)), ".", ",", 1)
}

// WindowTitle is the player window's title for a video file name.
func WindowTitle(video string) string {
	if strings.TrimSpace(video) == "" {
		return "LingoReel"
	}
	return video + " — LingoReel"
}

// BarTime shows the time on the control bar: "12:34 / 1:29:32".
func BarTime(position, duration time.Duration) string {
	if duration <= 0 {
		return Clock(position)
	}
	return Clock(position) + " / " + Clock(duration)
}

// BarItem is the text written on a control: the speeds, in the list and on
// the speed button. A speed not reported yet shows as normal.
func BarItem(item controlbar.Item) string {
	switch item.Action {
	case controlbar.ActionSpeed, controlbar.ActionSpeedChoice:
		if !item.Speed.Valid() {
			return Speed(playback.NormalSpeed)
		}
		return Speed(item.Speed)
	}
	return ""
}

// BarTip is a control's tip, with its key when it has one.
func BarTip(item controlbar.Item, state controlbar.State) string {
	tip := barTip(item, state)
	if tip != "" && !item.Enabled && item.Action != controlbar.ActionTranslation {
		tip += unavailable
	}
	return tip
}

func barTip(item controlbar.Item, state controlbar.State) string {
	switch item.Action {
	case controlbar.ActionPlayPause:
		if state.Paused {
			return "Reproduzir (Espaço)"
		}
		return "Pausar (Espaço)"
	case controlbar.ActionBack:
		return fmt.Sprintf("Voltar %d s (←)", int(controlbar.JumpStep/time.Second))
	case controlbar.ActionForward:
		return fmt.Sprintf("Avançar %d s (→)", int(controlbar.JumpStep/time.Second))
	case controlbar.ActionMute:
		if state.Muted {
			return "Ativar o som (M)"
		}
		return "Sem som (M)"
	case controlbar.ActionVolume:
		level := state.Volume.Clamp()
		if state.Muted {
			level = 0
		}
		return fmt.Sprintf("Volume: %d%% (↑ ↓)", int(math.Round(float64(level)*100)))
	case controlbar.ActionTrack:
		return "Clique ou arraste para ir a outro ponto"
	case controlbar.ActionTranslation:
		switch {
		case !state.HasTranslation:
			return "Nenhuma tradução escolhida"
		case state.TranslationVisible:
			return "Ocultar a tradução"
		}
		return "Mostrar a tradução"
	case controlbar.ActionSpeed:
		return "Velocidade"
	case controlbar.ActionFullscreen:
		if state.Fullscreen {
			return "Sair da tela cheia (Esc)"
		}
		return "Tela cheia (F)"
	}
	return ""
}
