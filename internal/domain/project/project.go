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
	"strings"
	"time"
	"unicode/utf8"

	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
)

// Version is the current version of the project file.
const Version = 1

// Limits of a project read from disk, which is not trusted: a hand-edited or
// hostile file cannot make the app hold more than this.
const (
	MaxTracks         = 64
	MaxEditsPerTrack  = 100_000
	MaxTextRunes      = 2000
	MaxKeyRunes       = 512
	MaxPassages       = 500
	MaxPassageNameLen = 80
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
}

// New is an empty project.
func New() Project {
	return Project{Tracks: map[string]Track{}, Passages: []Passage{}}
}

// Choice is what the user last picked for the video; empty keys and mode
// mean no choice.
type Choice struct {
	Audio       string     `json:"audio"`
	Primary     string     `json:"primary"`
	Translation string     `json:"translation"`
	Mode        study.Mode `json:"mode"`
}

// Track is what is kept for one subtitle track: its sync offset and the
// user's edits by event ID.
type Track struct {
	Offset Millis          `json:"offset"`
	Edits  map[string]Edit `json:"edits"`
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

// Passage is a passage saved for "Estudar trecho", on the playback clock.
type Passage struct {
	Name  string `json:"name"`
	Start Millis `json:"start"`
	End   Millis `json:"end"`
}

// Segment is the passage as the study rules see it.
func (p Passage) Segment() study.Segment {
	return study.Segment{Start: p.Start.Duration(), End: p.End.Duration()}
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
