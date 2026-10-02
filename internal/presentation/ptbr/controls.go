package ptbr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
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

// BarTips are the player window's tips, with keys named by char.
func BarTips(char KeyChar) func(controlbar.Item, controlbar.State, shortcut.Map) string {
	return func(item controlbar.Item, state controlbar.State, keys shortcut.Map) string {
		return BarTip(item, state, keys, char)
	}
}

// BarTip is a control's tip, with its key when it has one: the key the
// profile gives the command.
func BarTip(item controlbar.Item, state controlbar.State, keys shortcut.Map, char KeyChar) string {
	key := func(command shortcut.Command) string { return ShortcutLabel(keys.Of(command), char) }
	tip := barTip(item, state, key)
	if tip != "" && !item.Enabled && item.Action != controlbar.ActionTranslation {
		tip += unavailable
	}
	return tip
}

func barTip(item controlbar.Item, state controlbar.State, key func(shortcut.Command) string) string {
	with := func(text string, command shortcut.Command) string { return text + " (" + key(command) + ")" }
	jump := int(controlbar.JumpStep / time.Second)
	switch item.Action {
	case controlbar.ActionPlayPause:
		if state.Paused {
			return with("Reproduzir", shortcut.PlayPause)
		}
		return with("Pausar", shortcut.PlayPause)
	case controlbar.ActionBack:
		return with(fmt.Sprintf("Voltar %d s", jump), shortcut.SeekBack)
	case controlbar.ActionForward:
		return with(fmt.Sprintf("Avançar %d s", jump), shortcut.SeekForward)
	case controlbar.ActionMute:
		if state.Muted {
			return with("Ativar o som", shortcut.Mute)
		}
		return with("Sem som", shortcut.Mute)
	case controlbar.ActionVolume:
		level := state.Volume.Clamp()
		if state.Muted {
			level = 0
		}
		return fmt.Sprintf("Volume: %d%% (%s %s)", int(math.Round(float64(level)*100)), key(shortcut.VolumeUp), key(shortcut.VolumeDown))
	case controlbar.ActionTrack:
		return "Clique ou arraste para ir a outro ponto"
	case controlbar.ActionTranslation:
		switch {
		case !state.HasTranslation:
			return "Nenhuma tradução escolhida"
		case state.TranslationVisible:
			return with("Ocultar a tradução", shortcut.Translation)
		}
		return with("Mostrar a tradução", shortcut.Translation)
	case controlbar.ActionSpeed:
		return "Velocidade"
	case controlbar.ActionFullscreen:
		if state.Fullscreen {
			return with("Sair da tela cheia", shortcut.ExitFullscreen)
		}
		return with("Tela cheia", shortcut.Fullscreen)
	}
	return ""
}
