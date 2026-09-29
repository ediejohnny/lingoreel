package media_test

import (
	"testing"

	"lingoreel/internal/domain/media"
)

var studyPreferences = media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"}

func subtitleTrack(number uint64, language media.LanguageTag) media.Track {
	return media.Track{Number: number, Type: media.TrackSubtitle, Language: language, Readable: true}
}

func audioTrack(number uint64, language media.LanguageTag, isDefault bool) media.Track {
	return media.Track{Number: number, Type: media.TrackAudio, Language: language, Default: isDefault}
}

func forced(track media.Track) media.Track          { track.Forced = true; return track }
func asDefault(track media.Track) media.Track       { track.Default = true; return track }
func hearingImpaired(track media.Track) media.Track { track.HearingImpaired = true; return track }
func unreadable(track media.Track) media.Track      { track.Readable = false; return track }

func info(tracks ...media.Track) media.Info {
	return media.Info{Tracks: tracks}
}

func assertSelection(t *testing.T, got, want media.Selection) {
	t.Helper()
	if got != want {
		t.Fatalf("SelectTracks = %+v, want %+v", got, want)
	}
}

func TestSelectTracksTypicalStudyFile(t *testing.T) {
	file := info(
		media.Track{Number: 1, Type: media.TrackVideo},
		audioTrack(2, "pt-BR", true),
		audioTrack(3, "en", false),
		subtitleTrack(4, "es"),
		subtitleTrack(5, "pt-BR"),
		subtitleTrack(6, "en"),
	)
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Audio: 3, Primary: 6, Translation: 5})
}

func TestForcedTracksLoseAgainstFullTracksOfTheSameLanguage(t *testing.T) {
	file := info(
		asDefault(forced(subtitleTrack(1, "en"))),
		subtitleTrack(2, "en"),
		forced(subtitleTrack(3, "pt-BR")),
		subtitleTrack(4, "pt-BR"),
	)
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2, Translation: 4})
}

func TestTranslationRegionPreference(t *testing.T) {
	cases := []struct {
		name   string
		tracks []media.Track
		want   uint64
	}{
		{"exact region wins", []media.Track{subtitleTrack(1, "pt-PT"), subtitleTrack(2, "pt"), subtitleTrack(3, "pt-BR")}, 3},
		{"no region beats other region", []media.Track{subtitleTrack(1, "pt-PT"), subtitleTrack(2, "pt")}, 2},
		{"other region is still Portuguese", []media.Track{subtitleTrack(1, "pt-PT")}, 1},
		{"never another language", []media.Track{subtitleTrack(1, "es"), subtitleTrack(2, "gl")}, 0},
		{"unknown language is not Portuguese", []media.Track{subtitleTrack(1, "")}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracks := append([]media.Track{subtitleTrack(10, "en")}, tc.tracks...)
			if got := media.SelectTracks(info(tracks...), studyPreferences).Translation; got != tc.want {
				t.Fatalf("Translation = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPrimaryFallsBackToFirstReadableNonForcedTrack(t *testing.T) {
	file := info(
		forced(subtitleTrack(1, "es")),
		subtitleTrack(2, "fr"),
		subtitleTrack(3, "pt-BR"),
	)
	// Track 1 is forced, so the first readable non-forced track is 2.
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2, Translation: 3})
}

func TestPrimaryUsesForcedTrackWhenItIsTheOnlyOption(t *testing.T) {
	file := info(forced(subtitleTrack(7, "es")))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 7})
}

func TestUnreadableTracksAreNeverSelected(t *testing.T) {
	file := info(
		unreadable(subtitleTrack(1, "en")), // e.g. PGS, needs OCR
		subtitleTrack(2, "pt-BR"),
	)
	// The only readable track becomes the primary line, so no translation remains.
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2})
}

func TestRegularTrackBeatsHearingImpairedAndDefaultBreaksTies(t *testing.T) {
	sdh := info(hearingImpaired(subtitleTrack(1, "en")), subtitleTrack(2, "en"))
	assertSelection(t, media.SelectTracks(sdh, studyPreferences), media.Selection{Primary: 2})

	tie := info(subtitleTrack(1, "en"), asDefault(subtitleTrack(2, "en")))
	assertSelection(t, media.SelectTracks(tie, studyPreferences), media.Selection{Primary: 2})

	regional := info(subtitleTrack(1, "en-US"), subtitleTrack(2, "en-GB"))
	assertSelection(t, media.SelectTracks(regional, studyPreferences), media.Selection{Primary: 1})
}

func TestTranslationNeverReusesThePrimaryTrack(t *testing.T) {
	sameLanguage := media.Preferences{Primary: "en", Translation: "en"}

	one := info(subtitleTrack(1, "en"))
	assertSelection(t, media.SelectTracks(one, sameLanguage), media.Selection{Primary: 1})

	two := info(subtitleTrack(1, "en"), subtitleTrack(2, "en"))
	assertSelection(t, media.SelectTracks(two, sameLanguage), media.Selection{Primary: 1, Translation: 2})
}

func TestNoTranslationWanted(t *testing.T) {
	file := info(subtitleTrack(1, "en"), subtitleTrack(2, "pt-BR"))
	got := media.SelectTracks(file, media.Preferences{Audio: "en", Primary: "en"})
	assertSelection(t, got, media.Selection{Primary: 1})
}

func TestAudioFallbacks(t *testing.T) {
	cases := []struct {
		name   string
		tracks []media.Track
		want   uint64
	}{
		{"language match beats default", []media.Track{audioTrack(1, "pt-BR", true), audioTrack(2, "en", false)}, 2},
		{"default when no language matches", []media.Track{audioTrack(1, "ja", false), audioTrack(2, "ko", true)}, 2},
		{"first when nothing is default", []media.Track{audioTrack(5, "ja", false), audioTrack(6, "ko", false)}, 5},
		{"default breaks a language tie", []media.Track{audioTrack(1, "en", false), audioTrack(2, "en", true)}, 2},
		{"no audio at all", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := media.SelectTracks(info(tc.tracks...), studyPreferences).Audio; got != tc.want {
				t.Fatalf("Audio = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEmptyFileSelectsNothing(t *testing.T) {
	assertSelection(t, media.SelectTracks(media.Info{}, studyPreferences), media.Selection{})
}

func TestInfoLookups(t *testing.T) {
	file := info(audioTrack(2, "en", true), subtitleTrack(3, "en"), audioTrack(4, "pt", false))

	if got := file.TracksOf(media.TrackAudio); len(got) != 2 || got[0].Number != 2 || got[1].Number != 4 {
		t.Fatalf("TracksOf(audio) = %+v", got)
	}
	if got := file.TracksOf(media.TrackVideo); len(got) != 0 {
		t.Fatalf("TracksOf(video) = %+v, want none", got)
	}
	if track, ok := file.Track(3); !ok || track.Type != media.TrackSubtitle {
		t.Fatalf("Track(3) = %+v, %v", track, ok)
	}
	if _, ok := file.Track(99); ok {
		t.Fatal("Track(99) found a track that does not exist")
	}
}
