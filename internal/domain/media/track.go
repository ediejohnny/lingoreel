// Package media models what a video file contains: its tracks, their
// languages and the automatic choice of audio, primary subtitle and translation.
package media

import (
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// TrackType is the kind of content a track carries.
type TrackType string

const (
	TrackVideo    TrackType = "video"
	TrackAudio    TrackType = "audio"
	TrackSubtitle TrackType = "subtitle"
	TrackOther    TrackType = "other"
)

// ExternalTrackBase numbers the subtitle files found next to a video, in
// discovery order, far above any container track number in real files.
const ExternalTrackBase uint64 = 1 << 32

// Track describes one track of a media file, or a subtitle file next to it.
// Number is the container's track number (ExternalTrackBase and up for
// files) and is never zero; UID stays stable when the file is remuxed.
type Track struct {
	Number uint64
	UID    uint64
	Type   TrackType
	Codec  string
	// Language is the normalized BCP 47 tag, empty when unknown.
	Language LanguageTag
	// LanguageGuessed marks a language nobody stated: the container's
	// default ("eng" when Matroska omits the element). It is trusted less.
	LanguageGuessed bool
	Name            string
	Default         bool
	Forced          bool
	HearingImpaired bool

	// Video tracks only.
	Width     int
	Height    int
	FrameRate float64

	// Subtitle tracks only. Readable is false for bitmap formats (PGS,
	// VobSub), which would need OCR, and for encodings that cannot be read.
	SubtitleFormat subtitle.Format
	Readable       bool
	// Path is the subtitle file of an external track; empty for tracks
	// inside the video.
	Path string
	// Candidate marks a file found where other films' subtitles may be (a
	// subfolder, another name): it is chosen automatically only when its
	// timing matches the video.
	Candidate bool
	// Added marks a file the user picked, from anywhere, rather than one
	// found next to the video.
	Added bool
	// Alignment is how a readable file lines up with the video's own
	// subtitles; an empty Verdict means it was not measured.
	Alignment subtitle.Alignment
}

// AutoSelectable reports a track the automatic choice may use: readable, and
// for files, not timed for another cut or film, and matching the video when
// nothing else ties the file to it.
func (t Track) AutoSelectable() bool {
	switch {
	case !t.Readable:
		return false
	case t.Alignment.Verdict == subtitle.VerdictOtherCut || t.Alignment.Verdict == subtitle.VerdictNoMatch:
		return false
	case t.Candidate:
		return t.Alignment.Verdict == subtitle.VerdictMatch
	}
	return true
}

// External reports a subtitle file found next to the video.
func (t Track) External() bool {
	return t.Path != ""
}

// SubtitleContent holds the events read from one subtitle track and how many
// frames were dropped for being unreadable, empty or malformed.
type SubtitleContent struct {
	Events  []subtitle.Event
	Skipped int
}

// Info describes a media file.
type Info struct {
	Duration time.Duration
	Tracks   []Track
}

// TracksOf returns the tracks of one type in container order.
func (i Info) TracksOf(kind TrackType) []Track {
	return OfType(i.Tracks, kind)
}

// OfType returns the tracks of one type from a list, in its order.
func OfType(tracks []Track, kind TrackType) []Track {
	var of []Track
	for _, track := range tracks {
		if track.Type == kind {
			of = append(of, track)
		}
	}
	return of
}

// Track returns the track with the given number.
func (i Info) Track(number uint64) (Track, bool) {
	for _, track := range i.Tracks {
		if track.Number == number {
			return track, true
		}
	}
	return Track{}, false
}
