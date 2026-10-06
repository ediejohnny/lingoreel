package media_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	guessed := func(track media.Track) media.Track { track.LanguageGuessed = true; return track }
	file := info(asDefault(guessed(subtitleTrack(1, "en"))), subtitleTrack(2, "en-US"))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2})

	alone := info(guessed(subtitleTrack(1, "en")))
	assertSelection(t, media.SelectTracks(alone, studyPreferences), media.Selection{Primary: 1})

	audio := info(asDefault(guessed(audioTrack(1, "en", true))), audioTrack(2, "en", false))
	assertSelection(t, media.SelectTracks(audio, studyPreferences), media.Selection{Audio: 2})
}

func TestTranslationRegionPreference(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	file := info(
		forced(subtitleTrack(1, "es")),
		subtitleTrack(2, "fr"),
		subtitleTrack(3, "pt-BR"),
	)
	// Track 1 is forced, so the first readable non-forced track is 2.
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 2, Translation: 3})
}

func TestPrimaryUsesForcedTrackWhenItIsTheOnlyOption(t *testing.T) {
	t.Parallel()
	file := info(forced(subtitleTrack(7, "es")))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 7})
}

func TestUnreadableTracksAreNeverSelected(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	file := info(
		asDefault(named(subtitleTrack(4, "pt"), "Forced SRT")),
		named(subtitleTrack(5, "pt"), "Complet SRT"),
	)
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Translation: 5})
}

func TestForcedByNameLosesToTheFullTrack(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	file := info(subtitleTrack(1, "pt-BR"), named(subtitleTrack(2, "es"), "Forced"), subtitleTrack(3, "fr"))
	assertSelection(t, media.SelectTracks(file, studyPreferences), media.Selection{Primary: 3, Translation: 1})

	onlyForced := info(subtitleTrack(1, "pt-BR"), forced(subtitleTrack(2, "es")))
	assertSelection(t, media.SelectTracks(onlyForced, studyPreferences), media.Selection{Primary: 2, Translation: 1})
}

func TestRegularTrackBeatsHearingImpairedAndDefaultBreaksTies(t *testing.T) {
	t.Parallel()
	sdh := info(hearingImpaired(subtitleTrack(1, "en")), subtitleTrack(2, "en"))
	assertSelection(t, media.SelectTracks(sdh, studyPreferences), media.Selection{Primary: 2})

	tie := info(subtitleTrack(1, "en"), asDefault(subtitleTrack(2, "en")))
	assertSelection(t, media.SelectTracks(tie, studyPreferences), media.Selection{Primary: 2})

	regional := info(subtitleTrack(1, "en-US"), subtitleTrack(2, "en-GB"))
	assertSelection(t, media.SelectTracks(regional, studyPreferences), media.Selection{Primary: 1})
}

// file is a subtitle file next to the video, measured against its own
// subtitle with that verdict, share of lines lined up and speed.
func file(number uint64, language media.LanguageTag, verdict subtitle.Verdict, score, rate float64) media.Track {
	track := subtitleTrack(media.ExternalTrackBase+number, language)
	track.Path = fmt.Sprintf(`D:\Filmes\%d.srt`, number)
	track.Alignment = subtitle.Alignment{Verdict: verdict, Score: score, Rate: rate}
	return track
}

// Of several files in one language, the one that lines up best with the
// video's own subtitle comes first, not the first one found. A file timed
// for another frame rate (25 against 23.976 fps) can be corrected, but loses
// ten points; a file nothing could measure comes after the measured ones.
func TestTheFileThatFitsTheVideoBestComesFirst(t *testing.T) {
	t.Parallel()
	inside := subtitleTrack(4, "pt-BR")
	otherRate := file(1, "en", subtitle.VerdictMatch, 0.71, 25/(24000.0/1001))
	amazon := file(2, "en", subtitle.VerdictMatch, 0.79, 0.99999)
	bluray := file(3, "en", subtitle.VerdictMatch, 0.86, 1)
	assertSelection(t, media.SelectTracks(info(inside, otherRate, amazon, bluray), studyPreferences),
		media.Selection{Primary: bluray.Number, Translation: 4})

	closeBehind := file(5, "en", subtitle.VerdictMatch, 0.85, 25/(24000.0/1001)) // 75 points against 79
	assertSelection(t, media.SelectTracks(info(inside, closeBehind, amazon), studyPreferences),
		media.Selection{Primary: amazon.Number, Translation: 4})
	farAhead := file(7, "en", subtitle.VerdictMatch, 0.95, 25/(24000.0/1001)) // 85 points against 79
	assertSelection(t, media.SelectTracks(info(inside, amazon, farAhead), studyPreferences),
		media.Selection{Primary: farAhead.Number, Translation: 4})

	unmeasured := file(6, "en", subtitle.VerdictUnknown, 0, 1)
	assertSelection(t, media.SelectTracks(info(inside, unmeasured, amazon), studyPreferences),
		media.Selection{Primary: amazon.Number, Translation: 4})
}

// A subtitle inside the video was made for it: it comes before any file.
func TestASubtitleInsideTheVideoComesBeforeAnyFile(t *testing.T) {
	t.Parallel()
	perfect := file(1, "en", subtitle.VerdictMatch, 1, 1)
	assertSelection(t, media.SelectTracks(info(perfect, subtitleTrack(3, "en")), studyPreferences), media.Selection{Primary: 3})
}

// The lists show each line's best choice first, the one SelectTracks picks:
// the primary language's tracks from the best, then the translation's, then
// the rest as found; a track the app would never pick closes its language.
func TestTheListsShowEachLinesBestChoiceFirst(t *testing.T) {
	t.Parallel()
	spanish := file(1, "es", subtitle.VerdictMatch, 0.9, 1)
	otherRate := file(2, "en", subtitle.VerdictMatch, 0.71, 25/(24000.0/1001))
	portuguese := file(3, "pt", subtitle.VerdictMatch, 0.8, 1)
	broken := unreadable(file(4, "en", subtitle.VerdictMatch, 0.99, 1))
	bluray := file(5, "en", subtitle.VerdictMatch, 0.86, 1)
	inside := subtitleTrack(6, "pt-BR")
	tracks := []media.Track{spanish, otherRate, portuguese, broken, bluray, inside}

	var order []uint64
	for _, track := range media.RankSubtitles(tracks, studyPreferences) {
		order = append(order, track.Number)
	}
	if want := []uint64{bluray.Number, otherRate.Number, broken.Number, inside.Number, portuguese.Number, spanish.Number}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	assertSelection(t, media.SelectTracks(info(tracks...), studyPreferences), media.Selection{Primary: bluray.Number, Translation: inside.Number})
}

func TestTranslationNeverReusesThePrimaryTrack(t *testing.T) {
	t.Parallel()
	sameLanguage := media.Preferences{Primary: "en", Translation: "en"}

	one := info(subtitleTrack(1, "en"))
	assertSelection(t, media.SelectTracks(one, sameLanguage), media.Selection{Primary: 1})

	two := info(subtitleTrack(1, "en"), subtitleTrack(2, "en"))
	assertSelection(t, media.SelectTracks(two, sameLanguage), media.Selection{Primary: 1, Translation: 2})
}

func TestNoTranslationWanted(t *testing.T) {
	t.Parallel()
	file := info(subtitleTrack(1, "en"), subtitleTrack(2, "pt-BR"))
	got := media.SelectTracks(file, media.Preferences{Audio: "en", Primary: "en"})
	assertSelection(t, got, media.Selection{Primary: 1})
}

func TestAudioFallbacks(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	assertSelection(t, media.SelectTracks(media.Info{}, studyPreferences), media.Selection{})
}

func TestInfoLookups(t *testing.T) {
	t.Parallel()
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

// The voice followed is the film's track in the studied line's language:
// a dubbed track first in the file and a later commentary do not count.
func TestTheVoiceTrackSpeaksTheLinesLanguage(t *testing.T) {
	t.Parallel()
	file := info(
		media.Track{Number: 1, Type: media.TrackVideo},
		audioTrack(2, "pt-BR", true),
		audioTrack(3, "en", false),
		named(audioTrack(4, "en", false), "Commentary"),
		subtitleTrack(5, "en"),
	)
	file.Tracks[3].LanguageGuessed = true
	if track, position, found := media.VoiceTrack(file, "en"); !found || track.Number != 3 || position != 1 {
		t.Fatalf("English voice: track %d at %d (found %v), want track 3 at 1", track.Number, position, found)
	}
	if _, _, found := media.VoiceTrack(file, "fr"); found {
		t.Fatal("found a French voice in a film without one")
	}
}
