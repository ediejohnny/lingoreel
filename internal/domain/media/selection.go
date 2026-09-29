package media

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

// Weights of the selection score. Language dominates everything else; among
// tracks of the same language, forced tracks lose because they only carry the
// lines spoken in a foreign language.
const (
	languageWeight        = 100
	notForcedWeight       = 50
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
//   - primary: best language match, else the first readable subtitle;
//   - translation: best language match only, never another language;
//   - audio: best language match, else the default track, else the first.
func SelectTracks(info Info, preferences Preferences) Selection {
	var selection Selection
	selection.Audio = selectAudio(info.TracksOf(TrackAudio), preferences.Audio)

	subtitles := readableSubtitles(info.TracksOf(TrackSubtitle))
	selection.Primary = selectSubtitle(subtitles, preferences.Primary, 0, true)
	if preferences.Translation != "" {
		selection.Translation = selectSubtitle(subtitles, preferences.Translation, selection.Primary, false)
	}
	return selection
}

func readableSubtitles(tracks []Track) []Track {
	var readable []Track
	for _, track := range tracks {
		if track.Readable {
			readable = append(readable, track)
		}
	}
	return readable
}

// selectSubtitle returns the best scoring track other than excluded. When no
// track speaks the wanted language, it falls back to the first non-forced track
// only if fallbackToAny is set.
func selectSubtitle(tracks []Track, want LanguageTag, excluded uint64, fallbackToAny bool) uint64 {
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
	if best != 0 || !fallbackToAny {
		return best
	}

	for _, track := range tracks {
		if track.Number != excluded && !track.Forced {
			return track.Number
		}
	}
	for _, track := range tracks {
		if track.Number != excluded {
			return track.Number
		}
	}
	return 0
}

func subtitleQuality(track Track) int {
	score := 0
	if !track.Forced {
		score += notForcedWeight
	}
	if track.Default {
		score += defaultFlagWeight
	}
	if !track.HearingImpaired {
		score += notHearingImpairedBit
	}
	return score
}

func selectAudio(tracks []Track, want LanguageTag) uint64 {
	best, bestScore := uint64(0), 0
	for _, track := range tracks {
		score := languageMatch(track.Language, want) * languageWeight
		if score == 0 {
			continue
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
