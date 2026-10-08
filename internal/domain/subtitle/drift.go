package subtitle

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// Keeping a file on the voice while the film plays. A file that fits at the
// start can slip later: a version with a scene cut or added shows its lines
// early or late from there on. A window of the voice heard around the
// playhead tells how far the lines shown are from it (ReadDrift); a
// DriftTracker trusts only windows that agree, and moves the lines in small
// steps.
//
// The thresholds are starting points: measure them with lrdev drift-check
// on films before changing them.
const (
	// driftReach is how far a window looks: past it, it is another cut, not
	// a drift.
	driftReach       = 10 * time.Second
	driftCoarseStep  = 50 * time.Millisecond
	driftFineReach   = 50 * time.Millisecond
	minDriftLines    = 12
	minDriftVoiced   = 0.15
	maxDriftVoiced   = 0.85
	driftSharpness   = 0.15
	driftAgreeing    = 3
	driftTolerance   = 100 * time.Millisecond
	driftStart       = 250 * time.Millisecond
	driftStepSize    = 200 * time.Millisecond
	driftStepPause   = 3 * time.Second
	driftCooldown    = time.Minute
	driftVetoWindow  = time.Minute
	driftFreshWindow = 20 * time.Second
	// driftLookBack is the voice before the playhead a window holds; a drift
	// found is taken to start in its middle.
	driftLookBack = 100 * time.Second
)

// DriftReading is what one window of voice tells about the lines shown in
// it: Residual is how far they should move to fall on the voice; Score
// the share of them holding voice there; Sharpness how much better that
// place fits than any a second or more away, per frame under the lines
// (from 0 to 2); Lines how many were heard whole; Voiced the share of the
// window heard as voice.
type DriftReading struct {
	Residual         time.Duration
	Score, Sharpness float64
	Lines            int
	Voiced           float64
	edge             bool
}

// Trusted tells a window that says something: enough lines, a voice that is
// neither missing nor everywhere (a song, a crowd), most lines on it, and a
// clear best.
func (r DriftReading) Trusted() bool {
	return !r.edge && r.Lines >= minDriftLines && r.Voiced >= minDriftVoiced && r.Voiced <= maxDriftVoiced &&
		r.Score >= voiceMatchScore && r.Sharpness >= driftSharpness
}

// ReadDrift lines up the lines as they are shown now (in the film's time)
// with a window of the voice heard.
func ReadDrift(heard speech.Track, shown []speech.Span) DriftReading {
	if len(heard.Frames) == 0 {
		return DriftReading{}
	}
	h := hear(heard)
	v := voiceLines{}
	for _, line := range shown {
		if line.End > line.Start && line.End > h.start-driftReach && line.Start < h.end+driftReach {
			v.lines = append(v.lines, line)
			v.longest = max(v.longest, line.End-line.Start)
		}
	}
	slices.SortFunc(v.lines, func(a, b speech.Span) int { return int(a.Start - b.Start) })
	window := []heardVoice{h}
	type tried struct {
		offset time.Duration
		fit    float64
	}
	var all []tried
	for o := -driftReach; o <= driftReach; o += driftCoarseStep {
		all = append(all, tried{o, v.fit(window, 1, o)})
	}
	top := slices.MaxFunc(all, func(a, b tried) int { return cmpFloat(a.fit, b.fit) })
	best, bestFit := top.offset, top.fit
	for o := top.offset - driftFineReach; o <= top.offset+driftFineReach; o += voiceFineStep {
		if fit := v.fit(window, 1, o); fit > bestFit {
			best, bestFit = o, fit
		}
	}
	rivalFit := math.Inf(-1)
	for _, t := range all {
		if abs(t.offset-best) > voiceRivalReach {
			rivalFit = max(rivalFit, t.fit)
		}
	}
	under := 0
	v.within(h, 1, best, func(start, end time.Duration) { under += h.frame(end) - h.frame(start) })
	r := DriftReading{Residual: best, edge: abs(best) > driftReach-driftCoarseStep}
	if under > 0 && !math.IsInf(rivalFit, -1) {
		r.Sharpness = (bestFit - rivalFit) / float64(under)
	}
	r.Score, r.Lines = v.score(window, 1, best)
	r.Voiced = float64(h.voiced[len(h.voiced)-1]) / float64(len(heard.Frames))
	return r
}

// DriftAnchor says the lines are shown Auto later (earlier when negative)
// from From on.
type DriftAnchor struct{ From, Auto time.Duration }

// heardDrift is a trusted window kept until others agree with it.
type heardDrift struct{ drift, position time.Duration }

// DriftTracker follows one line's drift through a film: it learns where the
// voice sits when the film starts (its base), then corrects the moves from
// there. A correction made by hand is the new base; one against a step just
// taken, or putting the line back to zero, stops the tracker for the
// session. It is not safe for concurrent use.
type DriftTracker struct {
	base       time.Duration
	hasBase    bool
	recent     []heardDrift
	high       time.Duration
	auto       time.Duration
	target     time.Duration
	correcting bool
	from       time.Duration
	lastStep   time.Time
	stepSign   int
	quietUntil time.Time
	anchors    []DriftAnchor
	stopped    bool
}

// Auto is the shift the tracker applies now, on top of the user's own.
func (t *DriftTracker) Auto() time.Duration { return t.auto }

// Stopped tells a tracker the user overruled.
func (t *DriftTracker) Stopped() bool { return t.stopped }

// Anchors are the corrections settled so far, by where they start.
func (t *DriftTracker) Anchors() []DriftAnchor { return slices.Clone(t.anchors) }

// Heard takes a window read at a playhead position, ending at windowEnd in
// the film. A window counts only when it reaches driftFreshWindow past every
// window before it: voice heard again (a passage repeated, a seek back)
// tells nothing new.
func (t *DriftTracker) Heard(r DriftReading, position, windowEnd time.Duration, now time.Time) {
	if t.stopped || !r.Trusted() || windowEnd < t.high+driftFreshWindow {
		return
	}
	t.high = windowEnd
	drift := r.Residual
	if t.hasBase {
		drift -= t.base
	}
	t.recent = append(t.recent, heardDrift{drift: drift, position: position})
	if len(t.recent) > driftAgreeing {
		t.recent = t.recent[1:]
	}
	if len(t.recent) < driftAgreeing {
		return
	}
	drifts := make([]time.Duration, len(t.recent))
	for i, h := range t.recent {
		drifts[i] = h.drift
	}
	slices.Sort(drifts)
	if drifts[len(drifts)-1]-drifts[0] > driftTolerance {
		return
	}
	agreed, first := drifts[len(drifts)/2], t.recent[0].position
	t.recent = nil
	switch {
	case !t.hasBase:
		t.base, t.hasBase = agreed, true
	case t.correcting && abs(agreed) < driftStart/2:
		// Close enough: the rest is one last move, and the correction settles.
		t.correcting, t.target = false, t.auto+agreed
		t.settle()
		t.quietUntil = now.Add(driftCooldown)
	case t.correcting:
		t.target = t.auto + agreed
	case abs(agreed) >= driftStart && !now.Before(t.quietUntil):
		t.correcting, t.target = true, t.auto+agreed
		t.from = max(first-driftLookBack/2, 0)
	}
}

// settle keeps the correction reached as an anchor, replacing any after it.
func (t *DriftTracker) settle() {
	keep := sort.Search(len(t.anchors), func(i int) bool { return t.anchors[i].From >= t.from })
	t.anchors = append(t.anchors[:keep], DriftAnchor{From: t.from, Auto: t.target})
}

// Step is the move to make now, if any: at most driftStepSize, one every
// driftStepPause.
func (t *DriftTracker) Step(now time.Time) (time.Duration, bool) {
	if t.stopped || t.auto == t.target || now.Sub(t.lastStep) < driftStepPause {
		return 0, false
	}
	step := max(min(t.target-t.auto, driftStepSize), -driftStepSize)
	t.auto += step
	t.lastStep, t.stepSign = now, sign(step)
	// What was heard before the move was heard with the lines elsewhere.
	t.recent = nil
	return step, true
}

// Seek tells the tracker the playhead jumped; it answers the shift the
// lines take there, from the corrections settled, and drops one under way.
func (t *DriftTracker) Seek(position time.Duration) time.Duration {
	t.recent, t.correcting = nil, false
	t.auto = 0
	for _, a := range t.anchors {
		if a.From <= position {
			t.auto = a.Auto
		}
	}
	t.target = t.auto
	return t.auto
}

// Nudged tells the tracker the user moved the line by delta. The place
// chosen is the new base; a move against a step taken in the last minute
// stops the tracker.
func (t *DriftTracker) Nudged(delta time.Duration, now time.Time) {
	if delta == 0 || t.stopped {
		return
	}
	if t.stepSign != 0 && sign(delta) == -t.stepSign && now.Sub(t.lastStep) < driftVetoWindow {
		t.Stop()
		return
	}
	t.base -= delta
	t.recent, t.correcting, t.target = nil, false, t.auto
	t.quietUntil = now.Add(driftCooldown)
}

// Stop ends the tracker for the session: the user put the line back to its
// start, or overruled a step. The shift reached stays until a seek.
func (t *DriftTracker) Stop() {
	t.stopped, t.correcting, t.recent = true, false, nil
}

func sign(d time.Duration) int {
	switch {
	case d > 0:
		return 1
	case d < 0:
		return -1
	}
	return 0
}
