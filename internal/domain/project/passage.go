package project

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

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
