package media

import "strings"

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

// Weights of the selection score, compared as tiers: a higher tier always
// outweighs every lower one combined. Tracks of another language never take
// part. Among the rest, a forced track loses first, because it only carries
// the lines spoken in a foreign language; then a language nobody declared;
// then a less exact region; then the default and hearing-impaired flags.
const (
	notForcedWeight       = 10_000
	declaredWeight        = 1_000
	languageWeight        = 100 // times the match level, at most 3
	defaultFlagWeight     = 5
	notHearingImpairedBit = 2
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

// selectSubtitle returns the best scoring track in the wanted language other
// than excluded, or zero.
func selectSubtitle(tracks []Track, want LanguageTag, excluded uint64) uint64 {
	best, bestScore := uint64(0), -1
	for _, track := range tracks {
		if track.Number == excluded {
			continue
		}
		match := languageMatch(track.Language, want)
		if match == noLanguageMatch {
			continue
		}
		score := match*languageWeight + subtitleQuality(track)
		if score > bestScore {
			best, bestScore = track.Number, score
		}
	}
	return best
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

func subtitleQuality(track Track) int {
	score := 0
	if !isForced(track) {
		score += notForcedWeight
	}
	if !track.LanguageGuessed {
		score += declaredWeight
	}
	if track.Default {
		score += defaultFlagWeight
	}
	if !track.HearingImpaired {
		score += notHearingImpairedBit
	}
	return score
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
