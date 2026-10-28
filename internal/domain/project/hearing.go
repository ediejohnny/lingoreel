package project

import (
	"fmt"
	"regexp"
	"slices"
)

// Limits of a hearing kept: how many languages, and how many lines each.
const (
	MaxHeardLanguages = 8
	maxHeardLines     = 1000
)

// heardLanguage is the form of a language tried: its ISO 639 code.
var heardLanguage = regexp.MustCompile(`^[a-z]{2,3}$`)

// Hearing is what was heard in an audio track the file did not name: how
// sure each language tried was of its lines in it, and the language the
// track speaks, empty when its sound did not tell. A language tried later is
// heard and weighed against the scores kept.
type Hearing struct {
	Scores   []HeardScore `json:"scores"`
	Language string       `json:"language,omitempty"`
}

// HeardScore is how sure a language's model was of that language's lines in
// the track: the median sureness, from 0 to 1, over Lines lines.
type HeardScore struct {
	Language string  `json:"language"`
	Score    float64 `json:"score"`
	Lines    int     `json:"lines"`
}

// Covers tells every language given was tried already.
func (h *Hearing) Covers(languages []string) bool {
	if h == nil {
		return false
	}
	for _, language := range languages {
		if !slices.ContainsFunc(h.Scores, func(s HeardScore) bool { return s.Language == language }) {
			return false
		}
	}
	return true
}

func (h Hearing) validate() error {
	if len(h.Scores) > MaxHeardLanguages {
		return fmt.Errorf("%w: %d languages heard", ErrInvalid, len(h.Scores))
	}
	tried := false
	for _, s := range h.Scores {
		if !heardLanguage.MatchString(s.Language) || !(s.Score >= 0 && s.Score <= 1) || s.Lines < 0 || s.Lines > maxHeardLines {
			return fmt.Errorf("%w: language heard %+v", ErrInvalid, s)
		}
		tried = tried || s.Language == h.Language
	}
	if h.Language != "" && !tried {
		return fmt.Errorf("%w: language %q heard but not tried", ErrInvalid, h.Language)
	}
	return nil
}
