package ptbr

import (
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// Speed shows a speed as "1,25×".
func Speed(speed playback.Speed) string {
	return T("speed.value", "value", Number(float64(speed), 2))
}

// WindowTitle is the player window's title for a video file name.
func WindowTitle(video string) string {
	if strings.TrimSpace(video) == "" {
		return T("app.title")
	}
	return T("app.windowTitle", "video", video)
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
// profile gives the command. The tip of a control the player lacks says so.
func BarTip(item controlbar.Item, state controlbar.State, keys shortcut.Map, char KeyChar) string {
	key := func(command shortcut.Command) string { return ShortcutLabel(keys.Of(command), char) }
	tip := barTip(item, state, key)
	if tip != "" && !item.Enabled && item.Action != controlbar.ActionTranslation {
		tip = T("bar.tip.unavailable", "tip", tip)
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
			return with(T("bar.tip.play"), shortcut.PlayPause)
		}
		return with(T("bar.tip.pause"), shortcut.PlayPause)
	case controlbar.ActionBack:
		return with(T("bar.tip.back", "seconds", int(-back/time.Second)), shortcut.SeekBack)
	case controlbar.ActionForward:
		return with(T("bar.tip.forward", "seconds", int(forward/time.Second)), shortcut.SeekForward)
	case controlbar.ActionMute:
		if state.Muted {
			return with(T("bar.tip.unmute"), shortcut.Mute)
		}
		return with(T("bar.tip.mute"), shortcut.Mute)
	case controlbar.ActionVolume:
		level := state.Volume.Clamp()
		if state.Muted {
			level = 0
		}
		volume := T("bar.tip.volume", "percent", level.Percent())
		if keys := strings.TrimSpace(key(shortcut.VolumeUp) + " " + key(shortcut.VolumeDown)); keys != "" {
			volume += " (" + keys + ")"
		}
		return volume
	case controlbar.ActionTrack:
		return T("bar.tip.track")
	case controlbar.ActionTranslation:
		switch {
		case !state.HasTranslation:
			return T("bar.tip.noTranslation")
		case state.TranslationLocked:
			return T("bar.tip.translationLocked")
		case state.TranslationVisible:
			return with(T("bar.tip.hideTranslation"), shortcut.Translation)
		}
		return with(T("bar.tip.showTranslation"), shortcut.Translation)
	case controlbar.ActionSpeed:
		return T("bar.tip.speed")
	case controlbar.ActionFullscreen:
		if state.Fullscreen {
			return with(T("bar.tip.exitFullscreen"), shortcut.ExitFullscreen)
		}
		return with(T("bar.tip.fullscreen"), shortcut.Fullscreen)
	case controlbar.ActionFrameBack:
		return with(T("bar.tip.frameBack"), shortcut.FrameBack)
	case controlbar.ActionFrameForward:
		return with(T("bar.tip.frameForward"), shortcut.FrameForward)
	case controlbar.ActionMenu:
		switch item.Menu {
		case controlbar.MenuPassage:
			return T("bar.menu.passage")
		case controlbar.MenuKaraoke:
			return T("bar.menu.karaoke")
		case controlbar.MenuSubtitles:
			return T("bar.menu.subtitles")
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
				text = T("bar.entry.markStartAgain", "time", PassageClock(s.MarkStart))
			}
		case shortcut.PassageMarkEnd:
			if !s.MarkSet {
				text = T("bar.entry.markEndNoStart")
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
			text = T("bar.entry.outside", "name", text)
		}
		return controlbar.EntryText{Text: text}
	case controlbar.EntryLabel:
		return controlbar.EntryText{Text: labelText(entry.Field)}
	case controlbar.EntryStepper:
		return stepperText(entry.Field, s)
	}
	return controlbar.EntryText{Text: optionText(entry, state)}
}

// labelText is the title of a group of entries.
func labelText(field controlbar.Field) string {
	switch field {
	case controlbar.FieldOptions:
		return T("bar.label.options")
	case controlbar.FieldSaved:
		return T("bar.label.saved")
	case controlbar.FieldSync:
		return T("bar.label.sync")
	}
	return ""
}

func stepperText(field controlbar.Field, s controlbar.StudyState) controlbar.EntryText {
	switch field {
	case controlbar.FieldGap:
		return controlbar.EntryText{Text: T("bar.stepper.gap"), Value: Seconds(s.Options.Gap)}
	case controlbar.FieldGoal:
		value := T("bar.stepper.noGoal")
		if s.Options.Goal > 0 {
			value = T("bar.stepper.goalValue", "count", s.Options.Goal)
		}
		return controlbar.EntryText{Text: T("bar.stepper.goal"), Value: value}
	case controlbar.FieldMinGap:
		return controlbar.EntryText{Text: T("bar.stepper.minGap"), Value: Seconds(s.MinGap)}
	case controlbar.FieldLead:
		return controlbar.EntryText{Text: T("bar.stepper.lead"), Value: Seconds(s.Lead)}
	case controlbar.FieldRepeat:
		value := T("bar.stepper.off")
		if s.Repeat > 0 {
			value = T("bar.stepper.repeatValue", "times", s.Repeat)
		}
		return controlbar.EntryText{Text: T("bar.stepper.repeat"), Value: value}
	}
	return controlbar.EntryText{}
}

func optionText(entry controlbar.Entry, state controlbar.State) string {
	switch entry.Field {
	case controlbar.FieldSnap:
		return T("bar.option.snap")
	case controlbar.FieldLoop:
		return T("bar.option.loop")
	case controlbar.FieldProgressive:
		return T("bar.option.progressive")
	case controlbar.FieldLadder:
		return T("bar.option.ladder")
	case controlbar.FieldKaraoke:
		return T("bar.option.karaoke")
	case controlbar.FieldIndicator:
		if entry.Value == "countdown" {
			return T("bar.option.countdown")
		}
		return T("bar.option.bar")
	case controlbar.FieldFill:
		return T("bar.option.fill")
	case controlbar.FieldMode:
		return T("bar.option.mode", "mode", ModeName(study.Mode(entry.Value)))
	case controlbar.FieldTranslation:
		return T("bar.option.translation")
	case controlbar.FieldResetSync:
		return T("bar.option.resetSync")
	case controlbar.FieldEdit:
		if entry.Value == "translation" {
			return T("bar.option.editTranslation")
		}
		return T("bar.option.editPrimary")
	}
	return ""
}

// BarStatus is the study text on the bar: the start marked, the passage
// being studied, or nothing.
func BarStatus(s controlbar.StudyState) string {
	span := func() string { return PassageClock(s.Passage.Start) + "–" + PassageClock(s.Passage.End) }
	switch {
	case s.Active && s.Suspended:
		return T("bar.status.suspended", "span", span())
	case s.Active && s.AtEnd:
		return T("bar.status.atEnd", "span", span())
	case s.Active && s.Goal > 0:
		return T("bar.status.lapOfGoal", "span", span(), "lap", s.Lap, "goal", s.Goal)
	case s.Active:
		return T("bar.status.lap", "span", span(), "lap", s.Lap)
	case s.MarkSet:
		return T("bar.status.marked", "time", PassageClock(s.MarkStart))
	}
	return ""
}

// EditHint shows under the box that corrects a line over the video.
func EditHint() string { return T("edit.hint") }

// EditEmpty is told when Enter would leave the corrected line empty.
func EditEmpty() string { return T("edit.empty") }
