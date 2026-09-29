package study

import "testing"

func TestModeValid(t *testing.T) {
	cases := map[Mode]bool{
		ModeDual: true, ModeOnDemand: true, ModeAlwaysTranslated: true,
		"": false, "DUAL": false, "on_demand": false,
	}
	for mode, want := range cases {
		if got := mode.Valid(); got != want {
			t.Fatalf("Mode(%q).Valid() = %v, want %v", mode, got, want)
		}
	}
}

func TestRevealBehaviorValid(t *testing.T) {
	cases := map[RevealBehavior]bool{
		RevealToggle: true, RevealHold: true,
		"": false, "Hold": false, "press": false,
	}
	for behavior, want := range cases {
		if got := behavior.Valid(); got != want {
			t.Fatalf("RevealBehavior(%q).Valid() = %v, want %v", behavior, got, want)
		}
	}
}
