package subtitle

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// A video with no subtitle of its own to compare with is heard instead: the
// lines of a file that fits it fall where people speak. Only stretches of
// the film are heard, spread from its first scenes to its last, away from
// the opening and the credits, where nobody speaks or a song plays.
const (
	hearingCount  = 6
	hearingLength = 45 * time.Second
	hearingFrom   = 0.15
	hearingTo     = 0.85
)

// How a file is lined up with the voice: offsets are tried every
// voiceCoarseStep over the whole alignWindow, then every voiceFineStep
// around the best; a frame with voiceSpeech of speech is voice.
const (
	voiceCoarseStep = 100 * time.Millisecond
	voiceFineReach  = 150 * time.Millisecond
	voiceFineStep   = 10 * time.Millisecond
	voiceSpeech     = 0.5
	// lineVoice is the voice a line needs in it to fall where someone speaks.
	lineVoice = 150 * time.Millisecond
	// minVoiceLines is the fewest lines heard to tell anything.
	minVoiceLines = 20
	// voiceGroups split the stretches heard in groups that each line the file
	// up on their own; the file fits when groupsAgreeing of them find the
	// offset found, within groupTolerance. A share of the lines falling in
	// speech tells little alone, as most of a film is speech: files of other
	// films scored up to 1.00. Measured on nine films, the groups of a file
	// that fits found offsets up to about 400 ms apart (lines timed by hand,
	// a cartoon with music under its few lines), those of every other file
	// seconds to minutes apart. voiceMatchScore keeps out a file that agrees
	// by chance.
	voiceGroups     = 3
	groupsAgreeing  = 2
	groupTolerance  = 500 * time.Millisecond
	voiceMatchScore = 0.6
	// voiceRivalReach is how far another offset is to count as a rival.
	voiceRivalReach = time.Second
)

// HearingStretch is a stretch of a film to hear.
type HearingStretch struct{ From, Length time.Duration }

// HearingStretches are the stretches of a film heard to tell which file fits
// it: hearingCount of hearingLength, spread from 15% to 85% of the film, or
// that whole part when the film is too short for them.
func HearingStretches(duration time.Duration) []HearingStretch {
	if duration <= 0 {
		return nil
	}
	from, to := time.Duration(float64(duration)*hearingFrom), time.Duration(float64(duration)*hearingTo)
	span := to - from
	if span < hearingCount*hearingLength {
		return []HearingStretch{{From: from, Length: span}}
	}
	stretches := make([]HearingStretch, hearingCount)
	for i := range stretches {
		center := from + span*time.Duration(2*i+1)/time.Duration(2*hearingCount)
		stretches[i] = HearingStretch{From: center - hearingLength/2, Length: hearingLength}
	}
	return stretches
}

// HearingStretchesBetween are the stretches heard when the first ones leave
// a file undecided (VoiceAlignment.Undecided): one halfway between each two
// of HearingStretches, of the same length; none when there are not two.
func HearingStretchesBetween(duration time.Duration) []HearingStretch {
	first := HearingStretches(duration)
	if len(first) < 2 {
		return nil
	}
	between := make([]HearingStretch, len(first)-1)
	for i := range between {
		between[i] = HearingStretch{From: (first[i].From + first[i+1].From) / 2, Length: first[i].Length}
	}
	return between
}

// InterleaveHeard puts the stretches heard between the first ones back in
// the film's order: first[0], between[0], first[1], and so on.
func InterleaveHeard(first, between []speech.Track) []speech.Track {
	all := make([]speech.Track, 0, len(first)+len(between))
	for i, track := range first {
		all = append(all, track)
		if i < len(between) {
			all = append(all, between[i])
		}
	}
	return all
}

// VoiceAlignment is how a file lines up with the voice heard, with what the
// verdict was told by.
type VoiceAlignment struct {
	Alignment
	// Lines is how many of the file's lines fell in the stretches heard.
	Lines int
	// Rival is the score of the best other offset, voiceRivalReach away or more.
	Rival float64
	// Groups are the offsets each group of stretches finds on its own.
	Groups []time.Duration
}

// Undecided tells the file may fit though its groups did not agree: enough
// of its lines fell in speech, and groups of few lines each find chance
// offsets. More stretches (HearingStretchesBetween) can tell.
func (v VoiceAlignment) Undecided() bool {
	return v.Verdict == VerdictNoMatch && v.Score >= voiceMatchScore
}

// heardVoice is a stretch heard: s = 2p-1 for each frame (voice above zero,
// silence below), summed from the stretch's start, and the frames heard as
// voice summed the same way.
type heardVoice struct {
	start, end time.Duration
	sum        []float64
	voiced     []int
}

func hear(track speech.Track) heardVoice {
	h := heardVoice{
		start: track.Start, end: track.Start + time.Duration(len(track.Frames))*speech.FrameLength,
		sum: make([]float64, len(track.Frames)+1), voiced: make([]int, len(track.Frames)+1),
	}
	for i, frame := range track.Frames {
		h.sum[i+1] = h.sum[i] + 2*float64(frame.Speech) - 1
		h.voiced[i+1] = h.voiced[i]
		if frame.Speech >= voiceSpeech {
			h.voiced[i+1]++
		}
	}
	return h
}

// frame is the frame of the stretch at a time, held within it.
func (h heardVoice) frame(at time.Duration) int {
	return max(min(int((at-h.start)/speech.FrameLength), len(h.sum)-1), 0)
}

// voiceLines are a file's spoken lines, sorted, with the longest one's length.
type voiceLines struct {
	lines   []speech.Span
	longest time.Duration
}

// within calls each line that, shown by the alignment, may meet a stretch.
func (v voiceLines) within(h heardVoice, rate float64, offset time.Duration, each func(start, end time.Duration)) {
	from := time.Duration(float64(h.start-offset)/rate) - v.longest
	to := time.Duration(float64(h.end-offset) / rate)
	first := sort.Search(len(v.lines), func(i int) bool { return v.lines[i].Start >= from })
	for _, line := range v.lines[first:] {
		if line.Start >= to {
			return
		}
		each(time.Duration(float64(line.Start)*rate)+offset, time.Duration(float64(line.End)*rate)+offset)
	}
}

// fit is how well the lines fall on the voice of some stretches: the sum of
// s under them.
func (v voiceLines) fit(heard []heardVoice, rate float64, offset time.Duration) float64 {
	total := 0.0
	for _, h := range heard {
		v.within(h, rate, offset, func(start, end time.Duration) {
			total += h.sum[h.frame(end)] - h.sum[h.frame(start)]
		})
	}
	return total
}

// score is the share of the lines heard whole that hold lineVoice of voice,
// and how many were heard.
func (v voiceLines) score(heard []heardVoice, rate float64, offset time.Duration) (float64, int) {
	least := int(lineVoice / speech.FrameLength)
	hit, lines := 0, 0
	for _, h := range heard {
		v.within(h, rate, offset, func(start, end time.Duration) {
			if start < h.start || end > h.end {
				return
			}
			lines++
			if h.voiced[h.frame(end)]-h.voiced[h.frame(start)] >= least {
				hit++
			}
		})
	}
	if lines == 0 {
		return 0, 0
	}
	return float64(hit) / float64(lines), lines
}

// best finds the offset that fits best at a rate, coarse then fine, and
// the best fit more than voiceRivalReach away from it.
func (v voiceLines) best(heard []heardVoice, rate float64) (offset, rival time.Duration) {
	type tried struct {
		offset time.Duration
		fit    float64
	}
	var all []tried
	for o := -alignWindow; o <= alignWindow; o += voiceCoarseStep {
		all = append(all, tried{o, v.fit(heard, rate, o)})
	}
	top := slices.MaxFunc(all, func(a, b tried) int { return cmpFloat(a.fit, b.fit) })
	offset, bestFit := top.offset, top.fit
	for o := top.offset - voiceFineReach; o <= top.offset+voiceFineReach; o += voiceFineStep {
		if fit := v.fit(heard, rate, o); fit > bestFit {
			offset, bestFit = o, fit
		}
	}
	rivalFit := math.Inf(-1)
	for _, t := range all {
		if (t.offset-offset).Abs() > voiceRivalReach && t.fit > rivalFit {
			rival, rivalFit = t.offset, t.fit
		}
	}
	return offset, rival
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// AlignToVoice lines up a file's spoken lines with the voice heard in
// stretches of the video (HearingStretches), in the file's time: a line
// instant t shows at t*Rate + Offset, as with Align.
func AlignToVoice(heard []speech.Track, lines []speech.Span, videoDuration time.Duration) VoiceAlignment {
	unknown := VoiceAlignment{Alignment: Alignment{Rate: 1, Verdict: VerdictUnknown, Heard: true}}
	v := voiceLines{}
	for _, line := range lines {
		if line.Start >= 0 && line.End > line.Start && line.End <= maxAlignInstant {
			v.lines = append(v.lines, line)
			v.longest = max(v.longest, line.End-line.Start)
		}
	}
	slices.SortFunc(v.lines, func(a, b speech.Span) int { return int(a.Start - b.Start) })
	stretches := make([]heardVoice, 0, len(heard))
	for _, track := range heard {
		if len(track.Frames) > 0 {
			stretches = append(stretches, hear(track))
		}
	}
	if len(v.lines) < minAlignLines || len(stretches) == 0 {
		return unknown
	}

	result := VoiceAlignment{Alignment: Alignment{Rate: 1, Heard: true}}
	offset, rival := v.best(stretches, 1)
	result.Offset = offset
	result.Score, result.Lines = v.score(stretches, 1, offset)
	result.Rival, _ = v.score(stretches, 1, rival)
	for _, rate := range frameRateRatios {
		o, r := v.best(stretches, rate)
		if s, n := v.score(stretches, rate, o); s >= result.Score+rateMargin {
			result.Offset, result.Rate, result.Score, result.Lines = o, rate, s, n
			result.Rival, _ = v.score(stretches, rate, r)
		}
	}
	if result.Lines < minVoiceLines {
		unknown.Lines = result.Lines
		return unknown
	}

	agree := 0
	size := (len(stretches) + voiceGroups - 1) / voiceGroups
	for from := 0; from < len(stretches); from += size {
		group, _ := v.best(stretches[from:min(from+size, len(stretches))], result.Rate)
		result.Groups = append(result.Groups, group)
		if (group - result.Offset).Abs() <= groupTolerance {
			agree++
		}
	}
	switch {
	case runsPastEnd(Starts(eventsOf(v.lines)), result.Alignment, videoDuration):
		result.Verdict = VerdictOtherCut
	case result.Score >= voiceMatchScore && agree >= groupsAgreeing:
		result.Verdict = VerdictMatch
	default:
		result.Verdict = VerdictNoMatch
	}
	return result
}

// eventsOf turns spans into events, for the helpers that take events.
func eventsOf(spans []speech.Span) []Event {
	events := make([]Event, len(spans))
	for i, span := range spans {
		events[i] = Event{Start: span.Start, End: span.End}
	}
	return events
}
