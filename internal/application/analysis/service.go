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

// ErrUnsupportedFile reports a file the player cannot open.
var ErrUnsupportedFile = errors.New("unsupported video file")

// trackExtensions are the containers whose own tracks the Matroska reader
// understands.
var trackExtensions = []string{".mkv", ".webm"}

// externalOnlyExtensions are videos the players open whose own tracks are not
// read: their subtitles come only from the files next to them (the product
// works "especially" with .mkv, PRD §1).
var externalOnlyExtensions = []string{".mp4", ".m4v", ".avi", ".mov", ".wmv"}

// Reader reads media files. The Matroska file reader implements it.
type Reader interface {
	Analyze(path string) (media.Info, error)
	// Check confirms that a video whose tracks are not read can be opened.
	Check(path string) error
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
	// external keeps the subtitle files read during the analysis, so loading
	// one of them does not parse it again.
	external map[uint64]media.SubtitleContent
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

// IsSupported reports whether the player can open the file: a container whose
// tracks it reads, or a video that only gets subtitles from files.
func IsSupported(path string) bool {
	return readsTracks(path) || slices.Contains(externalOnlyExtensions, strings.ToLower(filepath.Ext(path)))
}

func readsTracks(path string) bool {
	return slices.Contains(trackExtensions, strings.ToLower(filepath.Ext(path)))
}

// Analyze reads the tracks of a video and suggests the audio, primary
// subtitle and translation for the given language preferences.
func (s *Service) Analyze(ctx context.Context, path string, preferences media.Preferences) (Analysis, error) {
	if !IsSupported(path) {
		return Analysis{}, fmt.Errorf("%w: %s", ErrUnsupportedFile, filepath.Base(path))
	}
	var info media.Info
	if readsTracks(path) {
		var err error
		if info, err = s.reader.Analyze(path); err != nil {
			return Analysis{}, fmt.Errorf("reading the tracks of %s: %w", filepath.Base(path), err)
		}
	} else if err := s.reader.Check(path); err != nil {
		return Analysis{}, fmt.Errorf("opening %s: %w", filepath.Base(path), err)
	}
	tracks, contents, err := s.externalTracks(ctx, path)
	if err != nil {
		return Analysis{}, err
	}
	info.Tracks = append(info.Tracks, tracks...)
	return Analysis{Path: path, Info: info, Suggested: media.SelectTracks(info, preferences), external: contents}, nil
}

// externalTracks describes the subtitle files next to the video. Each one is
// read now: to know whether it is readable and, when its name does not say,
// to detect its language from the text. What was read is kept for loading.
func (s *Service) externalTracks(ctx context.Context, videoPath string) ([]media.Track, map[uint64]media.SubtitleContent, error) {
	if s.files == nil {
		return nil, nil, nil
	}
	// A folder that cannot be listed only means no external subtitles.
	paths, _ := s.files.Find(videoPath)
	tracks := make([]media.Track, 0, len(paths))
	contents := map[uint64]media.SubtitleContent{}
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
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
			contents[track.Number] = content
		}
		tracks = append(tracks, track)
	}
	return tracks, contents, nil
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
		if cached, read := analysis.external[number]; read {
			content[number] = cached
			continue
		}
		if s.files == nil {
			return nil, fmt.Errorf("subtitle file %s cannot be read", track.Name)
		}
		_, file, err := s.files.Load(track.Path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", track.Name, err)
		}
		content[number] = file
	}
	if len(internal) > 0 {
		read, err := s.reader.ReadSubtitles(ctx, analysis.Path, internal, progress)
		if err != nil {
			return nil, fmt.Errorf("reading the subtitles of %s: %w", filepath.Base(analysis.Path), err)
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
