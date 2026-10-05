package subtitle_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestEventTextPrefersOverride(t *testing.T) {
	e := subtitle.Event{OriginalText: "That's sick!", OverrideText: "Que loucura!"}
	if got := e.Text(); got != "Que loucura!" {
		t.Fatalf("Text() = %q, want the override", got)
	}

	e.OverrideText = ""
	if got := e.Text(); got != "That's sick!" {
		t.Fatalf("Text() = %q, want the original", got)
	}
}

func TestEventValid(t *testing.T) {
	cases := []struct {
		name  string
		event subtitle.Event
		want  bool
	}{
		{"regular", event("a", 0, 1), true},
		{"override only", subtitle.Event{Start: 0, End: ms(1), OverrideText: "x"}, true},
		{"zero duration", event("z", 5, 5), false},
		{"inverted", event("i", 6, 5), false},
		{"negative start", event("n", -1, 5), false},
		{"no text", subtitle.Event{Start: 0, End: ms(1)}, false},
	}
	for _, tc := range cases {
		if got := tc.event.Valid(); got != tc.want {
			t.Fatalf("%s: Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEventActiveAtIsHalfOpen(t *testing.T) {
	e := event("a", 1_000, 2_000)
	cases := map[int]bool{999: false, 1_000: true, 1_999: true, 2_000: false}
	for at, want := range cases {
		if got := e.ActiveAt(ms(at)); got != want {
			t.Fatalf("ActiveAt(%dms) = %v, want %v", at, got, want)
		}
	}
}
