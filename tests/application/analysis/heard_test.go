package analysis_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// A language heard in an audio track is as good as one its file states: the
// track heard in English wins the English audio over one stated British
// English, which a guessed English never does.
func TestALanguageHeardCountsAsStated(t *testing.T) {
	t.Parallel()
	info := media.Info{Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo},
		{Number: 2, Type: media.TrackAudio, Language: "en", LanguageGuessed: true},
		{Number: 3, Type: media.TrackAudio, Language: "en-GB"},
	}}
	preferences := media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"}
	before := analysis.Analysis{Info: info, Suggested: media.SelectTracks(info, preferences)}
	if before.Suggested.Audio != 3 {
		t.Fatalf("before hearing, audio %d: the premise is the stated British track winning over a guessed English", before.Suggested.Audio)
	}

	heard := before.WithHeardLanguages(map[uint64]media.LanguageTag{2: "en", 3: "pt"}, preferences)
	if heard.Suggested.Audio != 2 {
		t.Errorf("after hearing, audio %d, want the track heard in English", heard.Suggested.Audio)
	}
	track, _ := heard.Info.Track(2)
	if track.Language != "en" || track.LanguageGuessed || !track.LanguageHeard {
		t.Errorf("the track heard: %+v, want English, heard, not guessed", track)
	}
	if stated, _ := heard.Info.Track(3); stated.Language != "en-GB" || stated.LanguageHeard {
		t.Errorf("a stated track was changed by a hearing: %+v", stated)
	}
	if original, _ := before.Info.Track(2); original.LanguageHeard {
		t.Error("the analysis heard was changed in place")
	}
}
