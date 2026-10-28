// Package contextmenu is the menu the right button opens over the video, as
// plain data: what it holds for a state of the session, never how a system
// draws it, so each system's own menu (the Windows popup menu today) is built
// from the same model. The texts are not here either: the presentation words
// every item, with the key the profile gives its command.
package contextmenu

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// Kind is what an item is.
type Kind int

const (
	// KindItem is chosen: it carries out its Action. One with no action
	// only tells something (a line's offset) and is never enabled.
	KindItem Kind = iota + 1
	// KindSubmenu opens its Children.
	KindSubmenu
	// KindSeparator is a line between groups.
	KindSeparator
)

// Action is what choosing an item does.
type Action int

const (
	ActionNone Action = iota
	// ActionCommand runs Command, as its shortcut does.
	ActionCommand
	// ActionEntry carries out Entry, as the bar's menus do.
	ActionEntry
	// ActionSpeed plays at Speed.
	ActionSpeed
	// ActionPreset gives Field the value Duration (the karaoke mode's times,
	// the pause between laps) or Count (repeats, the goal of laps).
	ActionPreset
	// ActionAudio switches to the audio track Track.
	ActionAudio
	// ActionSubtitle puts Track on Line; a zero Track takes the line away.
	ActionSubtitle
	// ActionAddSubtitle asks for a subtitle file for Line.
	ActionAddSubtitle
	// ActionAppearance opens the settings page of Line's look.
	ActionAppearance
	// ActionExport saves Line as a subtitle file.
	ActionExport
	// ActionSettings opens the settings at Page.
	ActionSettings
	// ActionEditHere corrects the line under the click, in its place.
	ActionEditHere
	// ActionCopyHere copies the line under the click.
	ActionCopyHere
	// ActionStopVideo ends the video; ActionOpenVideo ends it and asks for
	// another; ActionClose closes the window, as Alt+F4.
	ActionStopVideo
	ActionOpenVideo
	ActionClose
)

// Title names a submenu.
type Title int

const (
	TitleNone Title = iota
	TitlePlayback
	TitleSpeed
	TitleAudio
	TitleAudioTrack
	TitleSubtitles
	TitlePrimary
	TitleTranslation
	TitleSync
	TitleStudyMode
	TitleKaraoke
	TitleMinGap
	TitleLead
	TitleRepeat
	TitlePassage
	TitlePassageOptions
	TitleGap
	TitleGoal
	TitleSaved
)

// Icon names an item's picture, which each system draws its own way.
type Icon int

const (
	IconNone Icon = iota
	IconPlay
	IconPause
	IconStop
	IconFullscreen
	IconExitFullscreen
	IconBack
	IconForward
	IconFrameBack
	IconFrameForward
	IconSpeaker
	IconSubtitles
	IconKaraoke
	IconPassage
	IconFolder
	IconSettings
	IconClose
)

// Icons lists every icon, for the systems that draw them ahead.
func Icons() []Icon {
	return []Icon{
		IconPlay, IconPause, IconStop, IconFullscreen, IconExitFullscreen, IconBack, IconForward, IconFrameBack,
		IconFrameForward, IconSpeaker, IconSubtitles, IconKaraoke, IconPassage, IconFolder, IconSettings, IconClose,
	}
}

// Line is a subtitle line: the one on top, or the translation below.
type Line int

const (
	LinePrimary Line = iota
	LineTranslation
)

// Page is a page of the settings, as the app's panel names it.
type Page string

const (
	PageGeneral     Page = "general"
	PagePrimary     Page = "subtitles-primary"
	PageTranslation Page = "subtitles-translation"
	PageStudy       Page = "study"
)

// Valid tells the page is one the menu opens.
func (p Page) Valid() bool {
	return p == PageGeneral || p == PagePrimary || p == PageTranslation || p == PageStudy
}

// Item is one item of the menu.
type Item struct {
	Kind   Kind
	Action Action
	Title  Title
	Icon   Icon
	// What the action works on, by action.
	Command  shortcut.Command
	Entry    controlbar.Entry
	Speed    playback.Speed
	Field    controlbar.Field
	Duration time.Duration
	Count    int
	// Track is the track of ActionAudio and ActionSubtitle; Number 0 is none.
	Track media.Track
	Line  Line
	Page  Page
	// Segment is the saved passage an entry studies, for its name.
	Segment study.Segment
	// Text is the line under the click, which ActionCopyHere copies.
	Text string
	// Checked marks an option that is on; Radio makes it one choice among
	// the items around it.
	Checked, Radio bool
	Enabled        bool
	Children       []Item
}

// Text is how an item reads: its text and the key of its command, empty
// for a command without a key.
type Text struct {
	Text string
	Key  string
}

// Under is the subtitle line under the click.
type Under struct {
	Line Line
	Text string
}

// State is what the menu shows: the bar's state (the playback, the
// player's capabilities, the study), the tracks, what can be undone, the
// line under the click and whether the app's own actions are there.
type State struct {
	Bar controlbar.State
	// Tracks are the video's audio and subtitle tracks; Current the ones
	// playing.
	Tracks  []media.Track
	Current media.Selection
	// Offsets are each line's offset, by Line.
	Offsets          [2]time.Duration
	CanUndo, CanRedo bool
	// Under is the line under the click; nil for a click elsewhere.
	Under *Under
	// App tells the app's own actions are there (files, the settings,
	// another video); the development tool has none.
	App bool
}

// Ready-made values of the menu's choices: the bar's steppers become lists,
// since a menu has no [−] and [+].
var (
	minGapPresets = seconds(3, 5, 8, 10, 15, 20, 30, 45, 60)
	leadPresets   = seconds(2, 3, 4, 5, 6, 8, 10)
	gapPresets    = seconds(0, 1, 2, 3, 4, 5)
	repeatPresets = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	goalPresets   = []int{0, 3, 5, 10, 20, 50, 99}
)

// maxTrackItems bounds a list of tracks; a file with more is broken or hostile.
const maxTrackItems = 40

// speedTolerance absorbs floating point noise when matching a speed.
const speedTolerance = 1e-6

// Build is the menu for a state, top to bottom.
func Build(s State) []Item {
	b := s.Bar
	var items []Item
	if under := s.Under; under != nil {
		items = append(items,
			Item{Kind: KindItem, Action: ActionEditHere, Line: under.Line, Enabled: b.Capabilities.Pause},
			Item{Kind: KindItem, Action: ActionCopyHere, Line: under.Line, Text: under.Text, Enabled: s.App && under.Text != ""},
		)
		if s.App {
			items = append(items, Item{Kind: KindItem, Action: ActionAppearance, Line: under.Line, Page: pageOf(under.Line), Enabled: true})
		}
		items = append(items, separator())
	}
	play := Item{Kind: KindItem, Action: ActionCommand, Command: shortcut.PlayPause, Icon: IconPause, Enabled: b.Capabilities.Pause}
	if b.Paused {
		play.Icon = IconPlay
	}
	items = append(items, play)
	if s.App {
		items = append(items, Item{Kind: KindItem, Action: ActionStopVideo, Icon: IconStop, Enabled: true})
	}
	if b.Fullscreen {
		items = append(items, command(shortcut.ExitFullscreen, IconExitFullscreen, true))
	} else {
		items = append(items, command(shortcut.Fullscreen, IconFullscreen, true))
	}
	items = append(items, separator(),
		playbackMenu(b), audioMenu(s), subtitlesMenu(s), studyModeMenu(b), karaokeMenu(s), passageMenu(b),
		separator(),
	)
	if s.App {
		items = append(items,
			Item{Kind: KindItem, Action: ActionOpenVideo, Icon: IconFolder, Enabled: true},
			Item{Kind: KindItem, Action: ActionSettings, Icon: IconSettings, Page: PageGeneral, Enabled: true},
		)
	}
	items = append(items, Item{Kind: KindItem, Action: ActionClose, Icon: IconClose, Enabled: true})
	return tidy(items)
}

func playbackMenu(b controlbar.State) Item {
	c := b.Capabilities
	speeds := []Item{
		command(shortcut.SpeedUp, IconNone, c.Speed),
		command(shortcut.SpeedDown, IconNone, c.Speed),
		command(shortcut.SpeedNormal, IconNone, c.Speed),
		separator(),
	}
	speeds = append(speeds, speedChoices(b.Speed, c.Speed)...)
	seek := c.Seek
	return submenu(TitlePlayback, IconPlay,
		submenu(TitleSpeed, IconNone, speeds...),
		separator(),
		command(shortcut.SeekBack, IconBack, seek),
		command(shortcut.SeekForward, IconForward, seek),
		command(shortcut.SeekBack10, IconNone, seek),
		command(shortcut.SeekForward10, IconNone, seek),
		command(shortcut.SeekBack30, IconNone, seek),
		command(shortcut.SeekForward30, IconNone, seek),
		command(shortcut.PrecisionBack, IconNone, seek),
		command(shortcut.PrecisionForward, IconNone, seek),
		separator(),
		command(shortcut.FrameBack, IconFrameBack, c.FrameStepBackward || c.FrameStepBackwardEmulated),
		command(shortcut.FrameForward, IconFrameForward, c.FrameStepForward),
	)
}

// speedChoices lists the preset speeds with the current one checked; a
// current speed between presets joins the list in its place, and one that
// is no speed checks nothing.
func speedChoices(current playback.Speed, enabled bool) []Item {
	speeds := playback.SpeedSteps()
	if current.Valid() && !slices.ContainsFunc(speeds, func(speed playback.Speed) bool { return near(speed, current) }) {
		speeds = append(speeds, current)
		slices.Sort(speeds)
	}
	items := make([]Item, 0, len(speeds))
	for _, speed := range speeds {
		items = append(items, Item{
			Kind: KindItem, Action: ActionSpeed, Speed: speed, Radio: true,
			Checked: current.Valid() && near(speed, current), Enabled: enabled,
		})
	}
	return items
}

func near(a, b playback.Speed) bool {
	return math.Abs(float64(a-b)) < speedTolerance
}

func audioMenu(s State) Item {
	c := s.Bar.Capabilities
	var tracks []Item
	for _, track := range limited(s.Tracks, media.TrackAudio) {
		tracks = append(tracks, Item{
			Kind: KindItem, Action: ActionAudio, Track: track, Radio: true,
			Checked: track.Number == s.Current.Audio, Enabled: s.App && c.AudioTrackSelection,
		})
	}
	return submenu(TitleAudio, IconSpeaker,
		submenu(TitleAudioTrack, IconNone, tracks...),
		command(shortcut.VolumeUp, IconNone, c.Volume),
		command(shortcut.VolumeDown, IconNone, c.Volume),
		Item{Kind: KindItem, Action: ActionCommand, Command: shortcut.Mute, Checked: s.Bar.Muted, Enabled: c.Volume},
	)
}

func subtitlesMenu(s State) Item {
	st := s.Bar.Study
	reset := controlbar.Entry{Kind: controlbar.EntryAction, Field: controlbar.FieldResetSync, Enabled: st.HasPrimary || st.HasTranslation}
	translation := controlbar.Entry{
		Kind: controlbar.EntryToggle, Field: controlbar.FieldTranslation, Checked: st.TranslationOn,
		Enabled: st.HasTranslation && st.Mode.Hideable(),
	}
	return submenu(TitleSubtitles, IconSubtitles,
		lineMenu(s, LinePrimary),
		lineMenu(s, LineTranslation),
		separator(),
		entry(translation),
		command(shortcut.ShowBoth, IconNone, st.HasTranslation),
		command(shortcut.PrimaryOnly, IconNone, st.HasTranslation),
		submenu(TitleSync, IconNone,
			command(shortcut.SyncPrimaryEarlier, IconNone, st.HasPrimary),
			command(shortcut.SyncPrimaryLater, IconNone, st.HasPrimary),
			command(shortcut.SyncTranslationEarlier, IconNone, st.HasTranslation),
			command(shortcut.SyncTranslationLater, IconNone, st.HasTranslation),
			separator(),
			Item{Kind: KindItem, Line: LinePrimary, Duration: s.Offsets[LinePrimary]},
			Item{Kind: KindItem, Line: LineTranslation, Duration: s.Offsets[LineTranslation]},
			entry(reset),
		),
		separator(),
		entry(controlbar.Entry{Kind: controlbar.EntryAction, Field: controlbar.FieldEdit, Value: "primary", Enabled: st.HasPrimary}),
		entry(controlbar.Entry{Kind: controlbar.EntryAction, Field: controlbar.FieldEdit, Value: "translation", Enabled: st.HasTranslation}),
		command(shortcut.Undo, IconNone, s.CanUndo),
		command(shortcut.Redo, IconNone, s.CanRedo),
		appItem(s, Item{Kind: KindItem, Action: ActionExport, Line: LinePrimary, Enabled: st.HasPrimary}),
		appItem(s, Item{Kind: KindItem, Action: ActionExport, Line: LineTranslation, Enabled: st.HasTranslation}),
	)
}

// lineMenu lists the subtitles a line can show, the current one checked:
// the video's tracks and the files found or added; "none" only below.
func lineMenu(s State, line Line) Item {
	title, current := TitlePrimary, s.Current.Primary
	if line == LineTranslation {
		title, current = TitleTranslation, s.Current.Translation
	}
	var items []Item
	if line == LineTranslation {
		items = append(items, Item{Kind: KindItem, Action: ActionSubtitle, Line: line, Radio: true, Checked: current == 0, Enabled: s.App})
	}
	for _, track := range limited(s.Tracks, media.TrackSubtitle) {
		items = append(items, Item{
			Kind: KindItem, Action: ActionSubtitle, Track: track, Line: line, Radio: true,
			Checked: track.Number == current, Enabled: s.App && track.Readable,
		})
	}
	items = append(items, separator(),
		appItem(s, Item{Kind: KindItem, Action: ActionAddSubtitle, Line: line, Icon: IconFolder, Enabled: true}),
		appItem(s, Item{Kind: KindItem, Action: ActionAppearance, Line: line, Page: pageOf(line), Enabled: true}),
	)
	return submenu(title, IconNone, items...)
}

func studyModeMenu(b controlbar.State) Item {
	st := b.Study
	var items []Item
	for _, mode := range study.Modes() {
		items = append(items, Item{
			Kind: KindItem, Action: ActionEntry, Radio: true, Checked: st.Mode == mode, Enabled: st.HasTranslation,
			Entry: controlbar.Entry{Kind: controlbar.EntryChoice, Field: controlbar.FieldMode, Value: string(mode), Checked: st.Mode == mode, Enabled: st.HasTranslation},
		})
	}
	items = append(items, separator(),
		command(shortcut.RepeatPhrase, IconNone, st.HasPrimary),
		command(shortcut.RepeatLast3, IconNone, b.Capabilities.Seek),
		command(shortcut.RepeatLast5, IconNone, b.Capabilities.Seek),
		command(shortcut.RepeatLast10, IconNone, b.Capabilities.Seek),
	)
	return submenu(TitleStudyMode, IconNone, items...)
}

func karaokeMenu(s State) Item {
	st := s.Bar.Study
	items := []Item{
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldKaraoke, Checked: st.Karaoke, Enabled: st.HasPrimary}),
		separator(),
	}
	for _, indicator := range []study.Indicator{study.IndicatorBar, study.IndicatorCountdown} {
		on := st.Indicator == indicator
		items = append(items, Item{
			Kind: KindItem, Action: ActionEntry, Radio: true, Checked: on, Enabled: true,
			Entry: controlbar.Entry{Kind: controlbar.EntryChoice, Field: controlbar.FieldIndicator, Value: string(indicator), Checked: on, Enabled: true},
		})
	}
	items = append(items,
		submenu(TitleMinGap, IconNone, durations(controlbar.FieldMinGap, minGapPresets, st.MinGap, study.MinCueGap, study.MaxCueGap)...),
		submenu(TitleLead, IconNone, durations(controlbar.FieldLead, leadPresets, st.Lead, study.MinCueLead, study.MaxCueLead)...),
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldFill, Checked: st.Fill, Enabled: true}),
		submenu(TitleRepeat, IconNone, counts(controlbar.FieldRepeat, repeatPresets, st.Repeat, 0, study.MaxRepeats)...),
		separator(),
		command(shortcut.ReplayLine, IconNone, st.HasPrimary),
		command(shortcut.RepeatPhrase, IconNone, st.HasPrimary),
		separator(),
		appItem(s, Item{Kind: KindItem, Action: ActionSettings, Page: PageStudy, Enabled: true}),
	)
	return submenu(TitleKaraoke, IconKaraoke, items...)
}

func passageMenu(b controlbar.State) Item {
	st := b.Study
	items := []Item{
		command(shortcut.PassageMarkStart, IconNone, true),
		command(shortcut.PassageMarkEnd, IconNone, st.MarkSet),
	}
	switch {
	case st.Active && st.Suspended:
		items = append(items, command(shortcut.PassageResume, IconNone, true))
	case st.Active:
		items = append(items, command(shortcut.PassageStop, IconNone, true))
	}
	loop := st.Options.Mode == study.EndLoop
	options := []Item{
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldSnap, Checked: st.Snap, Enabled: st.HasPrimary}),
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldLoop, Checked: loop, Enabled: true}),
		disabledUnless(submenu(TitleGap, IconNone, durations(controlbar.FieldGap, gapPresets, st.Options.Gap, 0, study.MaxLoopGap)...), loop),
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldProgressive, Checked: st.Options.Progressive, Enabled: true}),
		entry(controlbar.Entry{Kind: controlbar.EntryToggle, Field: controlbar.FieldLadder, Checked: st.Options.Ladder, Enabled: true}),
		submenu(TitleGoal, IconNone, counts(controlbar.FieldGoal, goalPresets, st.Options.Goal, 0, study.MaxGoal)...),
	}
	var saved []Item
	for i, item := range st.Saved {
		saved = append(saved, Item{
			Kind: KindItem, Action: ActionEntry, Segment: study.Segment{Start: item.Start, End: item.End}, Enabled: !item.Outside,
			Entry: controlbar.Entry{Kind: controlbar.EntryPassage, Index: i, Enabled: !item.Outside},
		})
	}
	items = append(items, separator(), submenu(TitlePassageOptions, IconNone, options...), submenu(TitleSaved, IconNone, saved...))
	return submenu(TitlePassage, IconPassage, items...)
}

// durations lists ready-made times for a field with the current one
// checked: a current time that is not one of them joins the list, and a
// time out of the field's limits is left out.
func durations(field controlbar.Field, presets []time.Duration, current, low, high time.Duration) []Item {
	return presetItems(presets, current, low, high, func(value time.Duration) Item {
		return Item{Kind: KindItem, Action: ActionPreset, Field: field, Duration: value}
	})
}

// counts is durations for a field that counts.
func counts(field controlbar.Field, presets []int, current, low, high int) []Item {
	return presetItems(presets, current, low, high, func(value int) Item {
		return Item{Kind: KindItem, Action: ActionPreset, Field: field, Count: value}
	})
}

// presetItems lists ready-made values as radio items, the current one
// checked and joining the list when it is not one of them, the values out of
// the limits left out; item makes the item of a value.
func presetItems[T cmp.Ordered](presets []T, current, low, high T, item func(T) Item) []Item {
	values := slices.Clone(presets)
	if !slices.Contains(values, current) {
		values = append(values, current)
		slices.Sort(values)
	}
	var items []Item
	for _, value := range values {
		if value < low || value > high {
			continue
		}
		it := item(value)
		it.Radio, it.Checked, it.Enabled = true, value == current, true
		items = append(items, it)
	}
	return items
}

// limited is the tracks of a kind, at most maxTrackItems.
func limited(tracks []media.Track, kind media.TrackType) []media.Track {
	var list []media.Track
	for _, track := range tracks {
		if track.Type == kind && len(list) < maxTrackItems {
			list = append(list, track)
		}
	}
	return list
}

func command(command shortcut.Command, icon Icon, enabled bool) Item {
	return Item{Kind: KindItem, Action: ActionCommand, Command: command, Icon: icon, Enabled: enabled}
}

func entry(e controlbar.Entry) Item {
	return Item{
		Kind: KindItem, Action: ActionEntry, Entry: e, Enabled: e.Enabled,
		Checked: e.Checked, Radio: e.Kind == controlbar.EntryChoice,
	}
}

func submenu(title Title, icon Icon, children ...Item) Item {
	children = tidy(children)
	enabled := slices.ContainsFunc(children, func(child Item) bool { return child.Enabled })
	return Item{Kind: KindSubmenu, Title: title, Icon: icon, Children: children, Enabled: enabled}
}

func separator() Item {
	return Item{Kind: KindSeparator}
}

func disabledUnless(item Item, enabled bool) Item {
	item.Enabled = item.Enabled && enabled
	return item
}

// appItem keeps an item of the app's own only when the app is there; a
// zero item is dropped by tidy.
func appItem(s State, item Item) Item {
	if !s.App {
		return Item{}
	}
	return item
}

func pageOf(line Line) Page {
	if line == LineTranslation {
		return PageTranslation
	}
	return PagePrimary
}

// tidy drops the items left out and the separators with nothing to
// separate: at the start, at the end and twice in a row.
func tidy(items []Item) []Item {
	var kept []Item
	for _, item := range items {
		switch {
		case item.Kind == 0:
			continue
		case item.Kind == KindSeparator && (len(kept) == 0 || kept[len(kept)-1].Kind == KindSeparator):
			continue
		}
		kept = append(kept, item)
	}
	for len(kept) > 0 && kept[len(kept)-1].Kind == KindSeparator {
		kept = kept[:len(kept)-1]
	}
	return kept
}

func seconds(values ...int) []time.Duration {
	list := make([]time.Duration, len(values))
	for i, value := range values {
		list[i] = time.Duration(value) * time.Second
	}
	return list
}
