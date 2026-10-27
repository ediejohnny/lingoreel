// Package project models what the user keeps about one video: the
// corrections to its subtitles, how each track is synchronized, the choices
// last made for it and the passages saved for study. It lives beside the
// video and never changes the video itself.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Version is the current version of the project file. Version 2 added how
// the video was left (Last) and each passage's study plan; version 3 the
// subtitle files picked from elsewhere (AddedFiles).
const Version = 3

// Limits of a project read from disk, which is not trusted: a hand-edited or
// hostile file cannot make the app hold more than this.
const (
	MaxTracks         = 64
	MaxEditsPerTrack  = 100_000
	MaxTextRunes      = 2000
	MaxKeyRunes       = 512
	MaxPassages       = 500
	MaxPassageNameLen = 80
	// MaxAddedFiles bounds the subtitle files picked for one video.
	MaxAddedFiles = 16
	// MaxTime bounds every time in the file, far above any film.
	MaxTime = 100 * time.Hour
)

var (
	// ErrInvalid reports a project with values outside the limits.
	ErrInvalid = errors.New("invalid project")
	// ErrNewerVersion reports a file written by a newer version of the app,
	// which must not be overwritten.
	ErrNewerVersion = errors.New("project written by a newer version")
)

// hashPattern is the form of HashOf.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Millis is a time in whole milliseconds, as the file writes it.
type Millis int64

// MillisOf converts a time, rounded to the millisecond.
func MillisOf(d time.Duration) Millis {
	return Millis(d.Round(time.Millisecond) / time.Millisecond)
}

// Duration converts back.
func (m Millis) Duration() time.Duration {
	return time.Duration(m) * time.Millisecond
}

// Project is what the file holds for one video.
type Project struct {
	// Tracks are keyed by the track's identity in the video (see the
	// persistence adapter), not its number, which can change.
	Tracks   map[string]Track `json:"tracks"`
	Choice   Choice           `json:"choice"`
	Passages []Passage        `json:"passages"`
	Last     Last             `json:"last"`
	// AddedFiles are the subtitle files picked from outside the video's
	// folder, as the persistence adapter writes their paths, so they come
	// back the next time the video opens.
	AddedFiles []string `json:"addedFiles"`
}

// Last is how the video was left: where, at what speed and with which karaoke
// mode. Zero values keep the profile's, and a zero position starts from
// the beginning.
type Last struct {
	Position Millis   `json:"position"`
	Speed    float64  `json:"speed,omitempty"`
	Karaoke  *Karaoke `json:"karaoke,omitempty"`
}

// resumeLead and resumeTail bound where going on with the video is offered:
// not in its first seconds, not in its credits.
const (
	resumeLead = 30 * time.Second
	resumeTail = 60 * time.Second
)

// ResumeAt is where to offer going on with a video of the given duration
// (zero when unknown): where it was left, past its first seconds and before
// its credits; false offers nothing.
func (l Last) ResumeAt(duration time.Duration) (time.Duration, bool) {
	position := l.Position.Duration()
	if position < resumeLead || (duration > 0 && position > duration-resumeTail) {
		return 0, false
	}
	return position, true
}

// Karaoke is the karaoke mode as last used on the video: on or off, its
// timing, the indicator, the fill and the repeats. The colors stay with the
// profile, which owns the look.
type Karaoke struct {
	Enabled   bool            `json:"enabled"`
	MinGap    Millis          `json:"minGap"`
	Lead      Millis          `json:"lead"`
	Indicator study.Indicator `json:"indicator"`
	Fill      bool            `json:"fill"`
	Repeat    int             `json:"repeat"`
}

// Rules are the preview's timing rules.
func (k Karaoke) Rules() study.CueRules {
	return study.CueRules{MinGap: k.MinGap.Duration(), Lead: k.Lead.Duration()}
}

// PassagePlan is how a saved passage is studied, so "Estudar" plays it the
// way it was set up: pausing or looping at the end, and the helps.
type PassagePlan struct {
	Loop        bool   `json:"loop"`
	Gap         Millis `json:"gap"`
	Progressive bool   `json:"progressive"`
	Ladder      bool   `json:"ladder"`
	Goal        int    `json:"goal"`
}

// PlanOf keeps a practice plan in the file's form.
func PlanOf(plan study.PracticePlan) PassagePlan {
	return PassagePlan{Loop: plan.Mode == study.EndLoop, Gap: MillisOf(plan.Gap),
		Progressive: plan.Progressive, Ladder: plan.Ladder, Goal: plan.Goal}
}

// Practice is the plan as the study rules take it.
func (p PassagePlan) Practice() study.PracticePlan {
	mode := study.EndPause
	if p.Loop {
		mode = study.EndLoop
	}
	return study.PracticePlan{Mode: mode, Gap: p.Gap.Duration(), Progressive: p.Progressive, Ladder: p.Ladder, Goal: p.Goal}
}

// New is an empty project.
func New() Project {
	return Project{Tracks: map[string]Track{}, Passages: []Passage{}, AddedFiles: []string{}}
}

// Clone is a copy of p that shares nothing with it, so it can be written
// while p goes on changing. Nil maps and lists stay nil.
func (p Project) Clone() Project {
	c := p
	if p.Tracks != nil {
		c.Tracks = make(map[string]Track, len(p.Tracks))
		for key, track := range p.Tracks {
			c.Tracks[key] = track.clone()
		}
	}
	if p.Passages != nil {
		c.Passages = make([]Passage, len(p.Passages))
		for i, passage := range p.Passages {
			passage.Plan = copied(passage.Plan)
			c.Passages[i] = passage
		}
	}
	c.AddedFiles = slices.Clone(p.AddedFiles)
	c.Last.Karaoke = copied(p.Last.Karaoke)
	return c
}

func (t Track) clone() Track {
	c := t
	c.Fit = copied(t.Fit)
	if t.Edits != nil {
		c.Edits = make(map[string]Edit, len(t.Edits))
		for id, edit := range t.Edits {
			edit.Text, edit.Start, edit.End = copied(edit.Text), copied(edit.Start), copied(edit.End)
			c.Edits[id] = edit
		}
	}
	return c
}

// copied is a pointer to a copy of what value points to; nil stays nil.
func copied[T any](value *T) *T {
	if value == nil {
		return nil
	}
	c := *value
	return &c
}

// RememberFile keeps a subtitle file picked for the video, once; it reports
// whether the list changed. With the list full, the oldest file that is no
// longer there (present says which are) gives way; with every file there,
// the new one is not kept.
func (p *Project) RememberFile(file string, present func(file string) bool) bool {
	if slices.Contains(p.AddedFiles, file) {
		return false
	}
	files := slices.Clone(p.AddedFiles)
	if len(files) >= MaxAddedFiles {
		gone := slices.IndexFunc(files, func(kept string) bool { return !present(kept) })
		if gone < 0 {
			return false
		}
		files = slices.Delete(files, gone, gone+1)
	}
	p.AddedFiles = append(files, file)
	return true
}

// Choice is what the user last picked for the video; empty keys and mode
// mean no choice.
type Choice struct {
	Audio       string     `json:"audio"`
	Primary     string     `json:"primary"`
	Translation string     `json:"translation"`
	Mode        study.Mode `json:"mode"`
}

// Track is what is kept for one subtitle track: its sync offset, the user's
// edits by event ID, and how a file was found to fit the video's voice.
type Track struct {
	Offset Millis          `json:"offset"`
	Edits  map[string]Edit `json:"edits"`
	Fit    *Fit            `json:"fit,omitempty"`
}

// Fit is how a subtitle file was found to line up with the video's voice
// (subtitle.AlignToVoice), kept so the video is not heard again for it the
// next time it opens. Of is FitOf the file's lines: a file changed since is
// heard anew.
type Fit struct {
	Of      string           `json:"of"`
	Verdict subtitle.Verdict `json:"verdict"`
	Offset  int64            `json:"offset"` // ms, either way
	Rate    float64          `json:"rate"`
	Score   float64          `json:"score"`
}

// The rates a fit may keep: the frame rates' ratios and a little drift.
const (
	minFitRate = 0.9
	maxFitRate = 1.1
)

// FitOf identifies a file's lines by their times, as HashOf does a text.
func FitOf(events []subtitle.Event) string {
	var times strings.Builder
	for _, event := range events {
		fmt.Fprintf(&times, "%d-%d;", event.Start.Milliseconds(), event.End.Milliseconds())
	}
	return HashOf(times.String())
}

// FitFrom keeps an alignment found by the voice for a file.
func FitFrom(alignment subtitle.Alignment, of string) Fit {
	return Fit{Of: of, Verdict: alignment.Verdict, Offset: alignment.Offset.Milliseconds(), Rate: alignment.Rate, Score: alignment.Score}
}

// Alignment is the fit as the analysis takes it, heard.
func (f Fit) Alignment() subtitle.Alignment {
	return subtitle.Alignment{
		Offset: time.Duration(f.Offset) * time.Millisecond, Rate: f.Rate, Score: f.Score, Verdict: f.Verdict, Heard: true,
	}
}

func (f Fit) validate() error {
	switch f.Verdict {
	case subtitle.VerdictMatch, subtitle.VerdictNoMatch, subtitle.VerdictOtherCut, subtitle.VerdictUnknown:
	default:
		return fmt.Errorf("%w: fit verdict %q", ErrInvalid, f.Verdict)
	}
	limit := subtitle.MaxOffset.Milliseconds()
	switch {
	case !hashPattern.MatchString(f.Of):
		return fmt.Errorf("%w: fit of %q", ErrInvalid, f.Of)
	case f.Offset < -limit || f.Offset > limit:
		return fmt.Errorf("%w: fit offset %d ms", ErrInvalid, f.Offset)
	// Written as "not within" so NaN is refused too.
	case !(f.Rate >= minFitRate && f.Rate <= maxFitRate), !(f.Score >= 0 && f.Score <= 1):
		return fmt.Errorf("%w: fit rate %v, score %v", ErrInvalid, f.Rate, f.Score)
	}
	return nil
}

// Edit is the user's change to one line. Original is HashOf the line's
// original text, so an edit never lands on another text when the file
// changes; nil fields keep the file's own value.
type Edit struct {
	Original string  `json:"original"`
	Text     *string `json:"text,omitempty"`
	Start    *Millis `json:"start,omitempty"`
	End      *Millis `json:"end,omitempty"`
}

// Passage is a passage saved for studying, on the playback clock.
// Its times are the ones studied, already fitted to the lines.
type Passage struct {
	Name  string `json:"name"`
	Start Millis `json:"start"`
	End   Millis `json:"end"`
	// Plan is how it is studied; nil (a passage saved before version 2)
	// pauses at the end, with no helps.
	Plan *PassagePlan `json:"plan,omitempty"`
}

// Segment is the passage as the study rules see it.
func (p Passage) Segment() study.Segment {
	return study.Segment{Start: p.Start.Duration(), End: p.End.Duration()}
}

// Practice is how the passage is studied.
func (p Passage) Practice() study.PracticePlan {
	if p.Plan == nil {
		return study.PracticePlan{Mode: study.EndPause}
	}
	return p.Plan.Practice()
}

// Outside tells the passage starts at or past the end of a video of the
// given duration (zero when unknown), or leaves less than a lap of it: a
// passage saved on another cut of the film.
func (p Passage) Outside(duration time.Duration) bool {
	return duration > 0 && p.Start.Duration() > duration-study.MinSegment
}

// IndexOf finds a passage by name, whatever the case and the spaces around
// it; -1 when there is none.
func IndexOf(passages []Passage, name string) int {
	name = strings.TrimSpace(name)
	for i, passage := range passages {
		if strings.EqualFold(passage.Name, name) {
			return i
		}
	}
	return -1
}

// UniqueName returns name, or name with " (2)", " (3)"… when a passage
// already has it, so saving the same span twice keeps both. It never goes
// past MaxPassageNameLen: the suffix takes the end of a long name.
func UniqueName(passages []Passage, name string) string {
	name = strings.TrimSpace(name)
	if IndexOf(passages, name) < 0 {
		return name
	}
	for n := 2; ; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		base := []rune(name)
		if room := MaxPassageNameLen - utf8.RuneCountInString(suffix); len(base) > room {
			base = base[:room]
		}
		candidate := strings.TrimSpace(string(base)) + suffix
		if IndexOf(passages, candidate) < 0 {
			return candidate
		}
	}
}

// HashOf identifies a line's original text: 16 hex digits of its SHA-256.
func HashOf(original string) string {
	sum := sha256.Sum256([]byte(original))
	return hex.EncodeToString(sum[:8])
}

// Apply puts the edit on a line of the file. It reports false, and leaves
// the line alone, when the line's text is not the one the edit was made on
// or when the edited line would not be valid.
func (e Edit) Apply(line subtitle.Event) (subtitle.Event, bool) {
	if e.Original != HashOf(line.OriginalText) {
		return line, false
	}
	edited := line
	if e.Text != nil {
		edited.OverrideText = *e.Text
	}
	if e.Start != nil {
		edited.Start = e.Start.Duration()
	}
	if e.End != nil {
		edited.End = e.End.Duration()
	}
	if !edited.Valid() {
		return line, false
	}
	return edited, true
}

// Diff is the edit that turns a line of the file into its edited form;
// false when they do not differ.
func Diff(original, edited subtitle.Event) (Edit, bool) {
	edit := Edit{Original: HashOf(original.OriginalText)}
	changed := false
	if text := edited.Text(); text != original.Text() {
		edit.Text, changed = &text, true
	}
	if edited.Start != original.Start {
		start := MillisOf(edited.Start)
		edit.Start, changed = &start, true
	}
	if edited.End != original.End {
		end := MillisOf(edited.End)
		edit.End, changed = &end, true
	}
	return edit, changed
}

// Apply puts the track's edits on the lines of the file, in their order. It
// returns how many edits did not apply: their line is gone, its text
// changed, or the edit would break it.
func (t Track) Apply(lines []subtitle.Event) ([]subtitle.Event, int) {
	edited := make([]subtitle.Event, len(lines))
	used := 0
	for i, line := range lines {
		edited[i] = line
		edit, found := t.Edits[line.ID]
		if !found {
			continue
		}
		if applied, ok := edit.Apply(line); ok {
			edited[i] = applied
			used++
		}
	}
	return edited, len(t.Edits) - used
}

// Validate checks every value against the limits.
func (p Project) Validate() error {
	if len(p.Tracks) > MaxTracks {
		return fmt.Errorf("%w: %d tracks", ErrInvalid, len(p.Tracks))
	}
	for key, track := range p.Tracks {
		if err := validKey(key); err != nil {
			return err
		}
		if err := track.validate(); err != nil {
			return fmt.Errorf("track %q: %w", key, err)
		}
	}
	for _, key := range []string{p.Choice.Audio, p.Choice.Primary, p.Choice.Translation} {
		if key != "" {
			if err := validKey(key); err != nil {
				return err
			}
		}
	}
	if p.Choice.Mode != "" && !p.Choice.Mode.Valid() {
		return fmt.Errorf("%w: mode %q", ErrInvalid, p.Choice.Mode)
	}
	if len(p.Passages) > MaxPassages {
		return fmt.Errorf("%w: %d passages", ErrInvalid, len(p.Passages))
	}
	for _, passage := range p.Passages {
		if err := passage.Validate(); err != nil {
			return err
		}
	}
	if len(p.AddedFiles) > MaxAddedFiles {
		return fmt.Errorf("%w: %d added files", ErrInvalid, len(p.AddedFiles))
	}
	for _, path := range p.AddedFiles {
		if err := validKey(path); err != nil {
			return fmt.Errorf("added file: %w", err)
		}
	}
	return p.Last.validate()
}

// validate checks how the video was left. A speed of zero is "the
// profile's"; any other must be a speed the app plays.
func (l Last) validate() error {
	switch {
	case !validTime(l.Position):
		return fmt.Errorf("%w: last position %d ms", ErrInvalid, l.Position)
	case l.Speed != 0 && !playback.Speed(l.Speed).Valid():
		return fmt.Errorf("%w: last speed %v", ErrInvalid, l.Speed)
	}
	if l.Karaoke != nil {
		return l.Karaoke.Validate()
	}
	return nil
}

// Validate checks the karaoke mode's values against the study rules.
func (k Karaoke) Validate() error {
	switch {
	case k.Rules().Validate() != nil:
		return fmt.Errorf("%w: karaoke timing %d/%d ms", ErrInvalid, k.MinGap, k.Lead)
	case !k.Indicator.Valid():
		return fmt.Errorf("%w: karaoke indicator %q", ErrInvalid, k.Indicator)
	case k.Repeat < 0 || k.Repeat > study.MaxRepeats:
		return fmt.Errorf("%w: karaoke repeat %d", ErrInvalid, k.Repeat)
	}
	return nil
}

// Validate checks a passage: a name and a span inside the time limit.
func (p Passage) Validate() error {
	name := strings.TrimSpace(p.Name)
	switch {
	case name == "" || name != p.Name || !utf8.ValidString(p.Name) || utf8.RuneCountInString(p.Name) > MaxPassageNameLen:
		return fmt.Errorf("%w: passage name %q", ErrInvalid, p.Name)
	case !validTime(p.Start) || !validTime(p.End) || p.End <= p.Start:
		return fmt.Errorf("%w: passage %q from %d to %d ms", ErrInvalid, p.Name, p.Start, p.End)
	case p.Plan != nil && p.Plan.Practice().Validate() != nil:
		return fmt.Errorf("%w: passage %q plan %+v", ErrInvalid, p.Name, *p.Plan)
	}
	return nil
}

func (t Track) validate() error {
	if offset := t.Offset.Duration(); offset < -subtitle.MaxOffset || offset > subtitle.MaxOffset {
		return fmt.Errorf("%w: offset %d ms", ErrInvalid, t.Offset)
	}
	if len(t.Edits) > MaxEditsPerTrack {
		return fmt.Errorf("%w: %d edits", ErrInvalid, len(t.Edits))
	}
	for id, edit := range t.Edits {
		if err := edit.validate(); err != nil {
			return fmt.Errorf("line %q: %w", id, err)
		}
	}
	if t.Fit != nil {
		return t.Fit.validate()
	}
	return nil
}

func (e Edit) validate() error {
	switch {
	case !hashPattern.MatchString(e.Original):
		return fmt.Errorf("%w: original %q", ErrInvalid, e.Original)
	case e.Text != nil && (strings.TrimSpace(*e.Text) == "" || !utf8.ValidString(*e.Text) || utf8.RuneCountInString(*e.Text) > MaxTextRunes):
		return fmt.Errorf("%w: text", ErrInvalid)
	case e.Start != nil && !validTime(*e.Start), e.End != nil && !validTime(*e.End):
		return fmt.Errorf("%w: time", ErrInvalid)
	case e.Start != nil && e.End != nil && *e.End <= *e.Start:
		return fmt.Errorf("%w: end before start", ErrInvalid)
	}
	return nil
}

func validKey(key string) error {
	if strings.TrimSpace(key) == "" || !utf8.ValidString(key) || utf8.RuneCountInString(key) > MaxKeyRunes {
		return fmt.Errorf("%w: track key %q", ErrInvalid, key)
	}
	return nil
}

func validTime(m Millis) bool {
	return m >= 0 && m.Duration() <= MaxTime
}
