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

// SubtitleFiles finds and reads the subtitle files that sit next to a video.
type SubtitleFiles interface {
	Find(videoPath string) ([]string, error)
	Load(path string) (subtitle.Format, media.SubtitleContent, error)
}

// Analysis is what the player knows about a video after opening it. Its
// tracks include the subtitle files found next to the video.
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
	files  SubtitleFiles
}

// NewService creates the service on top of a media reader and, optionally
// (nil), a source of subtitle files.
func NewService(reader Reader, files SubtitleFiles) *Service {
	return &Service{reader: reader, files: files}
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
	info.Tracks = append(info.Tracks, s.externalTracks(path)...)
	return Analysis{Path: path, Info: info, Suggested: media.SelectTracks(info, preferences)}, nil
}

// externalTracks describes the subtitle files next to the video. Each one is
// read now: to know whether it is readable and, when its name does not say,
// to detect its language from the text.
func (s *Service) externalTracks(videoPath string) []media.Track {
	if s.files == nil {
		return nil
	}
	// A folder that cannot be listed only means no external subtitles.
	paths, _ := s.files.Find(videoPath)
	tracks := make([]media.Track, 0, len(paths))
	for i, path := range paths {
		hints := media.HintsFromFileName(path, videoPath)
		track := media.Track{
			Number: media.ExternalTrackBase + uint64(i), Type: media.TrackSubtitle,
			Name: filepath.Base(path), Path: path, Language: hints.Language,
			Forced: hints.Forced, HearingImpaired: hints.HearingImpaired,
		}
		format, content, err := s.files.Load(path)
		if err == nil {
			track.Readable, track.SubtitleFormat = true, format
			track.Codec = strings.ToUpper(string(format))
			if track.Language == "" {
				track.Language = media.DetectLanguage(joinedText(content.Events))
			}
		}
		tracks = append(tracks, track)
	}
	return tracks
}

func joinedText(events []subtitle.Event) string {
	var text strings.Builder
	for _, event := range events {
		text.WriteString(event.Text())
		text.WriteByte('\n')
	}
	return text.String()
}

// LoadSubtitles reads the given subtitle tracks of an analyzed video, inside
// it or in files next to it, and indexes their events. Zero track numbers
// mean "no track" and are ignored.
func (s *Service) LoadSubtitles(ctx context.Context, analysis Analysis, tracks []uint64, progress func(done, total int64)) (map[uint64]LoadedTrack, error) {
	wanted := uniqueTracks(tracks)
	if len(wanted) == 0 {
		return map[uint64]LoadedTrack{}, nil
	}
	var internal []uint64
	content := map[uint64]media.SubtitleContent{}
	for _, number := range wanted {
		track, found := analysis.Info.Track(number)
		if !found {
			return nil, fmt.Errorf("subtitle track %d does not exist", number)
		}
		if !track.External() {
			internal = append(internal, number)
			continue
		}
		if s.files == nil {
			return nil, fmt.Errorf("subtitle file %s cannot be read", track.Name)
		}
		_, file, err := s.files.Load(track.Path)
		if err != nil {
			return nil, err
		}
		content[number] = file
	}
	if len(internal) > 0 {
		read, err := s.reader.ReadSubtitles(ctx, analysis.Path, internal, progress)
		if err != nil {
			return nil, err
		}
		for number, track := range read {
			content[number] = track
		}
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
