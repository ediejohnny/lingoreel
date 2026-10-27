package session_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// What the voice adds moves the line on the screen, on top of the user's own
// offset, but the line's offset, the one kept with the video and told in
// the notices, stays the user's: a sync step moves only it.
func TestTheVoicesShiftIsShownButNeverTheUsersOffset(t *testing.T) {
	t.Parallel()
	line := subtitle.Event{ID: "L", Start: ms(10000), End: ms(12000), OriginalText: "line"}
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, line), Offset: subtitle.NewOffset(ms(200))}, nil)
	subtitles.SetVoiceShift(session.LinePrimary, ms(800))

	if got := subtitles.Offset(session.LinePrimary); got != subtitle.NewOffset(ms(200)) {
		t.Fatalf("the user's offset is %v with the voice's shift, want 200 ms", got)
	}
	if got := subtitles.Shown(session.LinePrimary); got != subtitle.NewOffset(ms(1000)) {
		t.Fatalf("the line is shown at %v, want 200 ms + 800 ms", got)
	}
	// The line's 10 s to 12 s, shown 200 ms + 800 ms later: 11 s to 13 s.
	shows := func(at time.Duration) bool { return len(subtitles.At(at).Primary) == 1 }
	if shows(ms(10900)) || !shows(ms(11000)) || !shows(ms(12900)) || shows(ms(13000)) {
		t.Fatal("the line is not on the screen from 11 s to 13 s, its time plus both offsets")
	}

	offset, _ := subtitles.ShiftOffset(session.LinePrimary, ms(100))
	if offset != subtitle.NewOffset(ms(300)) || subtitles.Shown(session.LinePrimary) != subtitle.NewOffset(ms(1100)) {
		t.Fatalf("a sync step gave %v, shown at %v; want 300 ms, shown at 1100 ms", offset, subtitles.Shown(session.LinePrimary))
	}
	subtitles.SetVoiceShift(session.LinePrimary, 0)
	if subtitles.Shown(session.LinePrimary) != subtitles.Offset(session.LinePrimary) {
		t.Fatal("the voice's shift taken away left the line elsewhere than the user's offset")
	}
}
