package ptbr

import (
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// closeKey is the key that closes a window in Windows, which the menu's
// "Fechar" shows: it is the system's, not a shortcut of the profile.
const closeKey = "Alt+F4"

// MenuTexts words the context menu's items, with keys named by char.
func MenuTexts(char KeyChar) func(contextmenu.Item, shortcut.Map) contextmenu.Text {
	return func(item contextmenu.Item, keys shortcut.Map) contextmenu.Text { return MenuText(item, keys, char) }
}

// MenuText words one item of the context menu: its text and the key the
// profile gives its command, empty for a command without one. People
// forget their shortcuts; the menu shows each one where it applies.
func MenuText(item contextmenu.Item, keys shortcut.Map, char KeyChar) contextmenu.Text {
	key := func(command shortcut.Command) string { return ShortcutLabel(keys.Of(command), char) }
	switch item.Kind {
	case contextmenu.KindSubmenu:
		return contextmenu.Text{Text: menuTitle(item.Title)}
	case contextmenu.KindSeparator:
		return contextmenu.Text{}
	}
	switch item.Action {
	case contextmenu.ActionCommand:
		return contextmenu.Text{Text: menuCommand(item), Key: key(item.Command)}
	case contextmenu.ActionEntry:
		return menuEntry(item, key)
	case contextmenu.ActionSpeed:
		return contextmenu.Text{Text: Speed(item.Speed)}
	case contextmenu.ActionPreset:
		return contextmenu.Text{Text: presetText(item)}
	case contextmenu.ActionAudio:
		return contextmenu.Text{Text: AudioLabel(item.Track)}
	case contextmenu.ActionSubtitle:
		if item.Track.Number == 0 {
			return contextmenu.Text{Text: T("menu.none")}
		}
		return contextmenu.Text{Text: SubtitleLabel(item.Track)}
	case contextmenu.ActionAddSubtitle:
		return contextmenu.Text{Text: T("menu.addSubtitle")}
	case contextmenu.ActionAppearance:
		return contextmenu.Text{Text: T("menu.appearance")}
	case contextmenu.ActionExport:
		if item.Line == contextmenu.LineTranslation {
			return contextmenu.Text{Text: T("menu.exportTranslation")}
		}
		return contextmenu.Text{Text: T("menu.exportPrimary")}
	case contextmenu.ActionSettings:
		if item.Page == contextmenu.PageStudy {
			return contextmenu.Text{Text: T("menu.karaokeSettings")}
		}
		return contextmenu.Text{Text: T("menu.settings")}
	case contextmenu.ActionEditHere:
		return contextmenu.Text{Text: T("menu.editHere")}
	case contextmenu.ActionCopyHere:
		return contextmenu.Text{Text: T("menu.copyHere")}
	case contextmenu.ActionStopVideo:
		return contextmenu.Text{Text: T("menu.stopVideo")}
	case contextmenu.ActionOpenVideo:
		return contextmenu.Text{Text: T("menu.openVideo")}
	case contextmenu.ActionClose:
		return contextmenu.Text{Text: T("menu.close"), Key: closeKey}
	case contextmenu.ActionNone:
		// A line's offset, told in the sync submenu.
		line := T("line.primary")
		if item.Line == contextmenu.LineTranslation {
			line = T("line.translation")
		}
		return contextmenu.Text{Text: T("menu.offset", "line", line, "offset", Offset(subtitle.Offset(item.Duration)))}
	}
	return contextmenu.Text{}
}

// menuCommand names a command in the menu: the shortcut's name, or a
// shorter one where the menu shows the state (play or pause, the sound).
func menuCommand(item contextmenu.Item) string {
	switch item.Command {
	case shortcut.PlayPause:
		if item.Icon == contextmenu.IconPlay {
			return T("bar.tip.play")
		}
		return T("bar.tip.pause")
	case shortcut.Mute:
		return T("menu.mute")
	case shortcut.ReplayLine:
		return T("menu.replayLine")
	}
	return CommandName(item.Command)
}

// menuEntry words an entry of the bar's menus as the context menu shows it.
func menuEntry(item contextmenu.Item, key func(shortcut.Command) string) contextmenu.Text {
	entry := item.Entry
	switch {
	case entry.Kind == controlbar.EntryPassage:
		text := PassageName(item.Segment.Start, item.Segment.End)
		if !item.Enabled {
			text = T("bar.entry.outside", "name", text)
		}
		return contextmenu.Text{Text: text}
	case entry.Field == controlbar.FieldMode:
		// Inside the study mode's submenu: the mode's name alone.
		return contextmenu.Text{Text: ModeName(study.Mode(entry.Value))}
	case entry.Field == controlbar.FieldTranslation:
		return contextmenu.Text{Text: optionText(entry, controlbar.State{}), Key: key(shortcut.Translation)}
	}
	return contextmenu.Text{Text: optionText(entry, controlbar.State{})}
}

// presetText is a ready-made value: a time, a number of repeats or a goal.
func presetText(item contextmenu.Item) string {
	switch item.Field {
	case controlbar.FieldRepeat:
		if item.Count == 0 {
			return T("menu.off")
		}
		return T("bar.stepper.repeatValue", "times", item.Count)
	case controlbar.FieldGoal:
		if item.Count == 0 {
			return T("menu.noGoal")
		}
		return T("bar.stepper.goalValue", "count", item.Count)
	}
	return Seconds(item.Duration)
}

// menuTitle is a submenu's name.
func menuTitle(title contextmenu.Title) string {
	switch title {
	case contextmenu.TitlePlayback:
		return T("menu.title.playback")
	case contextmenu.TitleSpeed:
		return T("menu.title.speed")
	case contextmenu.TitleAudio:
		return T("menu.title.audio")
	case contextmenu.TitleAudioTrack:
		return T("menu.title.audioTrack")
	case contextmenu.TitleSubtitles:
		return T("menu.title.subtitles")
	case contextmenu.TitlePrimary:
		return T("line.primary")
	case contextmenu.TitleTranslation:
		return T("line.translation")
	case contextmenu.TitleSync:
		return T("menu.title.sync")
	case contextmenu.TitleStudyMode:
		return T("menu.title.studyMode")
	case contextmenu.TitleKaraoke:
		return T("menu.title.karaoke")
	case contextmenu.TitleMinGap:
		return T("bar.stepper.minGap")
	case contextmenu.TitleLead:
		return T("bar.stepper.lead")
	case contextmenu.TitleRepeat:
		return T("bar.stepper.repeat")
	case contextmenu.TitlePassage:
		return T("menu.title.passage")
	case contextmenu.TitlePassageOptions:
		return T("menu.title.passageOptions")
	case contextmenu.TitleGap:
		return T("bar.stepper.gap")
	case contextmenu.TitleGoal:
		return T("menu.title.goal")
	case contextmenu.TitleSaved:
		return T("menu.title.saved")
	}
	return ""
}
