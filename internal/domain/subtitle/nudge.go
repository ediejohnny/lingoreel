package subtitle

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidNudge reports sync steps outside their ranges.
var ErrInvalidNudge = errors.New("invalid sync step")

// Ranges and defaults of the sync shortcut steps. The default step is a
// little over one frame at 23.976 fps; a 10 ms step is allowed but each
// press of it is hard to notice.
const (
	MinNudgeStep         = 10 * time.Millisecond
	MaxNudgeStep         = time.Second
	DefaultNudgeStep     = 50 * time.Millisecond
	MaxFastNudgeStep     = 5 * time.Second
	DefaultFastNudgeStep = 500 * time.Millisecond
	// FastNudgeAfter is how long a shortcut is held before the fast step.
	FastNudgeAfter = time.Second
)

// NudgeSteps are how far one press of a sync shortcut moves a subtitle line.
type NudgeSteps struct {
	// Step is the move of one press.
	Step time.Duration
	// FastStep is the move of each repeat once the shortcut was held for
	// FastNudgeAfter.
	FastStep time.Duration
}

// DefaultNudgeSteps returns 50 ms per press and 500 ms while held.
func DefaultNudgeSteps() NudgeSteps {
	return NudgeSteps{Step: DefaultNudgeStep, FastStep: DefaultFastNudgeStep}
}

// Validate checks that the step is within 10 ms to 1 s and the fast step
// between the step and 5 s.
func (s NudgeSteps) Validate() error {
	if s.Step < MinNudgeStep || s.Step > MaxNudgeStep {
		return fmt.Errorf("%w: step %v outside %v to %v", ErrInvalidNudge, s.Step, MinNudgeStep, MaxNudgeStep)
	}
	if s.FastStep < s.Step || s.FastStep > MaxFastNudgeStep {
		return fmt.Errorf("%w: fast step %v outside %v to %v", ErrInvalidNudge, s.FastStep, s.Step, MaxFastNudgeStep)
	}
	return nil
}

// Nudger turns the presses of one sync shortcut into offset moves: each
// press moves by Step; holding the shortcut repeats it, and after
// FastNudgeAfter every repeat moves by FastStep, to cross seconds quickly.
type Nudger struct {
	steps     NudgeSteps
	heldSince time.Time
}

// NewNudger creates a nudger; invalid steps fall back to the defaults.
func NewNudger(steps NudgeSteps) *Nudger {
	if steps.Validate() != nil {
		steps = DefaultNudgeSteps()
	}
	return &Nudger{steps: steps}
}

// Move returns how far one press moves the line. repeat tells that the key
// stayed down since the previous press, which the keyboard repeats.
func (n *Nudger) Move(now time.Time, repeat bool) time.Duration {
	if !repeat || n.heldSince.IsZero() {
		n.heldSince = now
		return n.steps.Step
	}
	if now.Sub(n.heldSince) >= FastNudgeAfter {
		return n.steps.FastStep
	}
	return n.steps.Step
}
