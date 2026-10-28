package analysis_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// A hearing listens to the audio track that speaks the line on top's
// language, the film's own and not a dub; without one, to the first audio
// track, with its channels and codec too, so the decoder reads it as it is.
func TestAHearingListensToTheFilmsOwnVoice(t *testing.T) {
	t.Parallel()
	film := analysis.Analysis{Path: `C:\film.mkv`, Info: media.Info{Tracks: []media.Track{
		{Number: 1, Type: media.TrackVideo},
		{Number: 2, Type: media.TrackAudio, Language: "pt-BR", Channels: 6, Codec: "ac3"},
		{Number: 3, Type: media.TrackAudio, Language: "en", Channels: 2, Codec: "aac"},
	}}}
	for _, tc := range []struct {
		language        media.LanguageTag
		track, channels int
		codec           string
	}{
		{"en", 1, 2, "aac"},
		{"fr", 0, 6, "ac3"},
	} {
		stretch := analysis.VoiceStretch(film, tc.language)
		if stretch.File != film.Path || stretch.Track != tc.track || stretch.Channels != tc.channels || stretch.Codec != tc.codec {
			t.Errorf("heard in %q: %+v, want audio track %d with %d channels of %s", tc.language, stretch, tc.track, tc.channels, tc.codec)
		}
	}
	unread := analysis.Analysis{Path: `C:\film.avi`}
	if stretch := analysis.VoiceStretch(unread, "en"); stretch.Track != 0 || stretch.Channels != 0 || stretch.Codec != "" {
		t.Errorf("a video whose tracks are not read: %+v, want its first audio as the decoder finds it", stretch)
	}
}

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
