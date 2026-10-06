package media

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Preferences are the languages the user wants, as BCP 47 tags. An empty
// Translation means no translation line.
type Preferences struct {
	Audio       LanguageTag
	Primary     LanguageTag
	Translation LanguageTag
}

// Selection holds the chosen track numbers; zero means no track. It is only a
// suggestion: the user can always pick other tracks.
type Selection struct {
	Audio       uint64
	Primary     uint64
	Translation uint64
}

// Weights of the audio score, compared as tiers: a higher tier always
// outweighs every lower one combined. Tracks of another language never take
// part; among the rest, a declared language, then a more exact region, then
// the default flag win.
const (
	declaredWeight    = 1_000
	languageWeight    = 100 // times the match level, at most 3
	defaultFlagWeight = 5
)

// Fit classes of a subtitle track: one inside the video was made for it; a
// file lined up with the video's own subtitle comes next; a file nothing
// could measure comes last.
const (
	fitUnmeasured = iota
	fitLinedUp
	fitInside
)

const (
	// otherSpeed is the least speed correction of a file timed for another
	// frame rate: 23.976 against 24 fps is 0.1%; the drift a fit corrects
	// along a film is far less.
	otherSpeed = 0.0005
	// otherSpeedPenalty is what a file timed for another frame rate loses, in
	// points of lines lined up: it can be corrected, but one timed for this
	// video fits more surely.
	otherSpeedPenalty = 10
)

// Language match levels.
const (
	noLanguageMatch   = 0
	sameBaseLanguage  = 1 // pt-PT for pt-BR
	unspecificRegion  = 2 // pt for pt-BR, or en-US for en
	exactLanguageTags = 3 // pt-BR for pt-BR
)

// SelectTracks picks the audio, primary subtitle and translation tracks
// following the fallbacks of the product rules:
//   - primary: best language match, else the first readable subtitle in a
//     language other than the translation's (both lines in the same language
//     would repeat each other, so the primary stays empty instead);
//   - translation: best language match only, never another language;
//   - audio: best language match, else the default track, else the first.
func SelectTracks(info Info, preferences Preferences) Selection {
	var selection Selection
	selection.Audio = selectAudio(info.TracksOf(TrackAudio), preferences.Audio)

	subtitles := selectableSubtitles(info.TracksOf(TrackSubtitle))
	selection.Primary = selectSubtitle(subtitles, preferences.Primary, 0)
	if selection.Primary == 0 {
		selection.Primary = fallbackSubtitle(subtitles, preferences.Translation)
	}
	if preferences.Translation != "" {
		selection.Translation = selectSubtitle(subtitles, preferences.Translation, selection.Primary)
	}
	return selection
}

// fallbackSubtitle is the primary line when no track speaks its language:
// the first readable track, non-forced first, that is not in the
// translation's language.
func fallbackSubtitle(tracks []Track, translation LanguageTag) uint64 {
	for _, wantForced := range []bool{false, true} {
		for _, track := range tracks {
			if isForced(track) == wantForced && languageMatch(track.Language, translation) == noLanguageMatch {
				return track.Number
			}
		}
	}
	return 0
}

func selectableSubtitles(tracks []Track) []Track {
	var selectable []Track
	for _, track := range tracks {
		if track.AutoSelectable() {
			selectable = append(selectable, track)
		}
	}
	return selectable
}

// selectSubtitle returns the best ranked track in the wanted language other
// than excluded, or zero; of two equal tracks, the first.
func selectSubtitle(tracks []Track, want LanguageTag, excluded uint64) uint64 {
	best, bestRank, found := uint64(0), subtitleRank{}, false
	for _, track := range tracks {
		if track.Number == excluded {
			continue
		}
		match := languageMatch(track.Language, want)
		if match == noLanguageMatch {
			continue
		}
		if rank := rankSubtitle(track, match); !found || rank.beats(bestRank) {
			best, bestRank, found = track.Number, rank, true
		}
	}
	return best
}

// RankSubtitles orders a video's subtitle tracks for the lists, so the app's
// best choice for each line comes first: the primary language's tracks from
// the best, then the translation's, then the rest as found. Within a
// language, the tracks the app would never pick on its own (unreadable,
// another cut) come last. The user can still pick any of them.
func RankSubtitles(tracks []Track, preferences Preferences) []Track {
	wanted := func(track Track) (LanguageTag, int) {
		switch {
		case languageMatch(track.Language, preferences.Primary) != noLanguageMatch:
			return preferences.Primary, 0
		case languageMatch(track.Language, preferences.Translation) != noLanguageMatch:
			return preferences.Translation, 1
		}
		return "", 2
	}
	ranked := slices.Clone(tracks)
	slices.SortStableFunc(ranked, func(a, b Track) int {
		language, group := wanted(a)
		if _, other := wanted(b); group != other {
			return cmp.Compare(group, other)
		}
		if language == "" {
			return 0
		}
		if pickA, pickB := a.AutoSelectable(), b.AutoSelectable(); pickA != pickB {
			if pickA {
				return -1
			}
			return 1
		}
		rankA, rankB := rankSubtitle(a, languageMatch(a.Language, language)), rankSubtitle(b, languageMatch(b.Language, language))
		switch {
		case rankA.beats(rankB):
			return -1
		case rankB.beats(rankA):
			return 1
		}
		return 0
	})
	return ranked
}

// subtitleRank orders the subtitle tracks of a language, compared field by
// field from the first: an earlier field always outweighs every later one.
type subtitleRank [7]int

// rankSubtitle places a track of the wanted language, whose language match
// level is match. A forced track loses first, because it only carries the
// lines spoken in a foreign language; then a language nobody declared; then a
// less exact region; then how well the track fits the video (subtitleFit);
// then the default flag, then the hearing-impaired one.
func rankSubtitle(track Track, match int) subtitleRank {
	class, points := subtitleFit(track)
	return subtitleRank{
		boolRank(!isForced(track)), boolRank(!track.LanguageGuessed), match,
		class, points, boolRank(track.Default), boolRank(!track.HearingImpaired),
	}
}

// beats reports a rank above another.
func (r subtitleRank) beats(other subtitleRank) bool {
	for i := range r {
		if r[i] != other[i] {
			return r[i] > other[i]
		}
	}
	return false
}

// subtitleFit is how well a track fits the video: a track inside it was made
// for it; a file lined up with the video's own subtitle scores the share of
// its lines that meet it, in points, less otherSpeedPenalty when it was timed
// for another frame rate; a file nothing could measure scores nothing.
func subtitleFit(track Track) (class, points int) {
	switch {
	case !track.External():
		return fitInside, 0
	case track.Alignment.Verdict != subtitle.VerdictMatch:
		return fitUnmeasured, 0
	}
	points = int(math.Round(track.Alignment.Score * 100))
	if math.Abs(track.Alignment.Rate-1) >= otherSpeed {
		points -= otherSpeedPenalty
	}
	return fitLinedUp, points
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// forcedNames mark forced tracks in their names; many releases name them so
// without setting the container flag.
var forcedNames = []string{"forced", "forçad", "forcad"}

// isForced reports a track that only carries the lines spoken in a foreign
// language, by its flag or by its name.
func isForced(track Track) bool {
	if track.Forced {
		return true
	}
	name := strings.ToLower(track.Name)
	for _, marker := range forcedNames {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

// VoiceTrack is the audio track that speaks a language, the one whose voice
// the karaoke mode follows, and its position among the audio tracks.
// Declared and default tracks come first, so the film wins over a
// commentary. False when no track speaks the language.
func VoiceTrack(info Info, language LanguageTag) (track Track, position int, found bool) {
	audio := info.TracksOf(TrackAudio)
	number := selectAudio(audio, language)
	for i, candidate := range audio {
		if candidate.Number == number && languageMatch(candidate.Language, language) != noLanguageMatch {
			return candidate, i, true
		}
	}
	return Track{}, 0, false
}

func selectAudio(tracks []Track, want LanguageTag) uint64 {
	best, bestScore := uint64(0), 0
	for _, track := range tracks {
		score := languageMatch(track.Language, want) * languageWeight
		if score == 0 {
			continue
		}
		if !track.LanguageGuessed {
			score += declaredWeight
		}
		if track.Default {
			score += defaultFlagWeight
		}
		if score > bestScore {
			best, bestScore = track.Number, score
		}
	}
	if best != 0 {
		return best
	}
	for _, track := range tracks {
		if track.Default {
			return track.Number
		}
	}
	if len(tracks) > 0 {
		return tracks[0].Number
	}
	return 0
}

// languageMatch rates how well a track language serves the wanted language.
func languageMatch(track, want LanguageTag) int {
	switch {
	case want == "" || track == "" || track.Base() != want.Base():
		return noLanguageMatch
	case track == want:
		return exactLanguageTags
	case track.Region() == "" || want.Region() == "":
		return unspecificRegion
	default:
		return sameBaseLanguage
	}
}
