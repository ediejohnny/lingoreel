// Package project models what the user keeps about one video: the
// corrections to its subtitles, how each track is synchronized, the choices
// last made for it and the passages saved for study. It lives beside the
// video and never changes the video itself.
package project

import (
	"errors"
	"fmt"
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
// subtitle files picked from elsewhere (AddedFiles); version 4 what was heard
// in an audio track the file did not name (Track.Hearing).
const Version = 4

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
	if t.Hearing != nil {
		hearing := *t.Hearing
		hearing.Scores = slices.Clone(t.Hearing.Scores)
		c.Hearing = &hearing
	}
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

// Track is what is kept for one track: a subtitle's sync offset, the user's
// edits by event ID and how a file was found to fit the video's voice; an
// audio track's language as heard.
type Track struct {
	Offset  Millis          `json:"offset"`
	Edits   map[string]Edit `json:"edits"`
	Fit     *Fit            `json:"fit,omitempty"`
	Hearing *Hearing        `json:"hearing,omitempty"`
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
		if err := t.Fit.validate(); err != nil {
			return err
		}
	}
	if t.Hearing != nil {
		return t.Hearing.validate()
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
