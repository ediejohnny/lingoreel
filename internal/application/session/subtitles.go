package session

import (
	"math"
	"sync"
	"time"
	"unicode"

	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Line identifies one of the two subtitle lines.
type Line int

const (
	LinePrimary Line = iota
	LineTranslation
)

// Track is a subtitle timeline shown with an offset.
type Track struct {
	Timeline *subtitle.Timeline
	Offset   subtitle.Offset
}

// Visible holds the events on screen for both lines.
type Visible struct {
	Primary     []subtitle.Event
	Translation []subtitle.Event
	// Cue is set when the primary line is the next one, shown before its
	// time by the karaoke mode; Primary and Translation then hold that line.
	Cue *study.Cue
	// Fill is the share of the primary line already spoken, for the karaoke
	// mode's fill; zero when nothing is filled.
	Fill float64
	// Filling tells the primary line is being painted: its fill may move
	// even while it is still zero, before the voice begins.
	Filling bool
	// Playing tells the video plays, which the screen shows too: a line
	// picked for a correction is let go when the video plays again.
	Playing bool
}

// Drawing steps: a preview's time left and a fill below them look the same
// at 25 frames per second, so they do not count as changes.
const (
	cueStep  = KaraokeFrame
	fillStep = 1.0 / 200
)

// Equal reports whether both snapshots show the same thing: the same text,
// the same preview step, the same fill step, and the video playing or not.
func (v Visible) Equal(other Visible) bool {
	if !sameEvents(v.Primary, other.Primary) || !sameEvents(v.Translation, other.Translation) {
		return false
	}
	if (v.Cue == nil) != (other.Cue == nil) {
		return false
	}
	if v.Cue != nil && (v.Cue.Event.ID != other.Cue.Event.ID || v.Cue.Remaining/cueStep != other.Cue.Remaining/cueStep) {
		return false
	}
	return v.Playing == other.Playing && math.Round(v.Fill/fillStep) == math.Round(other.Fill/fillStep)
}

// Animating tells the screen changes by itself while the video plays: a
// preview counting down or a line filling in.
func (v Visible) Animating() bool {
	return v.Cue != nil || (v.Filling && v.Fill < 1)
}

// KaraokeFrame is how often a preview or a fill is redrawn while the video
// plays: 25 frames per second, and no polling otherwise.
const KaraokeFrame = 40 * time.Millisecond

// Karaoke is the session's karaoke mode.
type Karaoke struct {
	Enabled bool
	Rules   study.CueRules
	// Fill paints the primary line as it is spoken.
	Fill bool
}

// VoiceFill paints a line, given by its text and its time on the playback
// clock, along its actor's voice at a position; false leaves the line to the
// estimate from its letters.
type VoiceFill func(text string, start, end, position time.Duration) (float64, bool)

func sameEvents(a, b []subtitle.Event) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Text() != b[i].Text() {
			return false
		}
	}
	return true
}

// maxVisiblePerLine bounds the events shown at once on one line; a hostile
// file can stack thousands of overlapping events. The latest ones win.
const maxVisiblePerLine = 4

// Subtitles decides which events are visible at a playback position.
// It is safe for concurrent use.
type Subtitles struct {
	mu      sync.RWMutex
	lines   [2]*Track
	karaoke Karaoke
	voice   VoiceFill
	// ownTiming shows the translation on its own file's time instead of
	// with the study line it translates.
	ownTiming bool
	changes   chan struct{}
}

// NewSubtitles creates the subtitles of a session; either track may be nil.
func NewSubtitles(primary, translation *Track) *Subtitles {
	return &Subtitles{lines: [2]*Track{primary, translation}, changes: make(chan struct{}, 1)}
}

// Changes signals every change of what the lines show at a given position,
// such as a new offset, so the runner redraws at once.
func (s *Subtitles) Changes() <-chan struct{} {
	return s.changes
}

func (s *Subtitles) notify() {
	select {
	case s.changes <- struct{}{}:
	default:
	}
}

// Valid tells the line is one of the two the screen shows.
func (l Line) Valid() bool {
	return l == LinePrimary || l == LineTranslation
}

// SetKaraoke turns the karaoke mode on or off and changes its rules, and
// signals the change.
func (s *Subtitles) SetKaraoke(karaoke Karaoke) {
	s.mu.Lock()
	s.karaoke = karaoke
	s.mu.Unlock()
	s.notify()
}

// SetOwnTiming shows the translation on its own file's time (true) or
// coming and going with the study line it translates (false, the default),
// and signals the change.
func (s *Subtitles) SetOwnTiming(own bool) {
	s.mu.Lock()
	s.ownTiming = own
	s.mu.Unlock()
	s.notify()
}

// SetVoice paints the primary line along the actor's voice, or by the
// estimate (nil), and signals the change.
func (s *Subtitles) SetVoice(voice VoiceFill) {
	s.mu.Lock()
	s.voice = voice
	s.mu.Unlock()
	s.notify()
}

// Karaoke returns the karaoke mode's state.
func (s *Subtitles) Karaoke() Karaoke {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.karaoke
}

// At returns the events visible at a playback position. The translation
// comes and goes with the study line it translates, unless it keeps its own
// time. With the karaoke mode on, a silence before a line shows that line
// ahead of time, with its translation, and a line being spoken carries its
// fill.
func (s *Subtitles) At(position time.Duration) Visible {
	s.mu.RLock()
	defer s.mu.RUnlock()
	visible := Visible{
		Primary:     activeAt(s.lines[LinePrimary], position),
		Translation: activeAt(s.lines[LineTranslation], position),
	}
	primary := s.lines[LinePrimary]
	if !s.karaoke.Enabled || primary == nil || primary.Timeline == nil {
		visible.Translation = s.translationWith(visible.Primary, visible.Translation, position)
		return visible
	}
	if len(visible.Primary) == 0 {
		if cue, upcoming := study.UpcomingCue(primary.Timeline, primary.Offset, position, s.karaoke.Rules); upcoming {
			visible.Cue = &cue
			visible.Primary = []subtitle.Event{cue.Event}
			if s.ownTiming && len(visible.Translation) == 0 {
				visible.Translation = activeAt(s.lines[LineTranslation], cue.Start)
			}
		}
		visible.Translation = s.translationWith(visible.Primary, visible.Translation, position)
		return visible
	}
	visible.Translation = s.translationWith(visible.Primary, visible.Translation, position)
	if s.karaoke.Fill {
		visible.Fill, visible.Filling = s.fill(primary, visible.Primary, position)
	}
	return visible
}

// fill is the share of the study events shown that is painted, and whether
// they are being painted. The latest one is being spoken; one shown with it,
// its time not over yet (the next line came before it went), is all painted,
// since the actor has moved on. The share counts the visible characters as
// the drawing does (layout.FillWidths), so those earlier lines end exactly
// painted. Called with s.mu held.
func (s *Subtitles) fill(primary *Track, shown []subtitle.Event, position time.Duration) (float64, bool) {
	line := shown[len(shown)-1]
	if len(shown) == 1 && !study.SpeechUnits(line.Text()).Spoken() {
		return 0, false // only a sound's description, or music: never painted
	}
	// Whatever paints the line, it is all painted before the line goes.
	start, end := primary.Offset.PlaybackTime(line.Start), primary.Offset.PlaybackTime(line.End)
	spoken := study.SpokenFraction(line, primary.Offset.TrackTime(position))
	if s.voice != nil {
		if fill, found := s.voice(line.Text(), start, end, position); found {
			spoken = fill
		}
	}
	spoken = study.FinishedFill(spoken, position, start, end)
	if len(shown) == 1 {
		return spoken, true
	}
	before, total := 0, 0
	for _, event := range shown {
		total += visibleCharacters(event.Text())
	}
	for _, event := range shown[:len(shown)-1] {
		before += visibleCharacters(event.Text())
	}
	if total == 0 {
		return 0, false
	}
	return (float64(before) + spoken*float64(total-before)) / float64(total), true
}

// visibleCharacters counts a text's characters that are not spaces, as the
// drawing of the fill does.
func visibleCharacters(text string) int {
	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// Track returns a line's track as it is now (offset included); nil for a
// missing or unknown line.
func (s *Subtitles) Track(line Line) *Track {
	if !line.Valid() {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if track := s.lines[line]; track != nil {
		copied := *track
		return &copied
	}
	return nil
}

// translationWith is the translation shown with the study events shown:
// those on its own time when it keeps them (or with no study line loaded),
// else the ones that say what the study events say, for as long as those
// show. Called with s.mu held.
func (s *Subtitles) translationWith(shown, own []subtitle.Event, position time.Duration) []subtitle.Event {
	primary, translation := s.lines[LinePrimary], s.lines[LineTranslation]
	if s.ownTiming || primary == nil || primary.Timeline == nil || translation == nil || translation.Timeline == nil {
		return own
	}
	return subtitle.Paired(shown,
		subtitle.Placed{Timeline: primary.Timeline, Offset: primary.Offset},
		subtitle.Placed{Timeline: translation.Timeline, Offset: translation.Offset},
		position, maxVisiblePerLine)
}

func activeAt(track *Track, position time.Duration) []subtitle.Event {
	if track == nil || track.Timeline == nil {
		return nil
	}
	active := track.Timeline.ActiveAt(track.Offset.TrackTime(position))
	// ActiveAt orders by Start, so the latest events are at the end.
	return active[max(len(active)-maxVisiblePerLine, 0):]
}

// NextChangeAfter returns the earliest playback position after the given one
// at which either line changes.
func (s *Subtitles) NextChangeAfter(position time.Duration) (time.Duration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var next time.Duration
	found := false
	for _, track := range s.lines {
		if track == nil || track.Timeline == nil {
			continue
		}
		change, ok := track.Timeline.NextChangeAfter(track.Offset.TrackTime(position))
		if !ok {
			continue
		}
		if at := track.Offset.PlaybackTime(change); !found || at < next {
			next, found = at, true
		}
	}
	if primary := s.lines[LinePrimary]; s.karaoke.Enabled && primary != nil {
		if begin, ok := study.NextCueStartAfter(primary.Timeline, primary.Offset, position, s.karaoke.Rules); ok && (!found || begin < next) {
			next, found = begin, true
		}
	}
	return next, found
}

// Offset returns the offset of a line; zero for a missing or unknown line.
func (s *Subtitles) Offset(line Line) subtitle.Offset {
	if !line.Valid() {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if track := s.lines[line]; track != nil {
		return track.Offset
	}
	return 0
}

// ShiftOffset moves the offset of a line by delta, within ±MaxOffset, and
// signals the change. It returns the new offset, or false for a missing or
// unknown line, which does not change.
func (s *Subtitles) ShiftOffset(line Line, delta time.Duration) (subtitle.Offset, bool) {
	if !line.Valid() {
		return 0, false
	}
	s.mu.Lock()
	track := s.lines[line]
	if track == nil {
		s.mu.Unlock()
		return 0, false
	}
	updated := *track
	updated.Offset = track.Offset.Shift(delta)
	s.lines[line] = &updated
	s.mu.Unlock()
	s.notify()
	return updated.Offset, true
}

// SetTimeline replaces a line's events, keeping its offset, and signals the
// change; it has no effect on a missing or unknown line.
func (s *Subtitles) SetTimeline(line Line, timeline *subtitle.Timeline) {
	if !line.Valid() || timeline == nil {
		return
	}
	s.mu.Lock()
	track := s.lines[line]
	if track != nil {
		updated := *track
		updated.Timeline = timeline
		s.lines[line] = &updated
	}
	s.mu.Unlock()
	if track != nil {
		s.notify()
	}
}

// SetTrack puts another track on a line, or none (nil), and signals the
// change: switching the subtitle of a line while the video plays, adding a
// translation or taking it away. An unknown line does not change.
func (s *Subtitles) SetTrack(line Line, track *Track) {
	if !line.Valid() {
		return
	}
	var kept *Track
	if track != nil {
		copied := *track
		kept = &copied
	}
	s.mu.Lock()
	s.lines[line] = kept
	s.mu.Unlock()
	s.notify()
}

// SetOffset changes the offset of a line and signals the change; it has no
// effect on a missing or unknown line.
func (s *Subtitles) SetOffset(line Line, offset subtitle.Offset) {
	if !line.Valid() {
		return
	}
	s.mu.Lock()
	track := s.lines[line]
	if track != nil {
		updated := *track
		updated.Offset = offset
		s.lines[line] = &updated
	}
	s.mu.Unlock()
	if track != nil {
		s.notify()
	}
}
