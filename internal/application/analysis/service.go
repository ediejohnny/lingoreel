// Package analysis holds the use cases that open a video: reading what it
// contains, suggesting the study tracks and loading their subtitles.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// ErrUnsupportedFile reports a file the player cannot open.
var ErrUnsupportedFile = errors.New("unsupported video file")

// trackExtensions are the containers whose own tracks the Matroska reader
// understands.
var trackExtensions = []string{".mkv", ".webm"}

// externalOnlyExtensions are videos the players open whose own tracks are not
// read: their subtitles come only from the files next to them (the product
// works "especially" with .mkv).
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
	// Find returns the files that belong to the video by their name or folder.
	Find(videoPath string) ([]string, error)
	// FindCandidates returns other files nearby that may belong to it.
	FindCandidates(videoPath string) ([]string, error)
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
	// internal keeps the container tracks read as the timing reference.
	internal map[uint64]media.SubtitleContent
}

// LoadedTrack is a subtitle track ready to be synchronized with playback.
type LoadedTrack struct {
	Timeline *subtitle.Timeline
	// Skipped counts frames dropped for being unreadable, empty or malformed.
	Skipped int
	// Corrected tells that a file's times were moved by Correction to match
	// the video.
	Corrected  bool
	Correction subtitle.Alignment
}

const (
	// minCorrectionOffset and minCorrectionRate are the least correction
	// worth applying; below them the file already matches.
	minCorrectionOffset = time.Millisecond
	minCorrectionRate   = 1e-6
)

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

// VideoExtensions are every extension IsSupported takes, such as ".mkv".
func VideoExtensions() []string {
	return slices.Concat(trackExtensions, externalOnlyExtensions)
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
	var internal map[uint64]media.SubtitleContent
	if len(contents) > 0 {
		if internal, err = s.alignFiles(ctx, path, info, tracks, contents); err != nil {
			return Analysis{}, err
		}
	}
	info.Tracks = append(info.Tracks, tracks...)
	return Analysis{Path: path, Info: info, Suggested: media.SelectTracks(info, preferences), external: contents, internal: internal}, nil
}

// alignFiles measures how each subtitle file lines up with the video against
// its fullest subtitle track, in any language. The container tracks read for
// that are returned to be kept, so loading one of them later reads nothing.
// A video whose tracks cannot be read is only compared by its length.
func (s *Service) alignFiles(ctx context.Context, path string, info media.Info, tracks []media.Track, contents map[uint64]media.SubtitleContent) (map[uint64]media.SubtitleContent, error) {
	var readable []uint64
	for _, track := range info.TracksOf(media.TrackSubtitle) {
		if track.Readable {
			readable = append(readable, track.Number)
		}
	}
	var internal map[uint64]media.SubtitleContent
	if len(readable) > 0 {
		read, err := s.reader.ReadSubtitles(ctx, path, readable, nil)
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case err == nil:
			internal = read
		}
	}
	var reference []time.Duration
	for _, number := range readable {
		if events := internal[number].Events; len(events) > len(reference) {
			reference = subtitle.Starts(events)
		}
	}
	for i := range tracks {
		if content, read := contents[tracks[i].Number]; read {
			tracks[i].Alignment = subtitle.Align(reference, subtitle.Starts(content.Events), info.Duration)
		}
	}
	return internal, nil
}

// externalTracks describes the subtitle files of the video, then the
// candidates nearby. Each one is read now: to know whether it is readable
// and, when its name does not say, to detect its language from the text.
// What was read is kept for loading.
func (s *Service) externalTracks(ctx context.Context, videoPath string) ([]media.Track, map[uint64]media.SubtitleContent, error) {
	if s.files == nil {
		return nil, nil, nil
	}
	// A folder that cannot be listed only means no external subtitles.
	paths, _ := s.files.Find(videoPath)
	own := len(paths)
	candidates, _ := s.files.FindCandidates(videoPath)
	paths = append(paths, candidates...)
	tracks := make([]media.Track, 0, len(paths))
	contents := map[uint64]media.SubtitleContent{}
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		hints := media.HintsFromFileName(filepath.Base(path), filepath.Base(videoPath))
		track := media.Track{
			Number: media.ExternalTrackBase + uint64(i), Type: media.TrackSubtitle,
			Name: filepath.Base(path), Path: path, Language: hints.Language,
			Forced: hints.Forced, HearingImpaired: hints.HearingImpaired,
			Candidate: i >= own,
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
			if cached, read := analysis.internal[number]; read {
				content[number] = cached
			} else {
				internal = append(internal, number)
			}
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
		events, result := track.Events, LoadedTrack{Skipped: track.Skipped}
		if alignment, ok := correction(analysis.Info, number); ok {
			events, result.Corrected, result.Correction = subtitle.Retime(events, alignment), true, alignment
		}
		timeline, err := subtitle.NewTimeline(events)
		if err != nil {
			return nil, fmt.Errorf("indexing subtitle track %d: %w", number, err)
		}
		result.Timeline = timeline
		loaded[number] = result
	}
	return loaded, nil
}

// correction returns how to move a file's times to match the video: only for
// a file whose timing matched (tracks of the video are never measured), and
// only when the move is measurable.
func correction(info media.Info, number uint64) (subtitle.Alignment, bool) {
	track, _ := info.Track(number)
	alignment := track.Alignment
	if alignment.Verdict != subtitle.VerdictMatch {
		return subtitle.Alignment{}, false
	}
	moves := alignment.Offset.Abs() >= minCorrectionOffset || math.Abs(alignment.Rate-1) >= minCorrectionRate
	return alignment, moves
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
