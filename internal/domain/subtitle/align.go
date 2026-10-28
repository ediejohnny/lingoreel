package subtitle

import (
	"math"
	"slices"
	"sort"
	"time"
)

// Verdict tells how a subtitle file fits a video, compared with a reference
// track of that video.
type Verdict string

const (
	// VerdictMatch: the file belongs to this cut of the video; Offset and
	// Rate line it up.
	VerdictMatch Verdict = "match"
	// VerdictOtherCut: parts of the file line up with different offsets, or
	// it runs past the end of the video: it was timed for another cut
	// (extended, director's cut) and drifts away scene after scene.
	VerdictOtherCut Verdict = "other-cut"
	// VerdictNoMatch: the lines do not follow the reference at all.
	VerdictNoMatch Verdict = "no-match"
	// VerdictUnknown: too few lines on either side to tell.
	VerdictUnknown Verdict = "unknown"
)

// Alignment is how a subtitle file lines up with a reference track: a
// candidate instant t shows at t*Rate + Offset.
type Alignment struct {
	Offset time.Duration
	// Rate corrects a file timed for another frame rate (25 against 23.976
	// fps); 1 when both run at the same speed.
	Rate float64
	// Score is the share of the file's lines that start close to a line of
	// the reference once aligned, from 0 to 1.
	Score   float64
	Verdict Verdict
	// Heard tells it was found by the voice heard in the video
	// (AlignToVoice), not by a subtitle of the video.
	Heard bool
}

const (
	// minAlignLines is the fewest lines either side needs: below it a
	// histogram peak is chance.
	minAlignLines = 20
	// maxAlignLines bounds the work on hostile files: longer lists are
	// sampled evenly, which keeps the peak where it is. Films have 1000 to
	// 1500 lines.
	maxAlignLines = 2000
	// alignWindow is the largest offset searched: a different opening, or
	// the minutes of scenes an extended cut adds before a part of the film.
	alignWindow = 5 * time.Minute
	// alignBucket is the histogram resolution, a little over a frame.
	alignBucket = 50 * time.Millisecond
	// matchTolerance is how close two starts must be to count as the same
	// line: translations of one release differ by a few frames.
	matchTolerance = 300 * time.Millisecond
	// matchScore is the least share of lined-up lines for a match.
	matchScore = 0.6
	// rateMargin is how much a frame-rate correction must beat the plain
	// alignment before it is believed.
	rateMargin = 0.1
	// cutParts splits the file in parts by line count: in a match every part
	// lines up; in another cut parts line up each with its own offset.
	cutParts = 4
	// partScore is the least share of lined-up lines in one part.
	partScore = 0.6
	// pastEndTolerance allows credits lines a little after the video ends.
	pastEndTolerance = 10 * time.Second
	// maxFitRateChange bounds the drift the line fit may correct: 0.1% is
	// about 5 s over a 90-minute film; more is a frame-rate change, already
	// tried, or a different cut.
	maxFitRateChange = 0.001
	// maxAlignInstant is later than any line of any film; instants past it
	// are ignored.
	maxAlignInstant = 100 * time.Hour
)

// frameRateRatios are the speed changes between the common frame rates
// (23.976, 24 and 25 fps), as reference time over file time.
var frameRateRatios = []float64{
	25 / (24000.0 / 1001), (24000.0 / 1001) / 25,
	25.0 / 24, 24.0 / 25,
	24 / (24000.0 / 1001), (24000.0 / 1001) / 24,
}

// Starts returns the sorted start instants of events.
func Starts(events []Event) []time.Duration {
	starts := make([]time.Duration, len(events))
	for i, event := range events {
		starts[i] = event.Start
	}
	slices.Sort(starts)
	return starts
}

// Align lines up a subtitle file with a reference track of the same video,
// both given as start instants. The reference may be in another language:
// the rhythm of the lines is the same. videoDuration, when known, catches
// files that run past the end of this cut.
func Align(reference, candidate []time.Duration, videoDuration time.Duration) Alignment {
	reference, candidate = usable(reference), usable(candidate)
	unknown := Alignment{Rate: 1, Verdict: VerdictUnknown}
	if len(reference) < minAlignLines || len(candidate) < minAlignLines {
		if runsPastEnd(candidate, Alignment{Rate: 1}, videoDuration) {
			unknown.Verdict = VerdictOtherCut
		}
		return unknown
	}
	reference = sample(reference)
	candidate = sample(candidate)

	best := alignAt(reference, candidate, 1)
	for _, rate := range frameRateRatios {
		if scaled := alignAt(reference, candidate, rate); scaled.Score >= best.Score+rateMargin {
			best = scaled
		}
	}
	if fitted, ok := fitLine(reference, candidate, best); ok && fitted.Score >= best.Score {
		best = fitted
	}
	switch {
	case runsPastEnd(candidate, best, videoDuration):
		best.Verdict = VerdictOtherCut
	case best.Score >= matchScore && everyPartFits(reference, candidate, best):
		best.Verdict = VerdictMatch
	case partsLineUp(reference, candidate) || runsPastEnd(candidate, Alignment{Rate: 1}, videoDuration):
		// Not lined up, but timed longer than this video: the extended cut
		// whose added scenes defeat any single alignment.
		best.Verdict = VerdictOtherCut
	default:
		best.Verdict = VerdictNoMatch
	}
	return best
}

// alignAt finds the best offset for one rate: the peak of the histogram of
// differences between reference and file starts, refined to the median of
// the differences around it.
func alignAt(reference, candidate []time.Duration, rate float64) Alignment {
	scaled := scale(candidate, rate)
	buckets := int(alignWindow / alignBucket)
	histogram := make([]int, 2*buckets+1)
	for _, start := range scaled {
		first := sort.Search(len(reference), func(i int) bool { return reference[i] >= start-alignWindow })
		for _, ref := range reference[first:] {
			difference := ref - start
			if difference > alignWindow {
				break
			}
			if index := bucketOf(difference) + buckets; index >= 0 && index < len(histogram) {
				histogram[index]++
			}
		}
	}
	// Sum three neighbouring buckets so a peak split by a bucket edge still wins.
	peak, peakCount := 0, -1
	for i := 1; i < len(histogram)-1; i++ {
		if count := histogram[i-1] + histogram[i] + histogram[i+1]; count > peakCount {
			peak, peakCount = i, count
		}
	}
	coarse := time.Duration(peak-buckets) * alignBucket

	var differences []time.Duration
	for _, start := range scaled {
		if difference, found := nearest(reference, start+coarse); found && difference.Abs() <= matchTolerance {
			differences = append(differences, difference+coarse)
		}
	}
	offset := coarse
	if len(differences) > 0 {
		slices.Sort(differences)
		offset = differences[len(differences)/2]
	}
	return Alignment{Offset: offset, Rate: rate, Score: score(reference, scaled, offset)}
}

// fitLine refines an alignment with a least-squares line through the lines it
// matched, which removes a slow drift (a file retimed from a source a hair
// faster or slower) that a single offset leaves growing along the film.
func fitLine(reference, candidate []time.Duration, alignment Alignment) (Alignment, bool) {
	var n, sumX, sumY, sumXX, sumXY float64
	for _, start := range candidate {
		shifted := time.Duration(float64(start)*alignment.Rate) + alignment.Offset
		difference, found := nearest(reference, shifted)
		if !found || difference.Abs() > matchTolerance {
			continue
		}
		x, y := start.Seconds(), (shifted + difference).Seconds()
		n++
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
	}
	denominator := n*sumXX - sumX*sumX
	if n < minAlignLines || denominator <= 0 {
		return Alignment{}, false
	}
	rate := (n*sumXY - sumX*sumY) / denominator
	offset := (sumY - rate*sumX) / n
	// Written as "not within" so a NaN rate is refused too.
	if !(math.Abs(rate-alignment.Rate) <= maxFitRateChange) {
		return Alignment{}, false
	}
	fitted := Alignment{Rate: rate, Offset: time.Duration(offset * float64(time.Second))}
	fitted.Score = score(reference, scale(candidate, rate), fitted.Offset)
	return fitted, true
}

// scale multiplies every instant by rate.
func scale(instants []time.Duration, rate float64) []time.Duration {
	scaled := make([]time.Duration, len(instants))
	for i, at := range instants {
		scaled[i] = time.Duration(float64(at) * rate)
	}
	return scaled
}

// bucketOf rounds a difference to the nearest histogram bucket.
func bucketOf(difference time.Duration) int {
	if difference < 0 {
		return -int((-difference + alignBucket/2) / alignBucket)
	}
	return int((difference + alignBucket/2) / alignBucket)
}

// score is the share of shifted starts within matchTolerance of a reference start.
func score(reference, scaled []time.Duration, offset time.Duration) float64 {
	if len(scaled) == 0 {
		return 0
	}
	matched := 0
	for _, start := range scaled {
		if difference, found := nearest(reference, start+offset); found && difference.Abs() <= matchTolerance {
			matched++
		}
	}
	return float64(matched) / float64(len(scaled))
}

// nearest returns the difference from at to the closest sorted instant.
func nearest(sorted []time.Duration, at time.Duration) (time.Duration, bool) {
	if len(sorted) == 0 {
		return 0, false
	}
	i := sort.Search(len(sorted), func(i int) bool { return sorted[i] >= at })
	best := time.Duration(0)
	found := false
	for _, j := range []int{i - 1, i} {
		if j < 0 || j >= len(sorted) {
			continue
		}
		if difference := sorted[j] - at; !found || difference.Abs() < best.Abs() {
			best, found = difference, true
		}
	}
	return best, found
}

// parts splits sorted instants in cutParts runs of lines, or none when a run
// would be too short to judge.
func parts(instants []time.Duration) [][]time.Duration {
	size := len(instants) / cutParts
	if size < minAlignLines {
		return nil
	}
	split := make([][]time.Duration, cutParts)
	for part := range cutParts {
		end := (part + 1) * size
		if part == cutParts-1 {
			end = len(instants)
		}
		split[part] = instants[part*size : end]
	}
	return split
}

// everyPartFits reports that the alignment holds along the whole file, not
// only for most of it: a scene added near the end leaves the last part off.
func everyPartFits(reference, candidate []time.Duration, alignment Alignment) bool {
	for _, part := range parts(candidate) {
		if !partFits(reference, part, alignment) {
			return false
		}
	}
	return true
}

// partFits reports a part whose lines meet the reference, or whose stretch of
// the film has its reference lines met: a part crowded with extra lines
// (sounds, songs) still covers the dialogue of its scenes.
func partFits(reference, part []time.Duration, alignment Alignment) bool {
	shown := scale(part, alignment.Rate)
	for i := range shown {
		shown[i] += alignment.Offset
	}
	if score(reference, shown, 0) >= partScore {
		return true
	}
	first := sort.Search(len(reference), func(i int) bool { return reference[i] >= shown[0] })
	last := sort.Search(len(reference), func(i int) bool { return reference[i] > shown[len(shown)-1] })
	covered := reference[first:last]
	return len(covered) >= minAlignLines && score(shown, covered, 0) >= partScore
}

// partsLineUp reports a file whose parts line up on their own, each with its
// own offset: the mark of a cut with scenes added or removed.
func partsLineUp(reference, candidate []time.Duration) bool {
	lined := 0
	for _, part := range parts(candidate) {
		if alignAt(reference, part, 1).Score >= partScore {
			lined++
		}
	}
	return lined >= 2
}

// runsPastEnd reports a file whose last line, aligned, starts after the video ends.
func runsPastEnd(candidate []time.Duration, alignment Alignment, videoDuration time.Duration) bool {
	if videoDuration <= 0 || len(candidate) == 0 {
		return false
	}
	last := time.Duration(float64(candidate[len(candidate)-1])*alignment.Rate) + alignment.Offset
	return last > videoDuration+pastEndTolerance
}

// Retime returns copies of events moved by an alignment: each instant t shows
// at t*Rate + Offset. The text is untouched. Starts that would fall before
// zero are clamped and events that would end before zero are dropped. A rate
// that is not positive counts as 1.
func Retime(events []Event, alignment Alignment) []Event {
	rate := alignment.Rate
	if !(rate > 0) || math.IsInf(rate, 0) {
		rate = 1
	}
	move := func(at time.Duration) time.Duration {
		return time.Duration(float64(at)*rate) + alignment.Offset
	}
	moved := make([]Event, 0, len(events))
	for _, event := range events {
		event.Start, event.End = max(move(event.Start), 0), move(event.End)
		if event.End > event.Start {
			moved = append(moved, event)
		}
	}
	return moved
}

// usable returns the instants in order without the ones no film has (before
// zero or after maxAlignInstant), which also keeps the arithmetic below far
// from overflowing.
func usable(instants []time.Duration) []time.Duration {
	if slices.IsSorted(instants) && (len(instants) == 0 || instants[0] >= 0 && instants[len(instants)-1] <= maxAlignInstant) {
		return instants
	}
	kept := make([]time.Duration, 0, len(instants))
	for _, at := range instants {
		if at >= 0 && at <= maxAlignInstant {
			kept = append(kept, at)
		}
	}
	slices.Sort(kept)
	return kept
}

// sample keeps at most maxAlignLines instants, evenly spread.
func sample(sorted []time.Duration) []time.Duration {
	if len(sorted) <= maxAlignLines {
		return sorted
	}
	sampled := make([]time.Duration, maxAlignLines)
	for i := range sampled {
		sampled[i] = sorted[i*len(sorted)/maxAlignLines]
	}
	return sampled
}
