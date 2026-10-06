// Package controlbar lays out the controls shown over the bottom of the
// video when the mouse goes there: the progress track, play and pause, the
// jumps, the volume, the time, the translation, the speed and fullscreen. It
// is pure: the window that draws the bar and takes the clicks lives in the
// infrastructure.
package controlbar

import (
	"math"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
)

// Action is what a part of the bar does.
type Action int

const (
	ActionNone Action = iota
	ActionPlayPause
	ActionBack
	ActionForward
	ActionMute
	ActionVolume
	ActionTrack
	ActionTranslation
	ActionSpeed
	ActionSpeedChoice
	ActionFullscreen
	// ActionFrameBack and ActionFrameForward step one frame.
	ActionFrameBack
	ActionFrameForward
	// ActionMenu opens or closes the item's Menu.
	ActionMenu
	// ActionEntry is a clickable entry of the open menu; ActionEntryText one
	// that only shows (a title, a line, a stepper's row).
	ActionEntry
	ActionEntryText
)

// Sizes at 100% scaling (96 DPI), in pixels.
const (
	panelHeight = 58.0
	sidePadding = 12.0
	trackZone   = 16.0
	trackThick  = 4.0
	button      = 40.0
	gap         = 4.0
	volumeWidth = 72.0
	timeWidth   = 124.0
	speedWidth  = 60.0
	// statusMin is the least width worth writing the study text in.
	statusMin     = 120.0
	menuColumns   = 3
	menuCell      = 28.0
	menuCellWidth = 58.0
	menuPadding   = 6.0
	// tooltipRoom is the room above the panel for a button's tip.
	tooltipRoom = 34.0
	// zoneShare is the bottom part of the window where the mouse shows the
	// bar; it is never smaller than zonePanels panels.
	zoneShare  = 0.2
	zonePanels = 2.0
	// maxScale bounds the scaling (1536 DPI), far above any monitor, so no
	// size overflows.
	maxScale = 16.0
)

// State is what the bar shows.
type State struct {
	Position, Duration time.Duration
	Paused             bool
	Speed              playback.Speed
	Volume             playback.Volume
	Muted              bool
	HasTranslation     bool
	TranslationVisible bool
	// TranslationLocked tells the study mode always shows the translation:
	// its button cannot hide it.
	TranslationLocked bool
	Fullscreen        bool
	Capabilities      playback.Capabilities
	// SpeedMenuOpen shows the list of speeds above the speed button.
	SpeedMenuOpen bool
	// OpenMenu is the menu shown above its button, MenuNone for none.
	OpenMenu Menu
	// Study is what the menus and the study text show.
	Study StudyState
}

// Item is a clickable part of the bar.
type Item struct {
	Action Action
	// Speed is the choice of an ActionSpeedChoice item, and the current
	// speed on the ActionSpeed button.
	Speed  playback.Speed
	Bounds layout.Rect
	// Enabled is false when the player lacks the feature: the item is shown
	// dimmed, keeps its tip and does nothing.
	Enabled bool
	// Active marks a pressed toggle: translation shown, sound muted,
	// fullscreen on, the current speed, an open menu.
	Active bool
	// Menu is the menu of an ActionMenu button; Entry the entry of an
	// ActionEntry or ActionEntryText.
	Menu  Menu
	Entry Entry
}

// Bar is a laid out control bar, in the window's client pixels.
type Bar struct {
	Scale float64
	// Band is the part of the window the bar may draw on: the panel, the
	// speed list and the tips above it.
	Band  layout.Rect
	Panel layout.Rect
	// Track is the visible progress line; Played is the share of it done.
	Track  layout.Rect
	Played float64
	// VolumeTrack is the volume line, zero when the bar is too narrow for it.
	// VolumeLevel is the share of it filled, of the player's ceiling;
	// VolumeFull is where 100% sits on it (1 when the ceiling is 100%), the
	// rest being the player's amplification.
	VolumeTrack   layout.Rect
	VolumeLevel   float64
	VolumeFull    float64
	volumeCeiling playback.Volume
	// Time is where the "current / total" text goes, zero when dropped.
	Time layout.Rect
	// Menu is the speed list's background, zero when closed.
	Menu  layout.Rect
	Items []Item
	// Status is where the study text goes ("Start: 10:38"), zero when
	// dropped.
	Status layout.Rect
	// MenuPanel is the open menu's background and MenuItems its entries,
	// zero and empty when none is open.
	MenuPanel layout.Rect
	MenuItems []Item
}

// Scale converts the DPI of the window into the bar's scale.
func Scale(dpi int) float64 {
	if dpi <= 0 {
		return 1
	}
	return float64(dpi) / layout.BaseDPI
}

// PanelHeight is the height of the bar's panel at a scale.
func PanelHeight(scale float64) float64 {
	return panelHeight * scale
}

// InZone reports whether a mouse at height y of a window area is low enough
// to show the bar.
func InZone(area layout.Rect, y, scale float64) bool {
	if !area.Valid() {
		return false
	}
	zone := math.Max(zoneShare*area.H, zonePanels*PanelHeight(scale))
	return y >= area.Bottom()-zone && y < area.Bottom()
}

// Layout places the bar at the bottom of a window area. Narrow windows drop
// parts in this order: the study text, the volume line, the frame steps,
// the jumps, the time, the speed, the translation, the mute button and the
// menus; play, fullscreen and the track stay.
func Layout(area layout.Rect, scale float64, state State) Bar {
	if !area.Valid() || !(scale > 0 && scale <= maxScale) {
		return Bar{}
	}
	s := scale
	panel := layout.Rect{X: area.X, Y: area.Bottom() - panelHeight*s, W: area.W, H: panelHeight * s}
	bar := Bar{Scale: s, Panel: panel}
	caps := state.Capabilities

	inner := layout.Rect{X: panel.X + sidePadding*s, W: math.Max(panel.W-2*sidePadding*s, 0)}
	bar.Track = layout.Rect{X: inner.X, Y: panel.Y + (trackZone-trackThick)/2*s, W: inner.W, H: trackThick * s}
	bar.Played = fraction(state.Position, state.Duration)
	trackHit := layout.Rect{X: inner.X, Y: panel.Y, W: inner.W, H: trackZone * s}

	parts := fit(inner.W / s)
	row := panel.Y + trackZone*s

	// Left group.
	x := inner.X
	add := func(item Item, width float64) {
		item.Bounds = layout.Rect{X: x, Y: row, W: width * s, H: button * s}
		bar.Items = append(bar.Items, item)
		x += (width + gap) * s
	}
	add(Item{Action: ActionPlayPause, Enabled: caps.Pause, Active: !state.Paused}, button)
	if parts.jumps {
		add(Item{Action: ActionBack, Enabled: caps.Seek}, button)
		add(Item{Action: ActionForward, Enabled: caps.Seek}, button)
	}
	if parts.frames {
		back := caps.FrameStepBackward || caps.FrameStepBackwardEmulated
		add(Item{Action: ActionFrameBack, Enabled: back}, button)
		add(Item{Action: ActionFrameForward, Enabled: caps.FrameStepForward}, button)
	}
	if parts.mute {
		add(Item{Action: ActionMute, Enabled: caps.Volume, Active: state.Muted}, button)
	}
	if parts.volume {
		slider := layout.Rect{X: x, Y: row, W: volumeWidth * s, H: button * s}
		bar.VolumeTrack = layout.Rect{X: slider.X + gap*s, Y: row + (button-trackThick)/2*s, W: slider.W - 2*gap*s, H: trackThick * s}
		ceiling := caps.VolumeCeiling()
		bar.volumeCeiling = ceiling
		bar.VolumeLevel = float64(state.Volume.Within(ceiling) / ceiling)
		bar.VolumeFull = float64(playback.FullVolume / ceiling)
		if state.Muted {
			bar.VolumeLevel = 0
		}
		bar.Items = append(bar.Items, Item{Action: ActionVolume, Bounds: slider, Enabled: caps.Volume})
		x += (volumeWidth + gap) * s
	}
	if parts.time {
		bar.Time = layout.Rect{X: x, Y: row, W: timeWidth * s, H: button * s}
		x += (timeWidth + gap) * s
	}

	// Right group, from the edge inwards.
	right := inner.Right()
	place := func(item Item, width float64) Item {
		right -= width * s
		item.Bounds = layout.Rect{X: right, Y: row, W: width * s, H: button * s}
		right -= gap * s
		bar.Items = append(bar.Items, item)
		return item
	}
	place(Item{Action: ActionFullscreen, Enabled: true, Active: state.Fullscreen}, button)
	var speed Item
	if parts.speed {
		speed = place(Item{Action: ActionSpeed, Speed: state.Speed, Enabled: caps.Speed, Active: state.SpeedMenuOpen}, speedWidth)
	}
	if parts.translation {
		place(Item{Action: ActionTranslation, Enabled: state.HasTranslation && !state.TranslationLocked, Active: state.TranslationVisible && state.HasTranslation}, button)
	}
	var open Item
	if parts.menus {
		menus := Menus()
		for i := len(menus) - 1; i >= 0; i-- {
			item := place(Item{Action: ActionMenu, Menu: menus[i], Enabled: true, Active: state.OpenMenu == menus[i]}, button)
			if item.Active {
				open = item
			}
		}
	}
	// The study text takes the room left between the two groups.
	if parts.status && right-x >= statusMin*s {
		bar.Status = layout.Rect{X: x, Y: row, W: right - x, H: button * s}
	}

	if state.SpeedMenuOpen && parts.speed && caps.Speed {
		bar.addMenu(speed, area, state.Speed)
	}
	if open.Action == ActionMenu {
		bar.layoutMenu(open, area, Entries(open.Menu, state.Study))
	}
	// The track takes the clicks left over at the top of the panel; a window
	// narrower than the margins has none.
	if trackHit.Valid() {
		bar.Items = append(bar.Items, Item{Action: ActionTrack, Bounds: trackHit, Enabled: caps.Seek && state.Duration > 0})
	}

	top := panel.Y - tooltipRoom*s
	if bar.Menu.Valid() {
		top = math.Min(top, bar.Menu.Y-tooltipRoom*s)
	}
	if bar.MenuPanel.Valid() {
		top = math.Min(top, bar.MenuPanel.Y)
	}
	top = math.Max(top, area.Y)
	bar.Band = layout.Rect{X: area.X, Y: top, W: area.W, H: area.Bottom() - top}
	return bar
}

// addMenu lays out the speed list above the speed button, in rows of
// menuColumns, kept inside the window.
func (b *Bar) addMenu(speed Item, area layout.Rect, current playback.Speed) {
	s := b.Scale
	steps := playback.SpeedSteps()
	rows := (len(steps) + menuColumns - 1) / menuColumns
	width := (menuColumns*menuCellWidth + 2*menuPadding) * s
	height := (float64(rows)*menuCell + 2*menuPadding) * s
	x := math.Min(speed.Bounds.Right()-width, area.Right()-width)
	x = math.Max(x, area.X)
	y := math.Max(b.Panel.Y-height-gap*s, area.Y)
	b.Menu = layout.Rect{X: x, Y: y, W: width, H: height}
	for i, step := range steps {
		column, row := i%menuColumns, i/menuColumns
		b.Items = append(b.Items, Item{
			Action: ActionSpeedChoice, Speed: step, Enabled: true,
			Active: math.Abs(float64(step-current)) < 1e-6,
			Bounds: layout.Rect{
				X: x + (menuPadding+float64(column)*menuCellWidth)*s,
				Y: y + (menuPadding+float64(row)*menuCell)*s,
				W: menuCellWidth * s, H: menuCell * s,
			},
		})
	}
}

// parts tells which optional parts fit.
type parts struct {
	jumps, frames, mute, volume, time, status, speed, translation, menus bool
}

// fit drops the optional parts, least useful first, until the rest fits in
// width (at 100% scaling). The menus go last: they reach everything else.
func fit(width float64) parts {
	p := parts{jumps: true, frames: true, mute: true, volume: true, time: true, status: true, speed: true, translation: true, menus: true}
	need := func() float64 {
		total := 2*button + gap // play and fullscreen
		add := func(on bool, w float64) {
			if on {
				total += w + gap
			}
		}
		add(p.jumps, 2*button+gap)
		add(p.frames, 2*button+gap)
		add(p.mute, button)
		add(p.volume, volumeWidth)
		add(p.time, timeWidth)
		add(p.status, statusMin)
		add(p.speed, speedWidth)
		add(p.translation, button)
		add(p.menus, float64(len(Menus()))*(button+gap)-gap)
		return total
	}
	for _, drop := range []*bool{&p.status, &p.volume, &p.frames, &p.jumps, &p.time, &p.speed, &p.translation, &p.mute, &p.menus} {
		if need() <= width {
			break
		}
		*drop = false
	}
	return p
}

// HitTest returns the item under a point: the open menu's entries first,
// then the speed list, the buttons and the track. ActionNone means nothing
// is there; a point on a menu's title or line is nothing too.
func (b Bar) HitTest(x, y float64) Item {
	if b.MenuPanel.Contains(x, y) {
		// From the end: a stepper's buttons come after its row.
		for i := len(b.MenuItems) - 1; i >= 0; i-- {
			if item := b.MenuItems[i]; item.Action == ActionEntry && item.Bounds.Contains(x, y) {
				return item
			}
		}
		return Item{}
	}
	for i := len(b.Items) - 1; i >= 0; i-- {
		if b.Items[i].Action == ActionSpeedChoice && b.Items[i].Bounds.Contains(x, y) {
			return b.Items[i]
		}
	}
	for _, item := range b.Items {
		if item.Action != ActionSpeedChoice && item.Bounds.Contains(x, y) {
			return item
		}
	}
	return Item{}
}

// Covers reports whether the bar takes a point: the panel or the open speed
// list. Elsewhere in the band the video shows through.
func (b Bar) Covers(x, y float64) bool {
	return b.Panel.Contains(x, y) || b.Menu.Contains(x, y) || b.MenuPanel.Contains(x, y)
}

// TrackPosition is the playback position for a point at x on the track,
// kept inside the video.
func (b Bar) TrackPosition(x float64, duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}
	share := along(x, b.Track)
	return time.Duration(math.Round(share * float64(duration)))
}

// VolumeAt is the volume for a point at x on the volume line, which spans
// the player's ceiling.
func (b Bar) VolumeAt(x float64) playback.Volume {
	ceiling := b.volumeCeiling
	if ceiling <= 0 {
		ceiling = playback.FullVolume
	}
	return (playback.Volume(along(x, b.VolumeTrack)) * ceiling).Within(ceiling)
}

// along is the share of a line's width before x, from 0 to 1.
func along(x float64, line layout.Rect) float64 {
	if !(line.W > 0) || math.IsNaN(x) {
		return 0
	}
	return math.Min(math.Max((x-line.X)/line.W, 0), 1)
}

// fraction is the share of the video already played, from 0 to 1.
func fraction(position, duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	return math.Min(math.Max(float64(position)/float64(duration), 0), 1)
}
