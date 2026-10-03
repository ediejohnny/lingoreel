package ptbr

import (
	"fmt"
	"time"

	"lingoreel/internal/application/session"
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
