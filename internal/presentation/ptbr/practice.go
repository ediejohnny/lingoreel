package ptbr

import (
	"fmt"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// ModeName names a study mode as the screens call it.
func ModeName(mode study.Mode) string {
	switch mode {
	case study.ModeDual:
		return T("mode.dual")
	case study.ModeOnDemand:
		return T("mode.onDemand")
	case study.ModeAlwaysTranslated:
		return T("mode.alwaysTranslated")
	}
	return string(mode)
}

// PassageClock shows a passage's time as h:mm:ss or m:ss, with tenths when
// it is not a whole second ("1:10:23,4"), which ParseClock reads back.
func PassageClock(d time.Duration) string {
	d = max(d, 0).Round(100 * time.Millisecond)
	whole := Clock(d)
	if tenths := (d % time.Second) / (100 * time.Millisecond); tenths > 0 {
		return fmt.Sprintf("%s%s%d", whole, T("number.decimal"), tenths)
	}
	return whole
}

// PassageName is the name a passage gets when it is saved by its shortcut,
// "Trecho 10:38–11:30": the user knows it in the list without opening it,
// and renames it at will. It is in whole seconds: fitting a passage to the
// lines moves its start half a second back, which a name needs not show.
func PassageName(start, end time.Duration) string {
	return T("passage.name", "start", Clock(start.Round(time.Second)), "end", Clock(end.Round(time.Second)))
}

// PassageMarkNotice words the notices of the passage shortcuts.
func PassageMarkNotice(start time.Duration) string {
	return T("notice.passage.marked", "time", PassageClock(start))
}

// PassageSavedNotice tells a passage was saved under a name.
func PassageSavedNotice(name string) string {
	return T("notice.passage.saved", "name", name)
}

// PlanSummary says how a passage is studied, its parts joined by dots: a
// pause at the end, or a loop with its gap, the progressive speed, the
// subtitle ladder and the goal of laps that apply.
func PlanSummary(plan study.PracticePlan) string {
	parts := []string{T("plan.pause")}
	if plan.Mode == study.EndLoop {
		parts = []string{T("plan.loop")}
		if plan.Gap > 0 {
			parts = append(parts, T("plan.gap", "gap", Seconds(plan.Gap)))
		}
	}
	if plan.Progressive {
		parts = append(parts, T("plan.progressive"))
	}
	if plan.Ladder {
		parts = append(parts, T("plan.ladder"))
	}
	if plan.Goal > 0 {
		parts = append(parts, T("plan.goal", "count", plan.Goal))
	}
	return strings.Join(parts, " · ")
}

// Seconds says a short time in seconds the Brazilian way: "2 s", "1,5 s".
func Seconds(d time.Duration) string {
	return T("unit.seconds", "value", Number(d.Seconds(), -1))
}

// MarkNotice words what a passage mark did for the notice on the video.
func MarkNotice(event watch.MarkEvent, segment study.Segment) string {
	switch event {
	case watch.MarkStarted:
		return PassageMarkNotice(segment.Start)
	case watch.MarkNoStart:
		return T("notice.mark.noStart")
	case watch.MarkTooShort:
		return T("notice.mark.tooShort")
	}
	return ""
}

// EditClock shows a line's time to the millisecond for the editor,
// "1:02:03,456" or "2:03,456", which ParseClock reads back.
func EditClock(d time.Duration) string {
	d = max(d, 0).Round(time.Millisecond)
	return fmt.Sprintf("%s%s%03d", Clock(d.Truncate(time.Second)), T("number.decimal"), d%time.Second/time.Millisecond)
}

// PracticeNotice words what happens to a passage for the notice on the
// video, such as "Passage 1:10:23–1:11:04 · repetition 3".
func PracticeNotice(event session.PracticeEvent, status session.PracticeStatus) string {
	span := PassageClock(status.Segment.Start) + "–" + PassageClock(status.Segment.End)
	switch event {
	case session.PracticeLap:
		if status.Plan.Goal > 0 {
			return T("notice.practice.lapOfGoal", "span", span, "lap", status.Lap, "goal", status.Plan.Goal)
		}
		return T("notice.practice.lap", "span", span, "lap", status.Lap)
	case session.PracticeAtEnd:
		return T("notice.practice.atEnd")
	case session.PracticeSuspended:
		return T("notice.practice.suspended")
	case session.PracticeFinished:
		return T("notice.practice.finished", "count", status.Plan.Goal)
	case session.PracticeFailed:
		return T("notice.practice.failed")
	}
	return ""
}
