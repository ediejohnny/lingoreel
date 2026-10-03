package ptbr

import (
	"fmt"
	"math"
	"strings"
	"time"

	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
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
	with := func(text string, command shortcut.Command) string {
		if label := key(command); label != "" {
			return text + " (" + label + ")"
		}
		return text
	}
	// The jump buttons run the 5 s seek commands.
	back, _ := shortcut.SeekBack.Jump()
	forward, _ := shortcut.SeekForward.Jump()
	switch item.Action {
	case controlbar.ActionPlayPause:
		if state.Paused {
			return with("Reproduzir", shortcut.PlayPause)
		}
		return with("Pausar", shortcut.PlayPause)
	case controlbar.ActionBack:
		return with(fmt.Sprintf("Voltar %d s", int(-back/time.Second)), shortcut.SeekBack)
	case controlbar.ActionForward:
		return with(fmt.Sprintf("Avançar %d s", int(forward/time.Second)), shortcut.SeekForward)
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
		volume := fmt.Sprintf("Volume: %d%%", int(math.Round(float64(level)*100)))
		if keys := strings.TrimSpace(key(shortcut.VolumeUp) + " " + key(shortcut.VolumeDown)); keys != "" {
			volume += " (" + keys + ")"
		}
		return volume
	case controlbar.ActionTrack:
		return "Clique ou arraste para ir a outro ponto"
	case controlbar.ActionTranslation:
		switch {
		case !state.HasTranslation:
			return "Nenhuma tradução escolhida"
		case state.TranslationLocked:
			return "A tradução fica sempre visível neste modo de estudo"
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
	case controlbar.ActionFrameBack:
		return with("Quadro anterior", shortcut.FrameBack)
	case controlbar.ActionFrameForward:
		return with("Próximo quadro", shortcut.FrameForward)
	case controlbar.ActionMenu:
		switch item.Menu {
		case controlbar.MenuPassage:
			return "Estudar trecho"
		case controlbar.MenuKaraoke:
			return "Modo Karaokê"
		case controlbar.MenuSubtitles:
			return "Legendas"
		}
	}
	return ""
}

// BarEntries word the bar's menu entries, with keys named by char.
func BarEntries(char KeyChar) func(controlbar.Entry, controlbar.State, shortcut.Map) controlbar.EntryText {
	return func(entry controlbar.Entry, state controlbar.State, keys shortcut.Map) controlbar.EntryText {
		return BarEntry(entry, state, keys, char)
	}
}

// BarEntry words one entry of the bar's menus: its text, the value a
// stepper shows, and the key of a command.
func BarEntry(entry controlbar.Entry, state controlbar.State, keys shortcut.Map, char KeyChar) controlbar.EntryText {
	s := state.Study
	switch entry.Kind {
	case controlbar.EntryCommand:
		text := CommandName(entry.Command)
		switch entry.Command {
		case shortcut.PassageMarkStart:
			if s.MarkSet {
				text = "Marcar o início de novo (agora em " + PassageClock(s.MarkStart) + ")"
			}
		case shortcut.PassageMarkEnd:
			if !s.MarkSet {
				text = "Marcar o fim e salvar (marque o início antes)"
			}
		}
		return controlbar.EntryText{Text: text, Key: ShortcutLabel(keys.Of(entry.Command), char)}
	case controlbar.EntryPassage:
		if entry.Index < 0 || entry.Index >= len(s.Saved) {
			return controlbar.EntryText{}
		}
		saved := s.Saved[entry.Index]
		text := PassageName(saved.Start, saved.End)
		if saved.Outside {
			text += " (fora deste vídeo)"
		}
		return controlbar.EntryText{Text: text}
	case controlbar.EntryLabel:
		return controlbar.EntryText{Text: map[controlbar.Field]string{
			controlbar.FieldOptions: "OPÇÕES DO TRECHO", controlbar.FieldSaved: "TRECHOS SALVOS", controlbar.FieldSync: "SINCRONIA",
		}[entry.Field]}
	case controlbar.EntryStepper:
		return stepperText(entry.Field, s)
	}
	return controlbar.EntryText{Text: optionText(entry, state)}
}

func stepperText(field controlbar.Field, s controlbar.StudyState) controlbar.EntryText {
	switch field {
	case controlbar.FieldGap:
		return controlbar.EntryText{Text: "Intervalo entre as voltas", Value: Seconds(s.Options.Gap)}
	case controlbar.FieldGoal:
		value := "sem meta"
		if s.Options.Goal > 0 {
			value = fmt.Sprintf("%d voltas", s.Options.Goal)
		}
		return controlbar.EntryText{Text: "Meta", Value: value}
	case controlbar.FieldMinGap:
		return controlbar.EntryText{Text: "Pausa mínima antes da fala", Value: Seconds(s.MinGap)}
	case controlbar.FieldLead:
		return controlbar.EntryText{Text: "Antecedência da prévia", Value: Seconds(s.Lead)}
	case controlbar.FieldRepeat:
		value := "desligado"
		if s.Repeat > 0 {
			value = fmt.Sprintf("%d×", s.Repeat)
		}
		return controlbar.EntryText{Text: "Repetir cada fala", Value: value}
	}
	return controlbar.EntryText{}
}

func optionText(entry controlbar.Entry, state controlbar.State) string {
	switch entry.Field {
	case controlbar.FieldSnap:
		return "Ajustar às falas"
	case controlbar.FieldLoop:
		return "Repetir sem parar"
	case controlbar.FieldProgressive:
		return "Velocidade progressiva"
	case controlbar.FieldLadder:
		return "Escada de legendas"
	case controlbar.FieldKaraoke:
		return "Ligar o Modo Karaokê"
	case controlbar.FieldIndicator:
		if entry.Value == "countdown" {
			return "Contagem regressiva"
		}
		return "Barra até a fala"
	case controlbar.FieldFill:
		return "Pintar a fala enquanto é dita"
	case controlbar.FieldMode:
		return "Modo: " + ModeName(study.Mode(entry.Value))
	case controlbar.FieldTranslation:
		return "Mostrar a tradução"
	case controlbar.FieldResetSync:
		return "Zerar a sincronia das duas legendas"
	case controlbar.FieldEdit:
		if entry.Value == "translation" {
			return "Corrigir a legenda de baixo…"
		}
		return "Corrigir a legenda de cima…"
	}
	return ""
}

// BarStatus is the study text on the bar: the start marked, the passage
// being studied, or nothing.
func BarStatus(s controlbar.StudyState) string {
	span := func() string { return PassageClock(s.Passage.Start) + "–" + PassageClock(s.Passage.End) }
	switch {
	case s.Active && s.Suspended:
		return "Trecho " + span() + " · fora do intervalo"
	case s.Active && s.AtEnd:
		return "Trecho " + span() + " · fim: Reproduzir repete"
	case s.Active && s.Goal > 0:
		return fmt.Sprintf("Trecho %s · volta %d de %d", span(), s.Lap, s.Goal)
	case s.Active:
		return fmt.Sprintf("Trecho %s · volta %d", span(), s.Lap)
	case s.MarkSet:
		return "Início do trecho: " + PassageClock(s.MarkStart)
	}
	return ""
}
