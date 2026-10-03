package controlbar

import (
	"math"
	"time"

	"lingoreel/internal/domain/layout"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
)

// Menu names a menu of the bar: the passage, the Modo Karaokê and the
// subtitles. The speed list is apart (ActionSpeed).
type Menu int

const (
	MenuNone Menu = iota
	MenuPassage
	MenuKaraoke
	MenuSubtitles
)

// Menus lists the bar's menus in the order of their buttons.
func Menus() []Menu {
	return []Menu{MenuPassage, MenuKaraoke, MenuSubtitles}
}

// EntryKind is what an entry of a menu is.
type EntryKind int

const (
	// EntryCommand runs its Command, as its shortcut does.
	EntryCommand EntryKind = iota + 1
	// EntryToggle turns its Field on or off.
	EntryToggle
	// EntryChoice picks its Value for its Field, among the entries with
	// the same field.
	EntryChoice
	// EntryStepper shows its Field's value with a [−] and a [+]; the
	// buttons are entries of their own with a Delta.
	EntryStepper
	// EntryAction does what its Field says, for its Value: zero the sync,
	// correct a line.
	EntryAction
	// EntryPassage studies the saved passage at Index.
	EntryPassage
	// EntryLabel is a title; EntrySeparator a line. Neither is clicked.
	EntryLabel
	EntrySeparator
)

// Field is what an option entry changes.
type Field int

const (
	FieldNone Field = iota
	// The passage's options, which also apply to the passages the end key
	// closes.
	FieldSnap
	FieldLoop
	FieldGap
	FieldProgressive
	FieldLadder
	FieldGoal
	// The Modo Karaokê.
	FieldKaraoke
	FieldIndicator
	FieldMinGap
	FieldLead
	FieldFill
	FieldRepeat
	// The subtitles.
	FieldMode
	FieldTranslation
	FieldOffsets
	FieldResetSync
	FieldEdit
	// The labels.
	FieldOptions
	FieldSaved
	FieldSync
)

// Entry is one entry of a menu. Texts are not here: the window words them.
type Entry struct {
	Kind    EntryKind
	Command shortcut.Command
	Field   Field
	// Value is the choice of an EntryChoice ("bar", "dual") and the line of
	// FieldEdit and FieldResetSync ("primary", "translation").
	Value string
	// Index is the saved passage of an EntryPassage.
	Index int
	// Delta is -1 or +1 on a stepper's buttons.
	Delta   int
	Checked bool
	Enabled bool
}

// EntryText is how a menu entry reads, as the presentation words it: its
// text, the value it shows on the right (a stepper's "10 s") and the key of
// its command.
type EntryText struct {
	Text  string
	Value string
	Key   string
}

// SavedItem is a saved passage as the passage menu lists it.
type SavedItem struct {
	Start, End time.Duration
	// Outside tells the passage starts after the end of the video.
	Outside bool
}

// StudyState is what the menus and the study text show: the passage being
// marked or studied, its options, the saved passages, the Modo Karaokê and
// the subtitles.
type StudyState struct {
	MarkSet   bool
	MarkStart time.Duration
	// Passage is the passage being studied, if Active.
	Active, Suspended, AtEnd bool
	Passage                  study.Segment
	Lap, Goal                int
	// Options are how the next passage is studied.
	Snap    bool
	Options study.PracticePlan
	Saved   []SavedItem

	Karaoke   bool
	Indicator study.Indicator
	MinGap    time.Duration
	Lead      time.Duration
	Fill      bool
	Repeat    int

	Mode           study.Mode
	HasPrimary     bool
	HasTranslation bool
	TranslationOn  bool
	// Paused allows editing a line.
	Paused bool
}

// Steps of the menus' steppers and the karaoke's limits.
const (
	gapStep     = time.Second
	goalStep    = 1
	timingStep  = time.Second
	repeatStep  = 1
	maxSavedRow = 6
)

// Entries are the entries of a menu for a state, in order.
func Entries(menu Menu, s StudyState) []Entry {
	switch menu {
	case MenuPassage:
		return passageEntries(s)
	case MenuKaraoke:
		return karaokeEntries(s)
	case MenuSubtitles:
		return subtitleEntries(s)
	}
	return nil
}

func passageEntries(s StudyState) []Entry {
	entries := []Entry{
		{Kind: EntryCommand, Command: shortcut.PassageMarkStart, Enabled: true},
		{Kind: EntryCommand, Command: shortcut.PassageMarkEnd, Enabled: s.MarkSet},
	}
	switch {
	case s.Active && s.Suspended:
		entries = append(entries, Entry{Kind: EntryCommand, Command: shortcut.PassageResume, Enabled: true})
	case s.Active:
		entries = append(entries, Entry{Kind: EntryCommand, Command: shortcut.PassageStop, Enabled: true})
	}
	loop := s.Options.Mode == study.EndLoop
	entries = append(entries,
		Entry{Kind: EntrySeparator},
		Entry{Kind: EntryLabel, Field: FieldOptions},
		Entry{Kind: EntryToggle, Field: FieldSnap, Checked: s.Snap, Enabled: s.HasPrimary},
		Entry{Kind: EntryToggle, Field: FieldLoop, Checked: loop, Enabled: true},
	)
	if loop {
		entries = append(entries, stepper(FieldGap, s.Options.Gap > 0, s.Options.Gap < study.MaxLoopGap)...)
	}
	entries = append(entries,
		Entry{Kind: EntryToggle, Field: FieldProgressive, Checked: s.Options.Progressive, Enabled: true},
		Entry{Kind: EntryToggle, Field: FieldLadder, Checked: s.Options.Ladder, Enabled: true},
	)
	entries = append(entries, stepper(FieldGoal, s.Options.Goal > 0, s.Options.Goal < study.MaxGoal)...)
	if len(s.Saved) > 0 {
		entries = append(entries, Entry{Kind: EntrySeparator}, Entry{Kind: EntryLabel, Field: FieldSaved})
		// The latest ones; the panel lists them all.
		first := max(0, len(s.Saved)-maxSavedRow)
		for i := first; i < len(s.Saved); i++ {
			entries = append(entries, Entry{Kind: EntryPassage, Index: i, Enabled: !s.Saved[i].Outside})
		}
	}
	return entries
}

func karaokeEntries(s StudyState) []Entry {
	entries := []Entry{
		{Kind: EntryToggle, Field: FieldKaraoke, Checked: s.Karaoke, Enabled: s.HasPrimary},
		{Kind: EntryChoice, Field: FieldIndicator, Value: string(study.IndicatorBar), Checked: s.Indicator == study.IndicatorBar, Enabled: true},
		{Kind: EntryChoice, Field: FieldIndicator, Value: string(study.IndicatorCountdown), Checked: s.Indicator == study.IndicatorCountdown, Enabled: true},
	}
	entries = append(entries, stepper(FieldMinGap, s.MinGap-timingStep >= study.MinCueGap, s.MinGap+timingStep <= study.MaxCueGap)...)
	entries = append(entries, stepper(FieldLead, s.Lead-timingStep >= study.MinCueLead, s.Lead+timingStep <= study.MaxCueLead)...)
	entries = append(entries, Entry{Kind: EntryToggle, Field: FieldFill, Checked: s.Fill, Enabled: true})
	entries = append(entries, stepper(FieldRepeat, s.Repeat > 0, s.Repeat < study.MaxRepeats)...)
	entries = append(entries,
		Entry{Kind: EntrySeparator},
		Entry{Kind: EntryCommand, Command: shortcut.ReplayLine, Enabled: s.HasPrimary},
		Entry{Kind: EntryCommand, Command: shortcut.RepeatPhrase, Enabled: s.HasPrimary},
	)
	return entries
}

func subtitleEntries(s StudyState) []Entry {
	entries := make([]Entry, 0, 16)
	for _, mode := range study.Modes() {
		entries = append(entries, Entry{Kind: EntryChoice, Field: FieldMode, Value: string(mode), Checked: s.Mode == mode, Enabled: s.HasTranslation})
	}
	entries = append(entries,
		Entry{Kind: EntryToggle, Field: FieldTranslation, Checked: s.TranslationOn, Enabled: s.HasTranslation && s.Mode.Hideable()},
		Entry{Kind: EntrySeparator},
		Entry{Kind: EntryLabel, Field: FieldSync},
		Entry{Kind: EntryCommand, Command: shortcut.SyncPrimaryEarlier, Enabled: s.HasPrimary},
		Entry{Kind: EntryCommand, Command: shortcut.SyncPrimaryLater, Enabled: s.HasPrimary},
		Entry{Kind: EntryCommand, Command: shortcut.SyncTranslationEarlier, Enabled: s.HasTranslation},
		Entry{Kind: EntryCommand, Command: shortcut.SyncTranslationLater, Enabled: s.HasTranslation},
		Entry{Kind: EntryAction, Field: FieldResetSync, Enabled: s.HasPrimary || s.HasTranslation},
		Entry{Kind: EntrySeparator},
		Entry{Kind: EntryAction, Field: FieldEdit, Value: "primary", Enabled: s.HasPrimary},
		Entry{Kind: EntryAction, Field: FieldEdit, Value: "translation", Enabled: s.HasTranslation},
	)
	return entries
}

// stepper is a stepper's row and its [−] and [+] buttons.
func stepper(field Field, down, up bool) []Entry {
	return []Entry{
		{Kind: EntryStepper, Field: field, Enabled: down || up},
		{Kind: EntryStepper, Field: field, Delta: -1, Enabled: down},
		{Kind: EntryStepper, Field: field, Delta: +1, Enabled: up},
	}
}

// Step is how much a stepper's button changes its field.
func Step(field Field) (time.Duration, int) {
	switch field {
	case FieldGap:
		return gapStep, 0
	case FieldMinGap, FieldLead:
		return timingStep, 0
	case FieldGoal:
		return 0, goalStep
	case FieldRepeat:
		return 0, repeatStep
	}
	return 0, 0
}

// Sizes of a menu at 100% scaling, in pixels.
const (
	menuWidth     = 300.0
	menuRow       = 28.0
	menuLabel     = 22.0
	menuSeparator = 9.0
	menuPad       = 6.0
	stepButton    = 24.0
)

// rowHeight is an entry's height before scaling.
func rowHeight(kind EntryKind) float64 {
	switch kind {
	case EntrySeparator:
		return menuSeparator
	case EntryLabel:
		return menuLabel
	}
	return menuRow
}

// layoutMenu places a menu's entries in a panel above its button, kept
// inside the window; entries that do not fit in the height above the bar
// are left out from the end (the saved passages first).
func (b *Bar) layoutMenu(button Item, area layout.Rect, entries []Entry) {
	s := b.Scale
	width := menuWidth * s
	room := b.Panel.Y - area.Y - gap*s
	height := 2 * menuPad * s
	fitted := entries[:0:0]
	for _, entry := range entries {
		// The buttons of a stepper sit on its row.
		if entry.Kind == EntryStepper && entry.Delta != 0 {
			fitted = append(fitted, entry)
			continue
		}
		next := height + rowHeight(entry.Kind)*s
		if next > room {
			break
		}
		height = next
		fitted = append(fitted, entry)
	}
	// No room for a single entry (a window lower than the bar): no menu.
	if len(fitted) == 0 {
		return
	}
	x := math.Min(button.Bounds.Right()-width, area.Right()-width)
	x = math.Max(x, area.X)
	y := math.Max(b.Panel.Y-height-gap*s, area.Y)
	b.MenuPanel = layout.Rect{X: x, Y: y, W: width, H: height}
	row := y + menuPad*s
	var stepRow layout.Rect
	for _, entry := range fitted {
		if entry.Kind == EntryStepper && entry.Delta != 0 {
			size := stepButton * s
			right := stepRow.Right() - menuPad*s
			if entry.Delta < 0 {
				right -= size + gap*s
			}
			bounds := layout.Rect{X: right - size, Y: stepRow.Y + (stepRow.H-size)/2, W: size, H: size}
			b.MenuItems = append(b.MenuItems, Item{Action: ActionEntry, Entry: entry, Bounds: bounds, Enabled: entry.Enabled})
			continue
		}
		h := rowHeight(entry.Kind) * s
		bounds := layout.Rect{X: x, Y: row, W: width, H: h}
		row += h
		action := ActionEntry
		if entry.Kind == EntryLabel || entry.Kind == EntrySeparator || entry.Kind == EntryStepper {
			action = ActionEntryText
			if entry.Kind == EntryStepper {
				stepRow = bounds
			}
		}
		b.MenuItems = append(b.MenuItems, Item{Action: action, Entry: entry, Bounds: bounds, Enabled: entry.Enabled})
	}
}
