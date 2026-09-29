package study_test

import (
	"testing"

	"lingoreel/internal/domain/study"
)

func TestModeValid(t *testing.T) {
	cases := map[study.Mode]bool{
		study.ModeDual: true, study.ModeOnDemand: true, study.ModeAlwaysTranslated: true,
		"": false, "DUAL": false, "on_demand": false, " dual": false,
	}
	for mode, want := range cases {
		if got := mode.Valid(); got != want {
			t.Fatalf("Mode(%q).Valid() = %v, want %v", mode, got, want)
		}
	}
}

func TestRevealBehaviorValid(t *testing.T) {
	cases := map[study.RevealBehavior]bool{
		study.RevealToggle: true, study.RevealHold: true,
		"": false, "Hold": false, "press": false,
	}
	for behavior, want := range cases {
		if got := behavior.Valid(); got != want {
			t.Fatalf("RevealBehavior(%q).Valid() = %v, want %v", behavior, got, want)
		}
	}
}
