package voice_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// A stretch heard to choose the subtitle comes back from its own start,
// judged with the second before it: a voice going on when the stretch
// begins is voice from its first frames.
func TestAStretchIsHeardWithTheSecondBeforeIt(t *testing.T) {
	t.Parallel()
	sound := testkit.Then(testkit.Silence(9800*ms), testkit.Voiced(2*time.Second, -20, 41), testkit.Silence(5*time.Second))
	var done []float64
	heard, err := voice.Hear(context.Background(), decoder{sound: sound}, warmingModel{},
		voice.Stretch{File: "film.mkv", From: 10 * time.Second, Length: 5 * time.Second}, func(share float64) { done = append(done, share) })
	if err != nil {
		t.Fatal(err)
	}
	if len(done) < 10 || !slices.IsSorted(done) || done[len(done)-1] < 0.95 {
		t.Fatalf("progress told %v, want it rising to the end", done)
	}
	if heard.Start != 10*time.Second || len(heard.Frames) != 500 {
		t.Fatalf("heard %d frames from %v, want 500 from 10s", len(heard.Frames), heard.Start)
	}
	if speech := heard.Frames[10].Speech; speech != 1 { // 10.1 s
		t.Fatalf("at 10.1 s the speech is %v, want the voice heard since 9.8 s", speech)
	}
}
