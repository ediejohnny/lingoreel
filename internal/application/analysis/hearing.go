package analysis

import (
	"maps"
	"slices"
	"time"

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

// FitByVoice lines up the unheard files with the voice heard in stretches of
// the video (subtitle.HearingStretches), and suggests the tracks again; it
// returns the new analysis, the one passed in left as it was, and the fit
// found for each file.
func FitByVoice(a Analysis, heard []speech.Track, preferences media.Preferences) (Analysis, map[uint64]subtitle.Alignment) {
	fits := map[uint64]subtitle.Alignment{}
	for _, number := range a.Unheard() {
		fits[number] = subtitle.AlignToVoice(heard, spokenLines(a.FileLines(number)), a.Info.Duration).Alignment
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
