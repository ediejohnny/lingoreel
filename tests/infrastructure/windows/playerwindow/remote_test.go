package playerwindow_test

import (
	"fmt"
	"slices"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/study"
)

// The fake session's side of the bar's menus: what they read, and a record
// of what they change.

func (r *fakeRemote) record(change string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.changes = append(r.changes, change)
}

func (r *fakeRemote) changed() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.changes)
}

func (r *fakeRemote) MarkedStart() (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markStart, r.marked
}

func (r *fakeRemote) Passage() session.PracticeStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.passage
}

func (r *fakeRemote) PassageOptions() (study.PracticePlan, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.plan, r.snap
}

func (r *fakeRemote) SetPassageOptions(plan study.PracticePlan, snap bool) error {
	r.mu.Lock()
	r.plan, r.snap = plan, snap
	r.mu.Unlock()
	r.record(fmt.Sprintf("options %s gap=%v goal=%d progressive=%v ladder=%v snap=%v", plan.Mode, plan.Gap, plan.Goal, plan.Progressive, plan.Ladder, snap))
	return nil
}

func (r *fakeRemote) SavedPassages() []watch.SavedPassage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.saved)
}

func (r *fakeRemote) StudySavedAt(index int) error {
	r.record(fmt.Sprintf("study saved %d", index))
	return nil
}

func (r *fakeRemote) Karaoke() session.Karaoke {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.karaoke
}

func (r *fakeRemote) SetKaraokeSettings(karaoke session.Karaoke) error {
	if err := karaoke.Rules.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	r.karaoke = karaoke
	r.mu.Unlock()
	r.record(fmt.Sprintf("karaoke on=%v gap=%v lead=%v fill=%v", karaoke.Enabled, karaoke.Rules.MinGap, karaoke.Rules.Lead, karaoke.Fill))
	return nil
}

func (r *fakeRemote) Indicator() study.Indicator {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indicator
}

func (r *fakeRemote) SetIndicator(indicator study.Indicator) error {
	r.mu.Lock()
	r.indicator = indicator
	r.mu.Unlock()
	r.record("indicator " + string(indicator))
	return nil
}

func (r *fakeRemote) LineRepeats() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.repeats
}

func (r *fakeRemote) SetLineRepeats(times int) error {
	r.mu.Lock()
	r.repeats = times
	r.mu.Unlock()
	r.record(fmt.Sprintf("repeats %d", times))
	return nil
}

func (r *fakeRemote) SetMode(mode study.Mode) error {
	r.mu.Lock()
	r.mode = mode
	r.mu.Unlock()
	r.record("mode " + string(mode))
	return nil
}

func (r *fakeRemote) HasPrimary() bool { return true }

func (r *fakeRemote) ResetSync(line session.Line) {
	r.record(fmt.Sprintf("reset sync %d", line))
}

func (r *fakeRemote) OpenEditor(line session.Line) error {
	r.record(fmt.Sprintf("editor %d", line))
	return nil
}
