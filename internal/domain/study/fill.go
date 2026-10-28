package study

import (
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// Painting a line by its voice. When the voice was found ahead (decoded
// before the video gets there, or a line heard before), PlanFill lays the
// words over the stretches of voice; when the player is only heard as it
// plays, LiveFill moves with the voice frame by frame. Both give the share
// of visible characters SpokenFraction gives, so the drawing stays the same.

// phrase is the words up to a pause of the text: a comma, a sentence's end
// or a change of speaker.
type phrase struct {
	words  []Unit
	weight float64
}

// phrasesOf splits the words at the text's pauses. A phrase where nothing
// takes time (a sound's description) joins the one after it, or the one
// before at the end, so every phrase is spoken.
func phrasesOf(units Units) []phrase {
	var out []phrase
	current := phrase{}
	for i, word := range units.Words {
		current.words = append(current.words, word)
		current.weight += word.Weight
		if (word.Break != BreakNone || i == len(units.Words)-1) && current.weight > 0 {
			out = append(out, current)
			current = phrase{}
		}
	}
	if len(current.words) > 0 && len(out) > 0 {
		last := &out[len(out)-1]
		last.words = append(last.words, current.words...)
	}
	return out
}

// painted is how many of the words' characters are painted once weight of
// their time has passed: a word fills along its weight, and one that takes
// no time is painted when reached.
func painted(words []Unit, weight float64) float64 {
	done, spent := 0.0, 0.0
	for _, word := range words {
		if word.Weight > 0 && spent+word.Weight > weight {
			return done + float64(word.Chars)*max(weight-spent, 0)/word.Weight
		}
		spent += word.Weight
		done += float64(word.Chars)
	}
	return done
}

func visibleChars(units Units) int {
	total := 0
	for _, word := range units.Words {
		total += word.Chars
	}
	return total
}

// fillPoint is how many characters are painted at a moment.
type fillPoint struct {
	at   time.Duration
	done float64
}

// FillPlan paints a line along the voice found in it: the words rise
// through each stretch of voice and hold in the silences between them.
type FillPlan struct {
	points  []fillPoint
	visible int
}

// PlanFill lays a line's words over the stretches of voice found for it
// (speech.DetectLine). When the text has as many phrases as the voice has
// stretches, each phrase goes in its own; otherwise the words spread over
// all the voice by their syllables.
func PlanFill(units Units, spans []speech.Span) FillPlan {
	if !units.Spoken() || len(spans) == 0 {
		return FillPlan{}
	}
	plan := FillPlan{visible: visibleChars(units)}
	if phrases := phrasesOf(units); len(phrases) == len(spans) {
		before := 0.0
		for i, p := range phrases {
			plan.lay(p.words, p.weight, spans[i:i+1], before)
			before += painted(p.words, p.weight)
		}
		return plan
	}
	weight := 0.0
	for _, word := range units.Words {
		weight += word.Weight
	}
	plan.lay(units.Words, weight, spans, 0)
	return plan
}

// PlanWords paints a line along its words as the speech model placed them
// in the sound (speech.Word, in order): each word said fills over its own
// time, so a quick speaker is followed word by word and what was said
// before the line showed is painted as it shows. What has no time of its
// own (a sound's description, punctuation alone, a number) is painted with
// the next word. When the words heard are not as many as the words said,
// they are the stretches PlanFill spreads the words over.
func PlanWords(units Units, words []speech.Word) FillPlan {
	var heard []speech.Word
	for _, w := range words {
		if w.Heard {
			heard = append(heard, w)
		}
	}
	said := 0
	for _, unit := range units.Words {
		if unit.Weight > 0 {
			said++
		}
	}
	if said != len(heard) {
		spans := make([]speech.Span, len(heard))
		for i, w := range heard {
			spans[i] = w.Span
		}
		return PlanFill(units, spans)
	}
	plan := FillPlan{visible: visibleChars(units)}
	done, next, held := 0.0, 0, 0.0
	var reached time.Duration // the points stay in order even if two words overlap
	for _, unit := range units.Words {
		if unit.Weight > 0 {
			w := heard[next]
			start := max(w.Start, reached)
			reached = max(w.End, start)
			if done > held && len(plan.points) > 0 {
				// What came between holds until this word, then shows with it.
				plan.points = append(plan.points, fillPoint{start, held})
			}
			held = done + float64(unit.Chars)
			plan.points = append(plan.points, fillPoint{start, done}, fillPoint{reached, held})
			next++
		}
		done += float64(unit.Chars)
	}
	if n := len(plan.points); n > 0 && plan.points[n-1].done < done {
		plan.points = append(plan.points, fillPoint{plan.points[n-1].at, done})
	}
	return plan
}

// lay spreads words over spans by the share of voice heard: a point at
// each span's edges and at each word's edge inside it.
func (p *FillPlan) lay(words []Unit, weight float64, spans []speech.Span, before float64) {
	voiced := time.Duration(0)
	for _, span := range spans {
		voiced += span.End - span.Start
	}
	if voiced <= 0 {
		return
	}
	edges := make([]float64, 0, len(words))
	spent := 0.0
	for _, word := range words {
		spent += word.Weight
		edges = append(edges, spent/weight)
	}
	heard := time.Duration(0)
	for _, span := range spans {
		from := float64(heard) / float64(voiced)
		heard += span.End - span.Start
		to := float64(heard) / float64(voiced)
		p.points = append(p.points, fillPoint{span.Start, before + painted(words, from*weight)})
		for _, edge := range edges {
			if edge > from && edge < to {
				at := span.Start + time.Duration((edge-from)/(to-from)*float64(span.End-span.Start))
				p.points = append(p.points, fillPoint{at, before + painted(words, edge*weight)})
			}
		}
		p.points = append(p.points, fillPoint{span.End, before + painted(words, to*weight)})
	}
}

// Fraction is the share of the line's visible characters painted at a
// moment on the clock of the spans: none before the voice, all after it.
func (p FillPlan) Fraction(at time.Duration) float64 {
	if len(p.points) == 0 || at < p.points[0].at {
		return 0
	}
	i, _ := slices.BinarySearchFunc(p.points, at, func(point fillPoint, at time.Duration) int {
		if point.at <= at {
			return -1
		}
		return 1
	})
	if i == len(p.points) {
		return 1
	}
	from, to := p.points[i-1], p.points[i]
	done := from.done + (to.done-from.done)*float64(at-from.at)/float64(to.at-from.at)
	return min(done/float64(p.visible), 1)
}

// How LiveFill follows a voice it hears only as it comes. It moves at
// least at a typical pace of speech, so it does not lag a quick speaker; it
// stops at the end of each phrase. A silence of pauseHeard is a pause in
// the speech: after one, a phrase mostly painted is finished and the next
// begins. A voice that runs on for runThrough past a phrase's end ran
// through the comma. The line is all said when the voice stops for
// finishSilence in its last phrase.
const (
	typicalPace   = 5.0 // syllables per second of voice
	pauseHeard    = 150 * time.Millisecond
	runThrough    = 300 * time.Millisecond
	mostlySaid    = 0.6
	finishSilence = 300 * time.Millisecond
)

// LiveFill paints a line as its voice is heard. It never goes back.
type LiveFill struct {
	words      []Unit
	ends       []float64 // the weight where each phrase ends
	visible    int
	start, end time.Duration
	pace       float64 // weight per second of voice

	spent             float64
	phrase            int
	held, heard, done bool
	voice             bool
	last, lastVoice   time.Duration
	heldSince         time.Duration
}

// NewLiveFill follows a line spoken from start to end on the clock of the
// frames it will hear.
func NewLiveFill(units Units, start, end time.Duration) *LiveFill {
	f := &LiveFill{words: units.Words, visible: visibleChars(units), start: start, end: end, last: start}
	if !units.Spoken() || end <= start {
		return f
	}
	total, ends := 0.0, 0.0
	for _, word := range units.Words {
		total += word.Weight + pauseWeight(word.Break)
	}
	for _, p := range phrasesOf(units) {
		ends += p.weight
		f.ends = append(f.ends, ends)
	}
	f.pace = max(total/(speakingShare*(end-start).Seconds()), typicalPace)
	return f
}

// Hear takes whether a frame at a moment holds voice; frames come in order,
// and those before the line are not heard.
func (f *LiveFill) Hear(at time.Duration, voice bool) {
	if len(f.ends) == 0 || f.done || at < f.last {
		return
	}
	if at >= f.end {
		f.done = true
		return
	}
	elapsed := (at - f.last).Seconds()
	switch {
	case voice:
		if f.heard && !f.voice && at-f.lastVoice >= pauseHeard {
			f.afterPause()
		}
		f.heard, f.lastVoice = true, at
		if f.held && at-f.heldSince >= runThrough {
			f.next()
		}
		if !f.held {
			f.advance(at, elapsed)
		}
	case f.heard && f.phrase == len(f.ends)-1 && at-f.lastVoice >= finishSilence && f.spent >= f.phraseStart()+f.phraseWeight()/2:
		f.done = true
	}
	f.voice, f.last = voice, at
}

func (f *LiveFill) phraseStart() float64 {
	if f.phrase == 0 {
		return 0
	}
	return f.ends[f.phrase-1]
}

func (f *LiveFill) phraseWeight() float64 { return f.ends[f.phrase] - f.phraseStart() }

// afterPause: the speaker paused. A phrase held at its end, or mostly
// painted, is over; a phrase barely begun was only a hesitation.
func (f *LiveFill) afterPause() {
	if f.phrase == len(f.ends)-1 {
		return
	}
	if f.held || f.spent >= f.phraseStart()+mostlySaid*f.phraseWeight() {
		f.spent = f.ends[f.phrase]
		f.next()
	}
}

func (f *LiveFill) next() {
	f.held = false
	f.phrase = min(f.phrase+1, len(f.ends)-1)
}

func (f *LiveFill) advance(at time.Duration, elapsed float64) {
	f.spent = min(f.spent+f.pace*elapsed, f.ends[f.phrase])
	if f.spent == f.ends[f.phrase] && f.phrase < len(f.ends)-1 {
		f.held, f.heldSince = true, at
	}
}

// Fraction is the share of the line's visible characters painted so far.
func (f *LiveFill) Fraction() float64 {
	switch {
	case len(f.ends) == 0:
		return 0
	case f.done:
		return 1
	}
	return min(painted(f.words, f.spent)/float64(f.visible), 1)
}
