package app

import (
	"context"
	"slices"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

// An audio track whose file does not tell its language is heard as the
// video opens, in the background, in each of the profile's two languages
// whose speech pack is here: the language's model places some lines of a
// subtitle in that language in the track (speech.HeardLanguage tells the
// rule). The language heard names the track and counts for the choice of
// the audio; the project keeps what was heard, so a track is heard again
// only in a language not tried yet.

// SpeechModel is a language's speech pack, opened to place lines in sound.
type SpeechModel interface {
	voice.Aligner
	Close()
}

// languageHearing is the hearing of the open video's untold audio tracks.
type languageHearing struct {
	cancel context.CancelFunc
	path   string
}

// spokenCandidate is a language to hear the tracks in, with the subtitle
// whose lines are heard.
type spokenCandidate struct {
	language string
	subtitle uint64
}

// withKeptHearings gives the video's untold audio tracks the languages the
// project kept for them.
func withKeptHearings(result analysis.Analysis, p *videoProject, preferences media.Preferences) analysis.Analysis {
	heard := map[uint64]media.LanguageTag{}
	for _, track := range untoldAudio(result.Info) {
		if hearing := p.value.Tracks[p.keys[track.Number]].Hearing; hearing != nil && hearing.Language != "" {
			heard[track.Number] = media.LanguageTag(hearing.Language)
		}
	}
	return result.WithHeardLanguages(heard, preferences)
}

// untoldAudio are a video's audio tracks whose file does not tell their
// language: none, or only the format's default.
func untoldAudio(info media.Info) []media.Track {
	var untold []media.Track
	for _, track := range info.TracksOf(media.TrackAudio) {
		if !track.LanguageHeard && (track.Language == "" || track.LanguageGuessed) {
			untold = append(untold, track)
		}
	}
	return untold
}

// readyPacks are the languages whose speech pack is here. It looks at the
// disk: called without a.mu.
func (a *API) readyPacks() map[string]bool {
	ready := map[string]bool{}
	if a.deps.SpeechPacks != nil {
		for _, state := range a.deps.SpeechPacks.States() {
			ready[state.Language] = state.Ready
		}
	}
	return ready
}

// spokenCandidates are the profile's languages a video's tracks can be
// heard in: each with a speech pack ready and a subtitle in it, the one
// suggested first.
func spokenCandidates(result analysis.Analysis, preferences media.Preferences, ready map[string]bool) []spokenCandidate {
	var candidates []spokenCandidate
	for _, tag := range []media.LanguageTag{preferences.Primary, preferences.Translation} {
		language := tag.Base()
		if language == "" || !ready[language] || slices.ContainsFunc(candidates, func(c spokenCandidate) bool { return c.language == language }) {
			continue
		}
		subtitles := result.Info.TracksOf(media.TrackSubtitle)
		i := slices.IndexFunc(subtitles, func(t media.Track) bool {
			return t.Number == result.Suggested.Primary && t.Language.Base() == language ||
				t.Number == result.Suggested.Translation && t.Language.Base() == language
		})
		if i < 0 {
			i = slices.IndexFunc(subtitles, func(t media.Track) bool { return t.Language.Base() == language && t.Readable })
		}
		if i >= 0 {
			candidates = append(candidates, spokenCandidate{language: language, subtitle: subtitles[i].Number})
		}
	}
	return candidates
}

// startLanguageHearing starts hearing a video just opened when it has audio
// tracks whose language its file does not tell, not yet heard in every
// language it can be heard in, and the voice model may work. Called with
// a.mu held.
func (a *API) startLanguageHearing(ctx context.Context, result analysis.Analysis, opened *videoProject, preferences media.Preferences,
	onBattery bool, ready map[string]bool) {
	ports := a.deps.Ports
	setup := voiceSetup(a.config.Voice)
	if a.deps.OpenSpeech == nil || ports.FindDecoder == nil || setup.Rest(onBattery) != watch.ModelAtWork {
		return
	}
	candidates := spokenCandidates(result, preferences, ready)
	languages := make([]string, 0, len(candidates))
	for _, c := range candidates {
		languages = append(languages, c.language)
	}
	// kept are the scores heard before, by track: only the languages a track
	// lacks are heard now.
	kept := map[uint64][]speech.LanguageScore{}
	var tracks []media.Track
	for _, track := range untoldAudio(result.Info) {
		hearing := opened.value.Tracks[opened.keys[track.Number]].Hearing
		if hearing.Covers(languages) {
			continue
		}
		tracks = append(tracks, track)
		if hearing != nil {
			for _, s := range hearing.Scores {
				kept[track.Number] = append(kept[track.Number], speech.LanguageScore{Language: s.Language, Score: s.Score, Lines: s.Lines})
			}
		}
	}
	if len(candidates) == 0 || len(tracks) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &languageHearing{cancel: cancel, path: result.Path}
	a.hearLanguage = h
	go a.hearLanguages(ctx, h, result, tracks, candidates, kept, setup.FFmpegPath)
}

// stopLanguageHearing stops hearing a video no longer open. Called with
// a.mu held.
func (a *API) stopLanguageHearing() {
	if a.hearLanguage != nil {
		a.hearLanguage.cancel()
		a.hearLanguage = nil
	}
}

// hearLanguages hears each track in each candidate language it lacks, one
// language's model open at a time, and decides each track's language from
// those scores and the ones kept.
func (a *API) hearLanguages(ctx context.Context, h *languageHearing, result analysis.Analysis, tracks []media.Track,
	candidates []spokenCandidate, kept map[uint64][]speech.LanguageScore, ffmpegPath string) {
	decoder := a.decoderFor(ctx, ffmpegPath, "the audio's language")
	if decoder == nil {
		a.endLanguageHearing(h, nil)
		return
	}
	numbers := make([]uint64, 0, len(candidates))
	for _, c := range candidates {
		numbers = append(numbers, c.subtitle)
	}
	loaded, err := a.deps.Service.LoadSubtitles(ctx, result, numbers, nil)
	if err != nil {
		if ctx.Err() == nil {
			a.logger.Warn("the audio's language was not heard: the subtitles did not load", "error", err)
		}
		a.endLanguageHearing(h, nil)
		return
	}
	audio := result.Info.TracksOf(media.TrackAudio)
	scores := map[uint64][]speech.LanguageScore{}
	for number, s := range kept {
		scores[number] = slices.Clone(s)
	}
	for _, c := range candidates {
		var lacking []media.Track
		for _, track := range tracks {
			if !slices.ContainsFunc(scores[track.Number], func(s speech.LanguageScore) bool { return s.Language == c.language }) {
				lacking = append(lacking, track)
			}
		}
		if len(lacking) == 0 {
			continue
		}
		model, err := a.deps.OpenSpeech(c.language)
		if err != nil {
			a.logger.Warn("the speech pack did not open", "language", c.language, "error", err)
			continue
		}
		lines := voice.LinesToHear(loaded[c.subtitle].Timeline.Events())
		heard := a.scoreTracks(ctx, decoder, result.Path, audio, lacking, voice.LanguageCandidate{Language: c.language, Model: model, Lines: lines}, scores)
		model.Close()
		if !heard {
			a.endLanguageHearing(h, nil)
			return
		}
	}
	a.endLanguageHearing(h, scores)
}

// scoreTracks scores one language in each track, adding to scores; false
// when the hearing stopped.
func (a *API) scoreTracks(ctx context.Context, decoder voice.Decoder, path string, audio, tracks []media.Track, candidate voice.LanguageCandidate, scores map[uint64][]speech.LanguageScore) bool {
	for _, track := range tracks {
		position := slices.IndexFunc(audio, func(t media.Track) bool { return t.Number == track.Number })
		stretch := voice.Stretch{File: path, Track: position, Channels: track.Channels, Codec: track.Codec}
		found, err := voice.ScoreLanguages(ctx, decoder, stretch, []voice.LanguageCandidate{candidate}, nil)
		if err != nil {
			if ctx.Err() == nil {
				a.logger.Warn("hearing an audio track's language failed", "track", track.Number, "error", err)
			}
			return false
		}
		scores[track.Number] = append(scores[track.Number], found...)
	}
	return true
}

// endLanguageHearing decides each track's language from its scores, gives
// the open video's tracks the languages heard, keeps the scores in its
// project and tells the screens the video again. A video no longer open, or
// nothing heard, changes nothing.
func (a *API) endLanguageHearing(h *languageHearing, scores map[uint64][]speech.LanguageScore) {
	decided := map[uint64]string{}
	heard := map[uint64]media.LanguageTag{}
	for number, s := range scores {
		language, _ := speech.HeardLanguage(s)
		decided[number] = language
		if language != "" {
			heard[number] = media.LanguageTag(language)
		}
		a.logger.Info("an audio track was heard", "track", number, "language", language, "scores", s)
	}
	a.mu.Lock()
	if a.hearLanguage != h {
		a.mu.Unlock()
		return
	}
	a.hearLanguage = nil
	if len(scores) == 0 {
		a.mu.Unlock()
		return
	}
	a.showFound(h.path, func(video analysis.Analysis, preferences media.Preferences) analysis.Analysis {
		return video.WithHeardLanguages(heard, preferences)
	}, func(opened *videoProject, _ analysis.Analysis) {
		for number, s := range scores {
			key := opened.keys[number]
			if key == "" {
				continue
			}
			hearing := &project.Hearing{Language: decided[number]}
			for _, score := range s {
				hearing.Scores = append(hearing.Scores, project.HeardScore{Language: score.Language, Score: score.Score, Lines: score.Lines})
			}
			track := opened.track(key)
			track.Hearing = hearing
			opened.value.Tracks[key] = track
		}
		a.markDirty(opened)
	})
}
