package session_test

import (
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestSubtitlesApplyEachLineOffset(t *testing.T) {
	t.Parallel()
	primary := &session.Track{Timeline: timeline(t, cue("en", 10_000, 12_000))}
	translation := &session.Track{Timeline: timeline(t, cue("pt", 10_500, 12_500)), Offset: subtitle.NewOffset(-ms(500))}
	subtitles := session.NewSubtitles(primary, translation)

	visible := subtitles.At(ms(10_000))
	if !slices.Equal(ids(visible.Primary), []string{"en"}) || !slices.Equal(ids(visible.Translation), []string{"pt"}) {
		t.Fatalf("At(10s) = %v / %v, want both lines", ids(visible.Primary), ids(visible.Translation))
	}
	if visible := subtitles.At(ms(12_000)); len(visible.Primary) != 0 || len(visible.Translation) != 0 {
		t.Fatalf("At(12s) = %v / %v, want nothing (translation ends at 12s with its offset)", ids(visible.Primary), ids(visible.Translation))
	}
}

func TestSubtitlesNextChangeConsidersBothLinesAndOffsets(t *testing.T) {
	t.Parallel()
	primary := &session.Track{Timeline: timeline(t, cue("en", 10_000, 12_000))}
	translation := &session.Track{Timeline: timeline(t, cue("pt", 10_500, 12_500)), Offset: subtitle.NewOffset(-ms(800))}
	subtitles := session.NewSubtitles(primary, translation)

	cases := []struct{ at, want int }{
		{0, 9_700},       // translation starts first thanks to its offset
		{9_700, 10_000},  // then the primary line
		{10_000, 11_700}, // translation ends
		{11_700, 12_000}, // primary ends
	}
	for _, tc := range cases {
		got, ok := subtitles.NextChangeAfter(ms(tc.at))
		if !ok || got != ms(tc.want) {
			t.Fatalf("NextChangeAfter(%dms) = %v,%v; want %dms", tc.at, got, ok, tc.want)
		}
	}
	if _, ok := subtitles.NextChangeAfter(ms(12_000)); ok {
		t.Fatal("a change was reported after the last event")
	}
}

func TestSubtitlesWithMissingLines(t *testing.T) {
	t.Parallel()
	subtitles := session.NewSubtitles(nil, nil)
	if visible := subtitles.At(time.Second); visible.Primary != nil || visible.Translation != nil {
		t.Fatalf("empty session shows %+v", visible)
	}
	if _, ok := subtitles.NextChangeAfter(0); ok {
		t.Fatal("empty session reported a change")
	}
	subtitles.SetOffset(session.LineTranslation, subtitle.NewOffset(time.Second)) // must not panic
	if subtitles.Offset(session.LineTranslation) != 0 {
		t.Fatal("offset stored on a missing line")
	}
}

func TestSetOffsetMovesOnlyThatLine(t *testing.T) {
	t.Parallel()
	primary := &session.Track{Timeline: timeline(t, cue("en", 1_000, 2_000))}
	translation := &session.Track{Timeline: timeline(t, cue("pt", 1_000, 2_000))}
	subtitles := session.NewSubtitles(primary, translation)

	subtitles.SetOffset(session.LineTranslation, subtitle.NewOffset(ms(500)))

	visible := subtitles.At(ms(1_200))
	if len(visible.Primary) != 1 || len(visible.Translation) != 0 {
		t.Fatalf("At(1.2s) = %v / %v; only the translation should be delayed", ids(visible.Primary), ids(visible.Translation))
	}
	if subtitles.Offset(session.LinePrimary) != 0 || subtitles.Offset(session.LineTranslation).Duration() != ms(500) {
		t.Fatal("offsets not stored per line")
	}
	if primary.Offset != 0 || translation.Offset != 0 {
		t.Fatal("SetOffset mutated the caller's track")
	}
}

// A line takes another track while the video plays, gains one (a translation
// added) or loses it, and each change wakes whoever draws the screen.
func TestSetTrackSwitchesALine(t *testing.T) {
	t.Parallel()
	subtitles := session.NewSubtitles(&session.Track{Timeline: timeline(t, cue("en", 1_000, 2_000))}, nil)
	added := &session.Track{Timeline: timeline(t, cue("pt", 1_000, 2_000)), Offset: subtitle.NewOffset(ms(100))}
	subtitles.SetTrack(session.LineTranslation, added)
	select {
	case <-subtitles.Changes():
	default:
		t.Fatal("adding a track signalled no change")
	}
	if visible := subtitles.At(ms(1_500)); !slices.Equal(ids(visible.Translation), []string{"pt"}) {
		t.Fatalf("the added translation shows %v", ids(visible.Translation))
	}
	added.Offset = subtitle.NewOffset(time.Hour)
	if got := subtitles.Track(session.LineTranslation); got == nil || got.Offset.Duration() != ms(100) {
		t.Fatalf("Track = %+v: the line must keep its own copy", got)
	}
	subtitles.SetTrack(session.LinePrimary, &session.Track{Timeline: timeline(t, cue("other", 1_000, 2_000))})
	if visible := subtitles.At(ms(1_500)); !slices.Equal(ids(visible.Primary), []string{"other"}) {
		t.Fatalf("the switched line shows %v", ids(visible.Primary))
	}
	subtitles.SetTrack(session.LineTranslation, nil)
	if subtitles.Track(session.LineTranslation) != nil || len(subtitles.At(ms(1_500)).Translation) != 0 {
		t.Fatal("the translation taken away still shows")
	}
	subtitles.SetTrack(session.Line(7), added) // an unknown line changes nothing
	if subtitles.Track(session.Line(7)) != nil {
		t.Fatal("an unknown line took a track")
	}
}

func TestVisibleEqualComparesIDsAndDisplayedText(t *testing.T) {
	t.Parallel()
	base := session.Visible{Primary: []subtitle.Event{cue("1", 0, 1)}}
	edited := session.Visible{Primary: []subtitle.Event{cue("1", 0, 1)}}
	edited.Primary[0].OverrideText = "edited"

	if !base.Equal(session.Visible{Primary: []subtitle.Event{cue("1", 0, 1)}}) {
		t.Fatal("identical snapshots differ")
	}
	if base.Equal(edited) {
		t.Fatal("an edited text was not noticed")
	}
	if base.Equal(session.Visible{Primary: []subtitle.Event{cue("2", 0, 1)}}) || base.Equal(session.Visible{}) {
		t.Fatal("different events compared equal")
	}
}
