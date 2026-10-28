package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Edit is the user's change to one line. Original is HashOf the line's
// original text, so an edit never lands on another text when the file
// changes; nil fields keep the file's own value.
type Edit struct {
	Original string  `json:"original"`
	Text     *string `json:"text,omitempty"`
	Start    *Millis `json:"start,omitempty"`
	End      *Millis `json:"end,omitempty"`
}

// hashPattern is the form of HashOf.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

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
