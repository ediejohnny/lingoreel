//go:build windows && amd64

package playerwindow

import (
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/controlbar"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/shortcut"
	"lingoreel/internal/domain/study"
)

// choose carries out a menu entry through the session, reading the value
// it changes when it runs, and tells whether the menu stays open: options
// and sync steps keep it, so several can be set in a row.
func (w *Window) choose(entry controlbar.Entry) (keepOpen bool) {
	switch entry.Kind {
	case controlbar.EntryCommand:
		command := entry.Command
		w.do(func(r watch.Remote) error { return r.Run(command, false) })
		return isSync(command)
	case controlbar.EntryToggle:
		field := entry.Field
		w.do(func(r watch.Remote) error { return toggle(r, field) })
		return true
	case controlbar.EntryChoice:
		field, value := entry.Field, entry.Value
		w.do(func(r watch.Remote) error { return pick(r, field, value) })
		return true
	case controlbar.EntryStepper:
		field, delta := entry.Field, entry.Delta
		w.do(func(r watch.Remote) error { return step(r, field, delta) })
		return true
	case controlbar.EntryPassage:
		index := entry.Index
		w.do(func(r watch.Remote) error { return r.StudySavedAt(index) })
		return false
	case controlbar.EntryAction:
		line := session.LinePrimary
		if entry.Value == "translation" {
			line = session.LineTranslation
		}
		switch entry.Field {
		case controlbar.FieldResetSync:
			w.do(func(r watch.Remote) error {
				r.ResetSync(session.LinePrimary)
				r.ResetSync(session.LineTranslation)
				return nil
			})
			return true
		case controlbar.FieldEdit:
			w.do(func(r watch.Remote) error { return r.OpenEditor(line) })
			return false
		}
	}
	return true
}

func isSync(command shortcut.Command) bool {
	switch command {
	case shortcut.SyncPrimaryEarlier, shortcut.SyncPrimaryLater, shortcut.SyncTranslationEarlier, shortcut.SyncTranslationLater:
		return true
	}
	return false
}

// toggle turns an option on or off.
func toggle(r watch.Remote, field controlbar.Field) error {
	switch field {
	case controlbar.FieldSnap, controlbar.FieldLoop, controlbar.FieldProgressive, controlbar.FieldLadder:
		plan, snap := r.PassageOptions()
		switch field {
		case controlbar.FieldSnap:
			snap = !snap
		case controlbar.FieldLoop:
			if plan.Mode == study.EndLoop {
				plan.Mode = study.EndPause
			} else {
				plan.Mode = study.EndLoop
			}
		case controlbar.FieldProgressive:
			plan.Progressive = !plan.Progressive
		case controlbar.FieldLadder:
			plan.Ladder = !plan.Ladder
		}
		return r.SetPassageOptions(withMode(plan), snap)
	case controlbar.FieldKaraoke, controlbar.FieldFill:
		karaoke := r.Karaoke()
		if field == controlbar.FieldKaraoke {
			karaoke.Enabled = !karaoke.Enabled
		} else {
			karaoke.Fill = !karaoke.Fill
		}
		return r.SetKaraokeSettings(karaoke)
	case controlbar.FieldTranslation:
		r.SetTranslationVisible(!r.TranslationVisible())
	}
	return nil
}

// pick chooses a value among the entries of a field.
func pick(r watch.Remote, field controlbar.Field, value string) error {
	switch field {
	case controlbar.FieldIndicator:
		return r.SetIndicator(study.Indicator(value))
	case controlbar.FieldMode:
		return r.SetMode(study.Mode(value))
	}
	return nil
}

// step moves a stepper's value one step, kept inside its limits.
func step(r watch.Remote, field controlbar.Field, delta int) error {
	duration, count := controlbar.Step(field)
	by := time.Duration(delta) * duration
	switch field {
	case controlbar.FieldGap, controlbar.FieldGoal:
		plan, snap := r.PassageOptions()
		if field == controlbar.FieldGap {
			plan.Gap = min(max(plan.Gap+by, 0), study.MaxLoopGap)
		} else {
			plan.Goal = min(max(plan.Goal+delta*count, 0), study.MaxGoal)
		}
		return r.SetPassageOptions(withMode(plan), snap)
	case controlbar.FieldMinGap, controlbar.FieldLead:
		karaoke := r.Karaoke()
		if field == controlbar.FieldMinGap {
			karaoke.Rules.MinGap = min(max(karaoke.Rules.MinGap+by, study.MinCueGap), study.MaxCueGap)
		} else {
			karaoke.Rules.Lead = min(max(karaoke.Rules.Lead+by, study.MinCueLead), study.MaxCueLead)
		}
		return r.SetKaraokeSettings(karaoke)
	case controlbar.FieldRepeat:
		return r.SetLineRepeats(min(max(r.LineRepeats()+delta*count, 0), study.MaxRepeats))
	}
	return nil
}

// withMode gives a plan without an end mode the one that pauses.
func withMode(plan study.PracticePlan) study.PracticePlan {
	if plan.Mode == "" {
		plan.Mode = study.EndPause
	}
	return plan
}

// studyState is what the menus and the study text show, from the session.
func studyState(r watch.Remote, sample playback.Sample) controlbar.StudyState {
	start, marked := r.MarkedStart()
	passage := r.Passage()
	plan, snap := r.PassageOptions()
	karaoke := r.Karaoke()
	state := controlbar.StudyState{
		MarkSet: marked, MarkStart: start,
		Active: passage.Active, Suspended: passage.Suspended, AtEnd: passage.AtEnd,
		Passage: passage.Segment, Lap: passage.Lap, Goal: passage.Plan.Goal,
		Snap: snap, Options: withMode(plan),
		Karaoke: karaoke.Enabled, Indicator: r.Indicator(), MinGap: karaoke.Rules.MinGap, Lead: karaoke.Rules.Lead,
		Fill: karaoke.Fill, Repeat: r.LineRepeats(),
		Mode: r.Mode(), HasPrimary: r.HasPrimary(), HasTranslation: r.HasTranslation(), TranslationOn: r.TranslationVisible(),
		Paused: sample.Status != playback.StatusPlaying,
	}
	for _, saved := range r.SavedPassages() {
		state.Saved = append(state.Saved, controlbar.SavedItem{
			Start: saved.Segment.Start, End: saved.Segment.End,
			Outside: sample.Duration > 0 && saved.Segment.Start > sample.Duration-study.MinSegment,
		})
	}
	return state
}
