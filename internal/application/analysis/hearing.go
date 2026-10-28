package analysis

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// Unheard are the subtitle files nothing told the fit of: the video has no
// subtitle of its own to compare them with. Hearing the video's voice tells
// (FitByVoice).
func (a Analysis) Unheard() []uint64 {
	var unheard []uint64
	for _, track := range a.Info.TracksOf(media.TrackSubtitle) {
		if track.External() && track.Readable && track.Alignment.Verdict == subtitle.VerdictUnknown && !track.Alignment.Heard {
			unheard = append(unheard, track.Number)
		}
	}
	return unheard
}

// HearingLength is how long the video is, for choosing the stretches to hear:
// its own length, or for a video whose tracks are not read, as far as its
// files' lines go.
func (a Analysis) HearingLength() time.Duration {
	if a.Info.Duration > 0 {
		return a.Info.Duration
	}
	var last time.Duration
	for _, number := range a.Unheard() {
		for _, event := range a.FileLines(number) {
			last = max(last, event.End)
		}
	}
	return last
}

// FileLines are the lines of a subtitle file as it was read, before any
// correction; nil for a track that is not a file read.
func (a Analysis) FileLines(number uint64) []subtitle.Event {
	return a.external[number].Events
}

// HearVoice hears the stretches of the video's voice that line its unheard
// files up (subtitle.HearingStretches), in the audio track VoiceStretch picks
// for language. progress, when not nil, hears the stretch being heard, from
// 0, and the share of it done, from 0 to 1.
func HearVoice(ctx context.Context, a Analysis, decoder voice.Decoder, model speech.Model, language media.LanguageTag, progress func(stretch int, done float64)) ([]speech.Track, error) {
	return hearStretches(ctx, a, decoder, model, language, subtitle.HearingStretches(a.HearingLength()), progress)
}

// HearMoreVoice hears the stretches between the first ones
// (subtitle.HearingStretchesBetween), as HearVoice does, when they left a
// file undecided (Undecided); FitByVoice takes them beside the first ones.
func HearMoreVoice(ctx context.Context, a Analysis, decoder voice.Decoder, model speech.Model, language media.LanguageTag, progress func(stretch int, done float64)) ([]speech.Track, error) {
	return hearStretches(ctx, a, decoder, model, language, subtitle.HearingStretchesBetween(a.HearingLength()), progress)
}

// Undecided tells some unheard file may fit the voice heard but more
// stretches are needed to tell (subtitle.VoiceAlignment.Undecided).
func Undecided(a Analysis, heard []speech.Track) bool {
	for _, number := range a.Unheard() {
		if subtitle.AlignToVoice(heard, spokenLines(a.FileLines(number)), a.Info.Duration).Undecided() {
			return true
		}
	}
	return false
}

func hearStretches(ctx context.Context, a Analysis, decoder voice.Decoder, model speech.Model, language media.LanguageTag, stretches []subtitle.HearingStretch, progress func(stretch int, done float64)) ([]speech.Track, error) {
	base := VoiceStretch(a, language)
	heard := make([]speech.Track, 0, len(stretches))
	for i, s := range stretches {
		stretch := base
		stretch.From, stretch.Length = s.From, s.Length
		var told func(float64)
		if progress != nil {
			told = func(done float64) { progress(i, done) }
		}
		track, err := voice.Hear(ctx, decoder, model, stretch, told)
		if err != nil {
			return nil, err
		}
		heard = append(heard, track)
	}
	return heard, nil
}

// VoiceStretch is the audio track a hearing listens to: the one speaking
// language, the film's own (subtitles are timed to it, not to a dub); else
// the first.
func VoiceStretch(a Analysis, language media.LanguageTag) voice.Stretch {
	stretch := voice.Stretch{File: a.Path}
	track, position, found := media.VoiceTrack(a.Info, language)
	if !found {
		audio := a.Info.TracksOf(media.TrackAudio)
		if len(audio) == 0 {
			return stretch // a video whose tracks are not read: its first audio
		}
		track, position = audio[0], 0
	}
	stretch.Track, stretch.Channels, stretch.Codec = position, track.Channels, track.Codec
	return stretch
}

// FitByVoice lines up the unheard files with the voice heard in the first
// stretches of the video (HearVoice), and suggests the tracks again; a file
// they left undecided is lined up with the stretches between them too
// (HearMoreVoice; none when they were not heard), while every other keeps
// the verdict of the first. It returns the new analysis, the one passed in
// left as it was, and the fit found for each file.
func FitByVoice(a Analysis, first, between []speech.Track, preferences media.Preferences) (Analysis, map[uint64]subtitle.Alignment) {
	fits := map[uint64]subtitle.Alignment{}
	for _, number := range a.Unheard() {
		lines := spokenLines(a.FileLines(number))
		fit := subtitle.AlignToVoice(first, lines, a.Info.Duration)
		if fit.Undecided() && len(between) > 0 {
			fit = subtitle.AlignToVoice(subtitle.InterleaveHeard(first, between), lines, a.Info.Duration)
		}
		fits[number] = fit.Alignment
	}
	return a.WithFits(fits, preferences), fits
}

// WithFits returns the analysis with the fits given to its files, found by
// the voice now or kept from before, and the tracks suggested again.
func (a Analysis) WithFits(fits map[uint64]subtitle.Alignment, preferences media.Preferences) Analysis {
	if len(fits) == 0 {
		return a
	}
	result := a
	result.Info.Tracks = slices.Clone(a.Info.Tracks)
	for i, track := range result.Info.Tracks {
		if fit, found := fits[track.Number]; found && track.External() {
			fit.Heard = true
			result.Info.Tracks[i].Alignment = fit
		}
	}
	result.external, result.internal = maps.Clone(a.external), maps.Clone(a.internal)
	result.Suggested = media.SelectTracks(result.Info, preferences)
	return result
}

// WithHeardLanguages returns the analysis with the languages heard in its
// audio tracks given to those whose file does not tell theirs, and the
// tracks suggested again; a language the file states is never replaced.
func (a Analysis) WithHeardLanguages(heard map[uint64]media.LanguageTag, preferences media.Preferences) Analysis {
	if len(heard) == 0 {
		return a
	}
	result := a
	result.Info.Tracks = slices.Clone(a.Info.Tracks)
	for i, track := range result.Info.Tracks {
		untold := track.Language == "" || track.LanguageGuessed || track.LanguageHeard
		if language, found := heard[track.Number]; found && track.Type == media.TrackAudio && untold && language != "" {
			result.Info.Tracks[i].Language = language
			result.Info.Tracks[i].LanguageGuessed = false
			result.Info.Tracks[i].LanguageHeard = true
		}
	}
	result.external, result.internal = maps.Clone(a.external), maps.Clone(a.internal)
	result.Suggested = media.SelectTracks(result.Info, preferences)
	return result
}

// spokenLines are the times of a file's lines someone says: not a song nor a
// sound described.
func spokenLines(events []subtitle.Event) []speech.Span {
	var lines []speech.Span
	for _, event := range events {
		if units := study.SpeechUnits(event.Text()); units.Spoken() && !units.Lyrics {
			lines = append(lines, speech.Span{Start: event.Start, End: event.End})
		}
	}
	return lines
}
