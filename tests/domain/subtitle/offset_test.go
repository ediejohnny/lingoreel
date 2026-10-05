package subtitle_test

import (
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestNegativeOffsetShowsLateTranslationEarlier(t *testing.T) {
	// The translation cue is 500 ms late compared with the speech at 10 s.
	timeline := mustTimeline(t, event("pt", 10_500, 12_500))
	offset := subtitle.NewOffset(-500 * time.Millisecond)

	if got := activeIDs(timeline.ActiveAt(offset.TrackTime(ms(10_000)))); !slices.Equal(got, []string{"pt"}) {
		t.Fatalf("with -500ms the cue should be active at 10s, got %v", got)
	}
	if got := timeline.ActiveAt(offset.TrackTime(ms(9_999))); len(got) != 0 {
		t.Fatalf("with -500ms the cue should not start before 10s, got %v", activeIDs(got))
	}
	if got := timeline.ActiveAt(offset.TrackTime(ms(12_000))); len(got) != 0 {
		t.Fatalf("with -500ms the cue should end at 12s, got %v", activeIDs(got))
	}
}

func TestPositiveOffsetDelaysTrack(t *testing.T) {
	timeline := mustTimeline(t, event("en", 1_000, 2_000))
	offset := subtitle.NewOffset(150 * time.Millisecond)

	if got := timeline.ActiveAt(offset.TrackTime(ms(1_149))); len(got) != 0 {
		t.Fatalf("cue shown before its delayed start: %v", activeIDs(got))
	}
	if got := timeline.ActiveAt(offset.TrackTime(ms(1_150))); len(got) != 1 {
		t.Fatalf("cue not shown at its delayed start, got %v", activeIDs(got))
	}
}

func TestZeroOffsetChangesNothing(t *testing.T) {
	offset := subtitle.NewOffset(0)
	for _, position := range []time.Duration{0, ms(1), -ms(1), 90 * time.Minute} {
		if offset.TrackTime(position) != position || offset.PlaybackTime(position) != position {
			t.Fatalf("zero offset moved %v", position)
		}
	}
}

func TestOffsetRoundTripsBetweenTimeBases(t *testing.T) {
	for _, shift := range []time.Duration{-350 * time.Millisecond, 0, 2 * time.Second} {
		offset := subtitle.NewOffset(shift)
		position := 42 * time.Second
		if got := offset.PlaybackTime(offset.TrackTime(position)); got != position {
			t.Fatalf("offset %v: PlaybackTime(TrackTime(x)) = %v, want %v", shift, got, position)
		}
	}
}

func TestOffsetIsClamped(t *testing.T) {
	cases := []struct {
		name string
		got  subtitle.Offset
		want time.Duration
	}{
		{"far positive", subtitle.NewOffset(time.Hour), subtitle.MaxOffset},
		{"far negative", subtitle.NewOffset(-time.Hour), -subtitle.MaxOffset},
		{"exactly the limit", subtitle.NewOffset(subtitle.MaxOffset), subtitle.MaxOffset},
		{"shift beyond the limit", subtitle.NewOffset(0).Shift(-time.Hour), -subtitle.MaxOffset},
		{"shift back to zero", subtitle.NewOffset(100 * time.Millisecond).Shift(-100 * time.Millisecond), 0},
		{"shift from the limit inward", subtitle.NewOffset(time.Hour).Shift(-time.Second), subtitle.MaxOffset - time.Second},
	}
	for _, tc := range cases {
		if tc.got.Duration() != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, tc.got.Duration(), tc.want)
		}
	}
}
