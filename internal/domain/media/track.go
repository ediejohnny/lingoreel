// Package media models what a video file contains: its tracks, their
// languages and the automatic choice of audio, primary subtitle and translation.
package media

import (
	"time"

	"lingoreel/internal/domain/subtitle"
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
	var tracks []Track
	for _, track := range i.Tracks {
		if track.Type == kind {
			tracks = append(tracks, track)
		}
	}
	return tracks
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
