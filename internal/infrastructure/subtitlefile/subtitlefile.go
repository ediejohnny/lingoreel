// Package subtitlefile finds the subtitle files that belong to a video, next
// to it or in a subtitles folder, and reads them. Users download subtitles
// themselves when the video has none in the wanted language.
package subtitlefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
	"lingoreel/internal/infrastructure/subtitleformat"
)

var (
	// ErrTooLarge reports a file larger than any real subtitle.
	ErrTooLarge = errors.New("subtitle file too large")
	// ErrUnsupported reports a file whose format is not a text subtitle.
	ErrUnsupported = errors.New("unsupported subtitle file")
)

// maxFolderEntries bounds how many entries of one folder are looked at, so a
// video in a huge folder (Downloads) does not stall the analysis.
const maxFolderEntries = 5000

var (
	subtitleExtensions = map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true}
	videoExtensions    = map[string]bool{".mkv": true, ".webm": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true, ".ts": true, ".m2ts": true}
	// subtitleFolders are folder names releases and users put subtitles in.
	subtitleFolders = map[string]bool{"subs": true, "sub": true, "subtitles": true, "subtitle": true, "legendas": true, "legenda": true}
)

// Files finds and reads subtitle files on disk.
type Files struct{}

// Find returns the subtitle files of a video: files named after it in its
// folder, or every subtitle file there when it is the folder's only video;
// the same inside a subtitles folder (Subs, Subtitles, Legendas), including a
// folder named after the video inside it. Files next to the video come first.
func (Files) Find(videoPath string) ([]string, error) {
	folder := filepath.Dir(videoPath)
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := readFolder(folder)
	if err != nil {
		return nil, fmt.Errorf("reading the video folder: %w", err)
	}
	onlyVideo := countVideos(entries) == 1

	var beside, inFolders []string
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case entry.Type().IsRegular() && isSubtitle(name) && (onlyVideo || namedAfter(name, videoName)):
			beside = append(beside, filepath.Join(folder, name))
		case entry.IsDir() && subtitleFolders[strings.ToLower(name)]:
			inFolders = append(inFolders, findInSubtitleFolder(filepath.Join(folder, name), videoName, onlyVideo)...)
		}
	}
	sort.Strings(beside)
	sort.Strings(inFolders)
	return append(beside, inFolders...), nil
}

func findInSubtitleFolder(folder, videoName string, onlyVideo bool) []string {
	entries, err := readFolder(folder)
	if err != nil {
		return nil // an unreadable subtitles folder only means fewer choices
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case entry.Type().IsRegular() && isSubtitle(name) && (onlyVideo || namedAfter(name, videoName)):
			found = append(found, filepath.Join(folder, name))
		case entry.IsDir() && strings.EqualFold(name, videoName):
			// Series releases keep one folder per episode: Subs/<episode>/.
			perVideo, err := readFolder(filepath.Join(folder, name))
			if err != nil {
				continue
			}
			for _, file := range perVideo {
				if file.Type().IsRegular() && isSubtitle(file.Name()) {
					found = append(found, filepath.Join(folder, name, file.Name()))
				}
			}
		}
	}
	return found
}

func readFolder(folder string) ([]os.DirEntry, error) {
	f, err := os.Open(folder)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// ReadDir with a limit reports io.EOF for an empty folder.
	entries, err := f.ReadDir(maxFolderEntries)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return entries, nil
}

func countVideos(entries []os.DirEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Type().IsRegular() && videoExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			count++
		}
	}
	return count
}

func isSubtitle(name string) bool {
	return subtitleExtensions[strings.ToLower(filepath.Ext(name))]
}

// namedAfter reports "Movie.en.srt" for "Movie", but not "Movie 2.srt".
func namedAfter(name, videoName string) bool {
	if videoName == "" || len(name) <= len(videoName) || !strings.EqualFold(name[:len(videoName)], videoName) {
		return false
	}
	return name[len(videoName)] == '.'
}

// Load reads and parses a subtitle file.
func (Files) Load(path string) (subtitle.Format, media.SubtitleContent, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", media.SubtitleContent{}, err
	}
	if !info.Mode().IsRegular() {
		return "", media.SubtitleContent{}, fmt.Errorf("%w: %s is not a file", ErrUnsupported, filepath.Base(path))
	}
	if info.Size() > subtitleformat.MaxInputBytes {
		return "", media.SubtitleContent{}, fmt.Errorf("%w: %s has %d bytes", ErrTooLarge, filepath.Base(path), info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", media.SubtitleContent{}, err
	}
	format, ok := subtitleformat.DetectFormat(path, data)
	if !ok {
		return "", media.SubtitleContent{}, fmt.Errorf("%w: %s", ErrUnsupported, filepath.Base(path))
	}
	parser, err := subtitleformat.NewParser(format)
	if err != nil {
		return "", media.SubtitleContent{}, err
	}
	result, err := parser.Parse(data)
	if err != nil {
		return "", media.SubtitleContent{}, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	return format, media.SubtitleContent{Events: result.Events, Skipped: result.Skipped}, nil
}
