package subtitle

import (
	"slices"
	"testing"
	"time"
)

func TestNegativeOffsetShowsLateTranslationEarlier(t *testing.T) {
	// The translation cue is 500 ms late compared with the speech at 10 s.
	timeline := mustTimeline(t, event("pt", 10_500, 12_500))
	offset := NewOffset(-500 * time.Millisecond)

	got := activeIDs(timeline.ActiveAt(offset.TrackTime(ms(10_000))))
	if !slices.Equal(got, []string{"pt"}) {
		t.Fatalf("with -500ms the cue should be active at 10s, got %v", got)
	}
	if got := timeline.ActiveAt(offset.TrackTime(ms(12_000))); len(got) != 0 {
		t.Fatalf("with -500ms the cue should end at 12s, got %v", activeIDs(got))
	}
}

func TestPositiveOffsetDelaysTrack(t *testing.T) {
	timeline := mustTimeline(t, event("en", 1_000, 2_000))
	offset := NewOffset(150 * time.Millisecond)

	if got := timeline.ActiveAt(offset.TrackTime(ms(1_100))); len(got) != 0 {
		t.Fatalf("cue shown before its delayed start: %v", activeIDs(got))
	}
	if got := timeline.ActiveAt(offset.TrackTime(ms(1_150))); len(got) != 1 {
		t.Fatalf("cue not shown at its delayed start, got %v", activeIDs(got))
	}
}

func TestOffsetRoundTripsBetweenTimeBases(t *testing.T) {
	offset := NewOffset(-350 * time.Millisecond)
	position := 42 * time.Second

	if got := offset.PlaybackTime(offset.TrackTime(position)); got != position {
		t.Fatalf("PlaybackTime(TrackTime(x)) = %v, want %v", got, position)
	}
}

func TestOffsetIsClamped(t *testing.T) {
	if got := NewOffset(time.Hour).Duration(); got != MaxOffset {
		t.Fatalf("NewOffset(1h) = %v, want %v", got, MaxOffset)
	}
	if got := NewOffset(0).Shift(-time.Hour).Duration(); got != -MaxOffset {
		t.Fatalf("Shift(-1h) = %v, want %v", got, -MaxOffset)
	}
	if got := NewOffset(100 * time.Millisecond).Shift(-100 * time.Millisecond); got != 0 {
		t.Fatalf("Shift back to zero = %v", got)
	}
}
