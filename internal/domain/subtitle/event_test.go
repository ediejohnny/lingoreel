package subtitle

import "testing"

func TestEventTextPrefersOverride(t *testing.T) {
	e := Event{OriginalText: "That's sick!", OverrideText: "Que loucura!"}
	if got := e.Text(); got != "Que loucura!" {
		t.Fatalf("Text() = %q, want the override", got)
	}

	e.OverrideText = ""
	if got := e.Text(); got != "That's sick!" {
		t.Fatalf("Text() = %q, want the original", got)
	}
}
