package app_test

import (
	"errors"
	"strings"
	"testing"

	"lingoreel/internal/presentation/app"
)

// The log tells what happened in a session, and never what the lines say.
func TestTheLogTellsTheSessionNotTheLines(t *testing.T) {
	r := startWatching(t) // Movie.mkv, English audio and lines on both tracks
	onTheLine(t, r)
	if err := errors.Join(r.api.SelectLine("primary"), r.api.EditLine(app.EditRequest{Text: "Segredo de estudo.", Start: "0:01", End: "0:02"})); err != nil {
		t.Fatal(err)
	}
	r.api.StopWatching()
	eventually(t, "the end in the log", func() bool { return strings.Contains(r.log.String(), "session ended") })

	log := r.log.String()
	for _, want := range []string{
		`msg="video opened" name=Movie.mkv`,
		`msg="session started" player=fake audio="3 en A_DTS" primary="4 en `, `translation="5 pt-BR `, "mode=on-demand",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the log lacks %q:\n%s", want, log)
		}
	}
	for _, secret := range []string{"Hi.", "Segredo de estudo"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log holds the line %q:\n%s", secret, log)
		}
	}
}
