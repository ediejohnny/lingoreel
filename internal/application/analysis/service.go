// Package analysis holds the use cases that open a video: reading what it
// contains, suggesting the study tracks and loading their subtitles.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// ErrUnsupportedFile reports a file whose container the player cannot read yet.
var ErrUnsupportedFile = errors.New("unsupported video file")

// supportedExtensions are the containers the Matroska reader understands.
var supportedExtensions = []string{".mkv", ".webm"}

// Reader reads media files. The Matroska file reader implements it.
type Reader interface {
	Analyze(path string) (media.Info, error)
	ReadSubtitles(ctx context.Context, path string, tracks []uint64, progress func(done, total int64)) (map[uint64]media.SubtitleContent, error)
}

// Analysis is what the player knows about a video after opening it.
type Analysis struct {
	Path      string
	Info      media.Info
	Suggested media.Selection
}

// LoadedTrack is a subtitle track ready to be synchronized with playback.
type LoadedTrack struct {
	Timeline *subtitle.Timeline
	// Skipped counts frames dropped for being unreadable, empty or malformed.
	Skipped int
}

// Service runs the analysis use cases.
type Service struct {
	reader Reader
}

// NewService creates the service on top of a media reader.
func NewService(reader Reader) *Service {
	return &Service{reader: reader}
}

// IsSupported reports whether the file extension belongs to a readable container.
func IsSupported(path string) bool {
	return slices.Contains(supportedExtensions, strings.ToLower(filepath.Ext(path)))
}

// Analyze reads the tracks of a video and suggests the audio, primary
// subtitle and translation for the given language preferences.
func (s *Service) Analyze(path string, preferences media.Preferences) (Analysis, error) {
	if !IsSupported(path) {
		return Analysis{}, fmt.Errorf("%w: %s", ErrUnsupportedFile, filepath.Base(path))
	}
	info, err := s.reader.Analyze(path)
	if err != nil {
		return Analysis{}, err
	}
	return Analysis{Path: path, Info: info, Suggested: media.SelectTracks(info, preferences)}, nil
}

// LoadSubtitles reads the given subtitle tracks and indexes their events.
// Zero track numbers mean "no track" and are ignored.
func (s *Service) LoadSubtitles(ctx context.Context, path string, tracks []uint64, progress func(done, total int64)) (map[uint64]LoadedTrack, error) {
	wanted := uniqueTracks(tracks)
	if len(wanted) == 0 {
		return map[uint64]LoadedTrack{}, nil
	}
	content, err := s.reader.ReadSubtitles(ctx, path, wanted, progress)
	if err != nil {
		return nil, err
	}

	loaded := make(map[uint64]LoadedTrack, len(wanted))
	for _, number := range wanted {
		track, found := content[number]
		if !found {
			return nil, fmt.Errorf("subtitle track %d was not read", number)
		}
		timeline, err := subtitle.NewTimeline(track.Events)
		if err != nil {
			return nil, fmt.Errorf("indexing subtitle track %d: %w", number, err)
		}
		loaded[number] = LoadedTrack{Timeline: timeline, Skipped: track.Skipped}
	}
	return loaded, nil
}

func uniqueTracks(tracks []uint64) []uint64 {
	var unique []uint64
	for _, number := range tracks {
		if number != 0 && !slices.Contains(unique, number) {
			unique = append(unique, number)
		}
	}
	return unique
}
