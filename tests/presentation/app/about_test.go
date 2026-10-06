package app_test

import (
	"strings"
	"testing"
)

func TestTheAboutCardTellsTheVersionAndTheNotices(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.attach(t)
	if got := r.api.About().Version; got != "0.13.0-test" {
		t.Errorf("version = %q, want 0.13.0-test", got)
	}
	// The notices cover Go, the modules, the voice model and the screens'
	// packages, each under its name and with its license as written.
	notices := r.api.ThirdPartyNotices()
	for _, covered := range []string{
		"\nGo go1.",
		"\ngithub.com/wailsapp/wails/v2 v",
		"\nSilero VAD (silero_vad_16k_op15.onnx) v6.2.1\n",
		"\nreact ",
		"Permission is hereby granted",
	} {
		if !strings.Contains(notices, covered) {
			t.Errorf("the notices lack %q", covered)
		}
	}
}
