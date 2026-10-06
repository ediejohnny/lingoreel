package subtitle_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// pairedAt is the translation shown at a position (ms) with the study events
// active there, as the session asks for it.
func pairedAt(study, translation subtitle.Placed, at int) []string {
	position := ms(at)
	shown := study.Timeline.ActiveAt(study.Offset.TrackTime(position))
	return activeIDs(subtitle.Paired(shown, study, translation, position, 4))
}

// The translation comes and goes with the study line it translates (user
// report: the two lines appeared and disappeared a few milliseconds apart).
func TestTheTranslationComesAndGoesWithItsStudyLine(t *testing.T) {
	t.Parallel()
	study := subtitle.Placed{Timeline: mustTimeline(t,
		event("en1", 10_000, 12_000),
		event("en2", 20_000, 22_000), event("en3", 22_000, 24_000),
		event("en4", 30_000, 34_000),
	)}
	translation := subtitle.Placed{Timeline: mustTimeline(t,
		event("pt1", 9_800, 12_300),                                // a little early and a little late
		event("pt2", 20_000, 24_000),                               // one translation for two study lines
		event("pt3", 30_000, 32_000), event("pt4", 32_000, 34_000), // two for one
		event("sign", 40_000, 42_000), // nothing in the study line says it
	)}
	for _, c := range []struct {
		at   int
		want []string
	}{
		{9_900, []string{}},       // the study line has not started: neither has its translation
		{10_000, []string{"pt1"}}, // both start together
		{11_999, []string{"pt1"}},
		{12_000, []string{}}, // both end together
		{12_200, []string{}},
		{21_000, []string{"pt2"}},
		{23_000, []string{"pt2"}},
		{31_000, []string{"pt3", "pt4"}}, // the whole translation of the line shown
		{41_000, []string{"sign"}},       // on its own time
		{42_000, []string{}},
	} {
		if got := pairedAt(study, translation, c.at); !slices.Equal(got, c.want) {
			t.Errorf("at %d ms: %v, want %v", c.at, got, c.want)
		}
	}
}

// Each line's offset puts it on the playback clock before they are paired: a
// translation file a second early, shifted a second later, pairs as if it
// were exact.
func TestPairingFollowsEachLinesOffset(t *testing.T) {
	t.Parallel()
	study := subtitle.Placed{Timeline: mustTimeline(t, event("en", 10_000, 12_000))}
	translation := subtitle.Placed{Timeline: mustTimeline(t, event("pt", 9_000, 11_000)), Offset: subtitle.Offset(time.Second)}
	if got := pairedAt(study, translation, 10_000); !slices.Equal(got, []string{"pt"}) {
		t.Fatalf("at 10 s: %v, want [pt]", got)
	}
	if got := pairedAt(study, translation, 12_500); len(got) != 0 {
		t.Fatalf("after the line: %v, want nothing", got)
	}
	// Unshifted, the same file is only half over the line: still the same
	// line (the overlap is half of the shorter one), and it waits for it.
	translation.Offset = 0
	if got := pairedAt(study, translation, 9_500); len(got) != 0 {
		t.Fatalf("before the line: %v, want nothing", got)
	}
}

// A translation barely touching a study line is not its translation: it
// shows on its own time.
func TestABarelyTouchingTranslationKeepsItsOwnTime(t *testing.T) {
	t.Parallel()
	study := subtitle.Placed{Timeline: mustTimeline(t, event("en", 10_000, 12_000))}
	translation := subtitle.Placed{Timeline: mustTimeline(t, event("pt", 11_800, 15_000))}
	for _, c := range []struct {
		at   int
		want []string
	}{
		{11_900, []string{"pt"}},
		{14_000, []string{"pt"}},
	} {
		if got := pairedAt(study, translation, c.at); !slices.Equal(got, c.want) {
			t.Errorf("at %d ms: %v, want %v", c.at, got, c.want)
		}
	}
}

// A hostile file stacking thousands of events never stalls a frame nor shows
// more than the limit.
func TestPairingAHostileFileKeepsTheLimit(t *testing.T) {
	t.Parallel()
	var many []subtitle.Event
	for i := range 20_000 {
		many = append(many, event(fmt.Sprint(i), 10_000, 20_000+i))
	}
	study := subtitle.Placed{Timeline: mustTimeline(t, many...)}
	translation := subtitle.Placed{Timeline: mustTimeline(t, many...)}
	started := time.Now()
	shown := study.Timeline.ActiveAt(ms(15_000))
	got := subtitle.Paired(shown[len(shown)-4:], study, translation, ms(15_000), 4)
	if len(got) > 4 {
		t.Fatalf("%d events shown, want at most 4", len(got))
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("pairing took %v", elapsed)
	}
}
