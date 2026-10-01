package controlbar_test

import (
	"testing"
	"time"

	"lingoreel/internal/domain/controlbar"
)

func TestVisibility(t *testing.T) {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var v controlbar.Visibility
	if shown, recheck := v.Update(start, false); shown || recheck != 0 {
		t.Fatalf("never wanted: shown %v recheck %v", shown, recheck)
	}
	if shown, recheck := v.Update(start, true); !shown || recheck != 0 {
		t.Fatalf("wanted: shown %v recheck %v", shown, recheck)
	}
	// The mouse leaves: it stays HideDelay (2 s) and asks to be checked again.
	if shown, recheck := v.Update(start.Add(500*time.Millisecond), false); !shown || recheck != 1500*time.Millisecond {
		t.Fatalf("after leaving: shown %v recheck %v", shown, recheck)
	}
	if shown, recheck := v.Update(start.Add(controlbar.HideDelay-time.Nanosecond), false); !shown || recheck != time.Nanosecond {
		t.Fatalf("1 ns before the delay: shown %v recheck %v", shown, recheck)
	}
	if shown, _ := v.Update(start.Add(controlbar.HideDelay), false); shown || v.Shown() {
		t.Fatal("still shown when the delay is over")
	}
	// Coming back before the delay starts it again from the last activity.
	v.Update(start, true)
	v.Update(start.Add(1900*time.Millisecond), true)
	if shown, recheck := v.Update(start.Add(3*time.Second), false); !shown || recheck != 900*time.Millisecond {
		t.Fatalf("delay counted from the last activity: shown %v recheck %v", shown, recheck)
	}
	v.Hide()
	if shown, _ := v.Update(start.Add(3*time.Second), false); shown {
		t.Fatal("Hide did not take the bar away")
	}
}
