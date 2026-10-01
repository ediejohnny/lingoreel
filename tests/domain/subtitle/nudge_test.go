package subtitle_test

import (
	"errors"
	"testing"
	"time"

	"lingoreel/internal/domain/subtitle"
)

func TestNudgeStepsRanges(t *testing.T) {
	cases := []struct {
		steps subtitle.NudgeSteps
		ok    bool
	}{
		{subtitle.DefaultNudgeSteps(), true},
		{subtitle.NudgeSteps{Step: 10 * time.Millisecond, FastStep: 10 * time.Millisecond}, true},
		{subtitle.NudgeSteps{Step: time.Second, FastStep: 5 * time.Second}, true},
		{subtitle.NudgeSteps{Step: 9 * time.Millisecond, FastStep: time.Second}, false},
		{subtitle.NudgeSteps{Step: time.Second + 1, FastStep: 2 * time.Second}, false},
		{subtitle.NudgeSteps{Step: 100 * time.Millisecond, FastStep: 99 * time.Millisecond}, false},
		{subtitle.NudgeSteps{Step: 100 * time.Millisecond, FastStep: 5*time.Second + 1}, false},
		{subtitle.NudgeSteps{Step: -50 * time.Millisecond, FastStep: time.Second}, false},
		{subtitle.NudgeSteps{}, false},
	}
	for _, c := range cases {
		err := c.steps.Validate()
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, subtitle.ErrInvalidNudge)) {
			t.Errorf("Validate(%+v) = %v, want ok %v", c.steps, err, c.ok)
		}
	}
	if got := subtitle.DefaultNudgeSteps(); got.Step != 50*time.Millisecond || got.FastStep != 500*time.Millisecond {
		t.Fatalf("defaults = %+v, want 50 ms and 500 ms", got)
	}
}

func TestNudgerFallsBackToTheDefaults(t *testing.T) {
	n := subtitle.NewNudger(subtitle.NudgeSteps{Step: time.Hour})
	if got := n.Move(time.Unix(0, 0), false); got != subtitle.DefaultNudgeStep {
		t.Fatalf("move with invalid steps = %v, want the default %v", got, subtitle.DefaultNudgeStep)
	}
}

func TestNudgerFastStepStartsAtOneSecondHeld(t *testing.T) {
	start := time.Unix(100, 0)
	n := subtitle.NewNudger(subtitle.DefaultNudgeSteps())
	if got := n.Move(start, false); got != 50*time.Millisecond {
		t.Fatalf("first press = %v", got)
	}
	if got := n.Move(start.Add(time.Second-time.Nanosecond), true); got != 50*time.Millisecond {
		t.Fatalf("held 1 ns short of 1 s = %v, want the small step", got)
	}
	if got := n.Move(start.Add(time.Second), true); got != 500*time.Millisecond {
		t.Fatalf("held 1 s = %v, want the fast step", got)
	}
	// A repeat with no press before it counts as a new press.
	fresh := subtitle.NewNudger(subtitle.DefaultNudgeSteps())
	if got := fresh.Move(start, true); got != 50*time.Millisecond {
		t.Fatalf("repeat without a press = %v", got)
	}
	if got := fresh.Move(start.Add(2*time.Second), true); got != 500*time.Millisecond {
		t.Fatalf("held since that repeat = %v, want the fast step", got)
	}
}
