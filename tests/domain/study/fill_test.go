package study_test

import (
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

const ms = time.Millisecond

// "How... (2 s)... are you?": 13 visible characters, "How..." six of them.
var howAreYou = []speech.Span{{Start: time.Second, End: 1250 * ms}, {Start: 3250 * ms, End: 3750 * ms}}

// The plan lays each phrase in its stretch of voice and holds in between;
// when the stretches do not match the phrases, the words share all the
// voice by their syllables.
func TestThePlanFollowsTheVoice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		text    string
		spans   []speech.Span
		visible float64
		at      []time.Duration
		want    []float64 // characters painted
	}{
		{"a pause between the phrases", "How... are you?", howAreYou, 13,
			[]time.Duration{950 * ms, 1125 * ms, 1250 * ms, 2 * time.Second, 3375 * ms, 3600 * ms, 3800 * ms},
			[]float64{0, 3, 6, 6, 7.5, 10.6, 13}},
		{"phrases of different lengths", "Hi... how are you doing?", []speech.Span{{Start: time.Second, End: 1400 * ms}, {Start: 3 * time.Second, End: 3600 * ms}}, 20,
			[]time.Duration{1200 * ms, 1400 * ms, 2 * time.Second, 3300 * ms},
			[]float64{2.5, 5, 5, 11}},
		{"one stretch for two phrases", "Wait, come back", []speech.Span{{Start: time.Second, End: 1780 * ms}}, 13,
			[]time.Duration{1130 * ms, 1260 * ms, 1650 * ms},
			[]float64{2.5, 5, 11}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			plan := study.PlanFill(study.SpeechUnits(c.text), c.spans)
			for i, at := range c.at {
				if got := plan.Fraction(at) * c.visible; math.Abs(got-c.want[i]) > 0.01 {
					t.Errorf("at %v: %.2f characters painted, want %.2f", at, got, c.want[i])
				}
			}
		})
	}
}

// Sintel's old man at 1:58, quick: "You're a" is said before the line shows
// (at 10 s of the cut), and the whole line in 2.6 s of its 3.45 s. The words
// as the speech model placed them, in the cut's time.
var aFoolsWords = []speech.Word{
	{Span: speech.Span{Start: 9720 * ms, End: 9880 * ms}, Heard: true, Confidence: 0.83},   // You're
	{Span: speech.Span{Start: 9920 * ms, End: 9940 * ms}, Heard: true, Confidence: 0.99},   // a
	{Span: speech.Span{Start: 10000 * ms, End: 10180 * ms}, Heard: true, Confidence: 0.97}, // fool
	{Span: speech.Span{Start: 10240 * ms, End: 10340 * ms}, Heard: true, Confidence: 1},    // for
	{Span: speech.Span{Start: 10420 * ms, End: 10760 * ms}, Heard: true, Confidence: 0.98}, // traveling
	{Span: speech.Span{Start: 10800 * ms, End: 11040 * ms}, Heard: true, Confidence: 0.98}, // alone,
	{Span: speech.Span{Start: 11120 * ms, End: 11220 * ms}, Heard: true, Confidence: 0.95}, // so
	{Span: speech.Span{Start: 11300 * ms, End: 11700 * ms}, Heard: true, Confidence: 0.99}, // completely
	{Span: speech.Span{Start: 11780 * ms, End: 12320 * ms}, Heard: true, Confidence: 0.75}, // unprepared.
}

// The plan by words paints each word over the time it was said: as the line
// shows, what was already said is painted; at each word's end everything up
// to it is; and the line is whole when the last word ends, not when the line
// goes. A number has no time of its own and is painted with the next word;
// words placed in another count than the words said fall back to the spread.
func TestThePlanByWordsFollowsAQuickSpeaker(t *testing.T) {
	t.Parallel()
	const text = "You're a fool for traveling alone,\nso completely unprepared."
	plan := study.PlanWords(study.SpeechUnits(text), aFoolsWords)
	const visible = 52.0
	checks := []struct {
		at   time.Duration
		want float64 // characters painted
	}{
		{9 * time.Second, 0},
		{10 * time.Second, 7}, // shown: "You're a" already said
		{10180 * ms, 11},      // "fool" said
		{10760 * ms, 23},      // "traveling" said
		{10920 * ms, 26},      // halfway through "alone,"
		{11040 * ms, 29},      // "alone," said: only "so completely unprepared." left
		{12320 * ms, visible}, // the last word said: all of it
		{13 * time.Second, visible},
	}
	for _, c := range checks {
		if got := plan.Fraction(c.at) * visible; math.Abs(got-c.want) > 0.01 {
			t.Errorf("at %v: %.2f characters painted, want %.2f", c.at, got, c.want)
		}
	}

	numbered := study.PlanWords(study.SpeechUnits("Gate 9 opens"), []speech.Word{
		{Span: speech.Span{Start: time.Second, End: 1300 * ms}, Heard: true},
		{Heard: false},
		{Span: speech.Span{Start: 1500 * ms, End: 1900 * ms}, Heard: true},
	})
	if got := numbered.Fraction(1400*ms) * 10; math.Abs(got-4) > 0.01 {
		t.Errorf("between the words: %.2f characters painted, want 4 (\"Gate\", the number waits)", got)
	}
	if got := numbered.Fraction(1500*ms) * 10; math.Abs(got-5) > 0.01 {
		t.Errorf("at the next word: %.2f characters painted, want 5 (the number with it)", got)
	}

	// What ends the line without a time of its own is painted as the last
	// word ends: the fill never stops short of the line's end.
	described := study.PlanWords(study.SpeechUnits("Open gate 9"), []speech.Word{
		{Span: speech.Span{Start: time.Second, End: 1300 * ms}, Heard: true},
		{Span: speech.Span{Start: 1500 * ms, End: 1900 * ms}, Heard: true},
	})
	if got := described.Fraction(1900 * ms); got != 1 {
		t.Errorf("as the last word ends: %.3f of the line painted, want all of it, the number too", got)
	}

	spread := study.PlanWords(study.SpeechUnits(text), aFoolsWords[:5])
	if got, want := spread.Fraction(10760*ms), study.PlanFill(study.SpeechUnits(text), spansOf(aFoolsWords[:5])).Fraction(10760*ms); got != want {
		t.Errorf("five words placed for nine said: %.3f painted, want the spread over them (%.3f)", got, want)
	}
}

func spansOf(words []speech.Word) []speech.Span {
	spans := make([]speech.Span, len(words))
	for i, w := range words {
		spans[i] = w.Span
	}
	return spans
}

// checkpoint is the characters painted (of 13) when a frame was heard.
type checkpoint struct {
	at       time.Duration
	min, max float64
}

// The live fill moves with the voice it hears, never back: it holds through
// a pause, goes on when a comma is run through, and ends when the voice of
// the last phrase does.
func TestTheLiveFillFollowsTheVoice(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		text       string
		start, end time.Duration
		voice      []speech.Span
		checks     []checkpoint
	}{
		{"a pause between the phrases", "How... are you?", 900 * ms, 3900 * ms, howAreYou, []checkpoint{
			{1240 * ms, 5, 6},   // most of "How..."
			{3240 * ms, 5, 6},   // held through the pause
			{3260 * ms, 6, 6.5}, // the pause ended the phrase
			{3740 * ms, 12, 13}, // "are you?" said
			{3900 * ms, 13, 13}, // the line is over
		}},
		{"a comma run through", "Wait, come back", 900 * ms, 2500 * ms, []speech.Span{{Start: time.Second, End: 1900 * ms}}, []checkpoint{
			{1400 * ms, 5, 5},   // waits at the comma
			{1890 * ms, 9, 11},  // the voice went on, so does the fill
			{2200 * ms, 13, 13}, // the voice of the last phrase stopped
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fill := study.NewLiveFill(study.SpeechUnits(c.text), c.start, c.end)
			before, next := 0.0, 0
			for at := c.start; at <= c.end && next < len(c.checks); at += 10 * ms {
				voice := false
				for _, span := range c.voice {
					voice = voice || (at >= span.Start && at < span.End)
				}
				fill.Hear(at, voice)
				got := fill.Fraction() * 13
				if got < before {
					t.Fatalf("at %v the fill went back from %.2f to %.2f", at, before, got)
				}
				before = got
				if check := c.checks[next]; at == check.at {
					if got < check.min-0.01 || got > check.max+0.01 {
						t.Errorf("at %v: %.2f characters painted, want %.1f..%.1f", at, got, check.min, check.max)
					}
					next++
				}
			}
			if next < len(c.checks) {
				t.Fatalf("checkpoint at %v never reached", c.checks[next].at)
			}
		})
	}
}
