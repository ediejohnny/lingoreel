package media_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
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

// A forced track only carries the foreign-language lines, so a full track in
// a less exact region still serves better: pt-BR forced scored 307 against
// 257 for a full "pt" track when the region outweighed "not forced".
func TestForcedTracksLoseEvenAgainstALessExactRegion(t *testing.T) {
	file := info(
		forced(subtitleTrack(1, "pt-BR")),
		subtitleTrack(2, "pt"),
		subtitleTrack(3, "pt-PT"),
		forced(subtitleTrack(4, "en-US")),
		subtitleTrack(5, "en-GB"),
	)
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 5, Translation: 2})

	onlyForced := info(forced(subtitleTrack(1, "pt-BR")), subtitleTrack(2, "en"))
	assertSelection(t, media.SelectTracks(onlyForced, studyPreferences), media.Selection{Primary: 2, Translation: 1})
}

// A language nobody declared (the Matroska default "eng") loses against a
// declared one, and still serves when it is the only candidate.
func TestGuessedLanguagesLoseAgainstDeclaredOnes(t *testing.T) {
	guessed := func(track media.Track) media.Track { track.LanguageGuessed = true; return track }
	file := info(asDefault(guessed(subtitleTrack(1, "en"))), subtitleTrack(2, "en-US"))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2})

	alone := info(guessed(subtitleTrack(1, "en")))
	assertSelection(t, media.SelectTracks(alone, studyPreferences), media.Selection{Primary: 1})

	audio := info(asDefault(guessed(audioTrack(1, "en", true))), audioTrack(2, "en", false))
	assertSelection(t, media.SelectTracks(audio, studyPreferences), media.Selection{Audio: 2})
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
	// The only readable track is in the translation's language: it is the
	// translation, and the primary line stays empty rather than repeat it.
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Translation: 2})
}

func named(track media.Track, name string) media.Track {
	track.Name = name
	return track
}

// A typical release: two Portuguese text tracks, the
// default one named "Forced SRT" without the forced flag, and no English.
func TestMovieWithOnlyPortugueseKeepsThePrimaryEmpty(t *testing.T) {
	file := info(
		asDefault(named(subtitleTrack(4, "pt"), "Forced SRT")),
		named(subtitleTrack(5, "pt"), "Complet SRT"),
	)
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Translation: 5})
}

func TestForcedByNameLosesToTheFullTrack(t *testing.T) {
	for _, name := range []string{"English (Forced)", "FORCED", "Legendas Forçadas", "forcadas"} {
		file := info(asDefault(named(subtitleTrack(1, "en"), name)), subtitleTrack(2, "en"))
		if got := media.SelectTracks(file, studyPreferences).Primary; got != 2 {
			t.Errorf("name %q: primary = %d, want the full track 2", name, got)
		}
	}
	// A name that merely contains "for" is not forced.
	file := info(named(subtitleTrack(1, "en"), "For the hearing"), subtitleTrack(2, "en"))
	if got := media.SelectTracks(file, studyPreferences).Primary; got != 1 {
		t.Errorf("primary = %d, want 1 (first of two equal tracks)", got)
	}
}

// Without English, the primary still falls back to another language, as long
// as it is not the translation's.
func TestPrimaryFallsBackToAnotherLanguageButNotTheTranslations(t *testing.T) {
	file := info(subtitleTrack(1, "pt-BR"), named(subtitleTrack(2, "es"), "Forced"), subtitleTrack(3, "fr"))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 3, Translation: 1})

	onlyForced := info(subtitleTrack(1, "pt-BR"), forced(subtitleTrack(2, "es")))
	assertSelection(t, media.SelectTracks(onlyForced, studyPreferences), media.Selection{Primary: 2, Translation: 1})
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
