package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Fit is how a subtitle file was found to line up with the video's voice
// (subtitle.AlignToVoice), kept so the video is not heard again for it the
// next time it opens. Of is FitOf the file's lines: a file changed since is
// heard anew.
type Fit struct {
	Of      string           `json:"of"`
	Verdict subtitle.Verdict `json:"verdict"`
	Offset  int64            `json:"offset"` // ms, either way
	Rate    float64          `json:"rate"`
	Score   float64          `json:"score"`
}

// The rates a fit may keep: the frame rates' ratios and a little drift.
const (
	minFitRate = 0.9
	maxFitRate = 1.1
)

// FitOf identifies a file's lines by their times, as HashOf does a text.
func FitOf(events []subtitle.Event) string {
	var times strings.Builder
	for _, event := range events {
		fmt.Fprintf(&times, "%d-%d;", event.Start.Milliseconds(), event.End.Milliseconds())
	}
	return HashOf(times.String())
}

// FitFrom keeps an alignment found by the voice for a file.
func FitFrom(alignment subtitle.Alignment, of string) Fit {
	return Fit{Of: of, Verdict: alignment.Verdict, Offset: alignment.Offset.Milliseconds(), Rate: alignment.Rate, Score: alignment.Score}
}

// Alignment is the fit as the analysis takes it, heard.
func (f Fit) Alignment() subtitle.Alignment {
	return subtitle.Alignment{
		Offset: time.Duration(f.Offset) * time.Millisecond, Rate: f.Rate, Score: f.Score, Verdict: f.Verdict, Heard: true,
	}
}

func (f Fit) validate() error {
	switch f.Verdict {
	case subtitle.VerdictMatch, subtitle.VerdictNoMatch, subtitle.VerdictOtherCut, subtitle.VerdictUnknown:
	default:
		return fmt.Errorf("%w: fit verdict %q", ErrInvalid, f.Verdict)
	}
	limit := subtitle.MaxOffset.Milliseconds()
	switch {
	case !hashPattern.MatchString(f.Of):
		return fmt.Errorf("%w: fit of %q", ErrInvalid, f.Of)
	case f.Offset < -limit || f.Offset > limit:
		return fmt.Errorf("%w: fit offset %d ms", ErrInvalid, f.Offset)
	// Written as "not within" so NaN is refused too.
	case !(f.Rate >= minFitRate && f.Rate <= maxFitRate), !(f.Score >= 0 && f.Score <= 1):
		return fmt.Errorf("%w: fit rate %v, score %v", ErrInvalid, f.Rate, f.Score)
	}
	return nil
}
