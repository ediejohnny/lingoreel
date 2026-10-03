package ptbr

import (
	"fmt"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/application/watch"
	"lingoreel/internal/domain/study"
)

// ModeName names a study mode, as RF-026 and RF-029 call them.
func ModeName(mode study.Mode) string {
	switch mode {
	case study.ModeDual:
		return "Dupla"
	case study.ModeOnDemand:
		return "Tradução sob demanda"
	case study.ModeAlwaysTranslated:
		return "Tradução sempre visível"
	}
	return string(mode)
}

// PassageClock shows a passage's time as h:mm:ss or m:ss, with tenths when
// it is not a whole second ("1:10:23,4"), which ParseClock reads back.
func PassageClock(d time.Duration) string {
	d = max(d, 0).Round(100 * time.Millisecond)
	whole := Clock(d)
	if tenths := (d % time.Second) / (100 * time.Millisecond); tenths > 0 {
		return fmt.Sprintf("%s,%d", whole, tenths)
	}
	return whole
}

// PassageName is the name a passage gets when it is saved by its shortcut,
// "Trecho 10:38–11:30": the user knows it in the list without opening it,
// and renames it at will. It is in whole seconds: fitting a passage to the
// lines moves its start half a second back, which a name needs not show.
func PassageName(start, end time.Duration) string {
	return "Trecho " + Clock(start.Round(time.Second)) + "–" + Clock(end.Round(time.Second))
}

// PassageMarkNotice words the notices of the passage shortcuts.
func PassageMarkNotice(start time.Duration) string {
	return "Início do trecho: " + PassageClock(start)
}

// PassageSavedNotice tells a passage was saved under a name.
func PassageSavedNotice(name string) string {
	return "Salvo: " + name
}

// MarkNotice words what a passage mark did for the notice on the video.
func MarkNotice(event watch.MarkEvent, segment study.Segment) string {
	switch event {
	case watch.MarkStarted:
		return PassageMarkNotice(segment.Start)
	case watch.MarkNoStart:
		return "Marque o início do trecho primeiro"
	case watch.MarkTooShort:
		return "O fim precisa vir pelo menos 1 s depois do início"
	}
	return ""
}

// EditClock shows a line's time to the millisecond for the editor,
// "1:02:03,456" or "2:03,456", which ParseClock reads back.
func EditClock(d time.Duration) string {
	d = max(d, 0).Round(time.Millisecond)
	return fmt.Sprintf("%s,%03d", Clock(d.Truncate(time.Second)), d%time.Second/time.Millisecond)
}

// PracticeNotice words what happens to a passage for the notice on the
// video: "Trecho 1:10:23–1:11:04 · repetição 3".
func PracticeNotice(event session.PracticeEvent, status session.PracticeStatus) string {
	span := PassageClock(status.Segment.Start) + "–" + PassageClock(status.Segment.End)
	switch event {
	case session.PracticeLap:
		text := fmt.Sprintf("Trecho %s · repetição %d", span, status.Lap)
		if status.Plan.Goal > 0 {
			text = fmt.Sprintf("Trecho %s · repetição %d de %d", span, status.Lap, status.Plan.Goal)
		}
		return text
	case session.PracticeAtEnd:
		return "Fim do trecho · Reproduzir para repetir"
	case session.PracticeSuspended:
		return "Trecho pausado — fora do intervalo"
	case session.PracticeFinished:
		return fmt.Sprintf("Meta de %d repetições concluída", status.Plan.Goal)
	}
	return ""
}
