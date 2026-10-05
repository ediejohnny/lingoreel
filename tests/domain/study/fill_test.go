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
