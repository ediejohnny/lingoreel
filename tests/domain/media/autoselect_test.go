package media_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestAutoSelectableFollowsTheTimingVerdict(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		readable  bool
		candidate bool
		verdict   subtitle.Verdict
		want      bool
	}{
		{"track of the video", true, false, "", true},
		{"unreadable", false, false, "", false},
		{"own file, not measured", true, false, subtitle.VerdictUnknown, true},
		{"own file, match", true, false, subtitle.VerdictMatch, true},
		{"own file, another cut", true, false, subtitle.VerdictOtherCut, false},
		{"own file, no match", true, false, subtitle.VerdictNoMatch, false},
		{"candidate, match", true, true, subtitle.VerdictMatch, true},
		{"candidate, not measured", true, true, "", false},
		{"candidate, unknown", true, true, subtitle.VerdictUnknown, false},
		{"candidate, another cut", true, true, subtitle.VerdictOtherCut, false},
		{"unreadable candidate, match", false, true, subtitle.VerdictMatch, false},
	}
	for _, c := range cases {
		track := media.Track{Readable: c.readable, Candidate: c.candidate, Alignment: subtitle.Alignment{Verdict: c.verdict}}
		if got := track.AutoSelectable(); got != c.want {
			t.Errorf("%s: AutoSelectable = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSelectionSkipsFilesForAnotherCut(t *testing.T) {
	t.Parallel()
	info := media.Info{Tracks: []media.Track{
		{Number: 1, Type: media.TrackSubtitle, Language: "en", Readable: true, Path: `x\DC.srt`,
			Alignment: subtitle.Alignment{Verdict: subtitle.VerdictOtherCut}},
		{Number: 2, Type: media.TrackSubtitle, Language: "en", Readable: true, Path: `x\sub\cinema.srt`, Candidate: true,
			Alignment: subtitle.Alignment{Verdict: subtitle.VerdictMatch}},
		{Number: 3, Type: media.TrackSubtitle, Language: "pt-BR", Readable: true, Path: `x\sub\pt.srt`, Candidate: true},
	}}
	got := media.SelectTracks(info, media.Preferences{Primary: "en", Translation: "pt-BR"})
	if want := (media.Selection{Primary: 2}); got != want {
		t.Fatalf("SelectTracks = %+v, want %+v", got, want)
	}
}
