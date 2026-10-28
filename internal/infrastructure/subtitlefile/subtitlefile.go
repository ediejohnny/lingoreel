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

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/capped"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

var (
	// ErrTooLarge reports a file larger than any real subtitle.
	ErrTooLarge = errors.New("subtitle file too large")
	// ErrUnsupported reports a file whose format is not a text subtitle.
	ErrUnsupported = errors.New("unsupported subtitle file")
)

const (
	// maxFolderEntries bounds how many entries of one folder are looked at,
	// so a video in a huge folder (Downloads) does not stall the analysis.
	maxFolderEntries = 5000
	// maxCandidateFolders bounds the subfolders searched for candidates.
	maxCandidateFolders = 50
	// maxCandidates bounds the candidates returned: each one is read and
	// compared with the video.
	maxCandidates = 50
	// maxSniffs bounds the files of one folder whose start is read to tell a
	// subtitle by its text, so a folder of thousands of files stays quick.
	maxSniffs = 100
	// sniffHead is how much of such a file is read: enough for the format's
	// first lines, never the whole file, however large it claims to be.
	sniffHead = 4096
)

var (
	subtitleExtensions = map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true}
	// sniffedExtensions are what a subtitle renamed by a user may end with:
	// nothing at all ("legenda") or plain text.
	sniffedExtensions = map[string]bool{"": true, ".txt": true}
	videoExtensions   = map[string]bool{".mkv": true, ".webm": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true, ".ts": true, ".m2ts": true}
	// subtitleFolders are folder names releases and users put subtitles in.
	subtitleFolders = map[string]bool{"subs": true, "sub": true, "subtitles": true, "subtitle": true, "legendas": true, "legenda": true}
)

// Files finds and reads subtitle files on disk.
type Files struct{}

// Find returns the subtitle files of a video: files named after it in its
// folder, or every subtitle file there when it is the folder's only video;
// the same inside a subtitles folder (Subs, Subtitles, Legendas), including a
// folder named after the video inside it; and every subtitle file in a folder
// named after the video. Files next to the video come first. A subtitle is
// told by its extension or, renamed without one, by its text (subtitles).
func (Files) Find(videoPath string) ([]string, error) {
	folder := filepath.Dir(videoPath)
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := readFolder(folder)
	if err != nil {
		return nil, fmt.Errorf("reading the video folder: %w", err)
	}
	onlyVideo := countVideos(entries) == 1

	var beside, inFolders []string
	test := subtitles{folder: folder}
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case (onlyVideo || namedAfter(name, videoName)) && test.is(entry):
			beside = append(beside, filepath.Join(folder, name))
		case entry.IsDir() && subtitleFolders[strings.ToLower(name)]:
			inFolders = append(inFolders, findInSubtitleFolder(filepath.Join(folder, name), videoName, onlyVideo)...)
		case entry.IsDir() && strings.EqualFold(name, videoName):
			inFolders = append(inFolders, subtitlesIn(filepath.Join(folder, name))...)
		}
	}
	sort.Strings(beside)
	sort.Strings(inFolders)
	return append(beside, inFolders...), nil
}

// FindCandidates returns subtitle files that may belong to the video, though
// nothing says so: the other ones in its folder and those one level inside
// any subfolder, such as the folder a downloaded zip was extracted to. A
// folder may hold several films, so they are only used once their timing
// matches the video. Files that Find returns are left out.
func (f Files) FindCandidates(videoPath string) ([]string, error) {
	own, err := f.Find(videoPath)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(own))
	for _, path := range own {
		known[strings.ToLower(path)] = true
	}
	var found []string
	add := func(paths []string) {
		for _, path := range paths {
			if !known[strings.ToLower(path)] && len(found) < maxCandidates {
				found = append(found, path)
			}
		}
	}

	folder := filepath.Dir(videoPath)
	entries, err := readFolder(folder)
	if err != nil {
		return nil, fmt.Errorf("reading the video folder: %w", err)
	}
	var beside, subfolders []string
	test := subtitles{folder: folder}
	for _, entry := range entries {
		switch {
		case test.is(entry):
			beside = append(beside, filepath.Join(folder, entry.Name()))
		case entry.IsDir():
			subfolders = append(subfolders, filepath.Join(folder, entry.Name()))
		}
	}
	sort.Strings(beside)
	add(beside)
	sort.Strings(subfolders)
	for _, subfolder := range subfolders[:min(len(subfolders), maxCandidateFolders)] {
		add(subtitlesIn(subfolder))
	}
	return found, nil
}

// subtitlesIn returns the subtitle files directly inside a folder, sorted;
// an unreadable folder only means fewer choices.
func subtitlesIn(folder string) []string {
	entries, err := readFolder(folder)
	if err != nil {
		return nil
	}
	var found []string
	test := subtitles{folder: folder}
	for _, entry := range entries {
		if test.is(entry) {
			found = append(found, filepath.Join(folder, entry.Name()))
		}
	}
	sort.Strings(found)
	return found
}

func findInSubtitleFolder(folder, videoName string, onlyVideo bool) []string {
	entries, err := readFolder(folder)
	if err != nil {
		return nil // an unreadable subtitles folder only means fewer choices
	}
	var found []string
	test := subtitles{folder: folder}
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case (onlyVideo || namedAfter(name, videoName)) && test.is(entry):
			found = append(found, filepath.Join(folder, name))
		case entry.IsDir() && strings.EqualFold(name, videoName):
			// Series releases keep one folder per episode: Subs/<episode>/.
			found = append(found, subtitlesIn(filepath.Join(folder, name))...)
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

// subtitles tells the subtitle files of one folder: by their extension, or,
// for a file a user renamed without one ("legenda") or to .txt, by the start
// of its text. Only maxSniffs files a folder are read so, only sniffHead bytes
// each, and only those small enough to be subtitles at all, by the size the
// folder listing gives.
type subtitles struct {
	folder  string
	sniffed int
}

func (s *subtitles) is(entry os.DirEntry) bool {
	if !entry.Type().IsRegular() {
		return false
	}
	extension := strings.ToLower(filepath.Ext(entry.Name()))
	if subtitleExtensions[extension] {
		return true
	}
	if !sniffedExtensions[extension] || s.sniffed >= maxSniffs {
		return false
	}
	info, err := entry.Info()
	if err != nil || info.Size() == 0 || info.Size() > subtitleformat.MaxInputBytes {
		return false
	}
	s.sniffed++
	return startsLikeASubtitle(filepath.Join(s.folder, entry.Name()))
}

// startsLikeASubtitle reads the start of a file and tells whether it is a
// subtitle by its text.
func startsLikeASubtitle(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	head := make([]byte, sniffHead)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false
	}
	_, ok := subtitleformat.DetectFormat("", head[:n])
	return ok
}

// namedAfter reports "Movie.en.srt" for "Movie", but not "Movie 2.srt".
func namedAfter(name, videoName string) bool {
	if videoName == "" || len(name) <= len(videoName) || !strings.EqualFold(name[:len(videoName)], videoName) {
		return false
	}
	return name[len(videoName)] == '.'
}

// Load reads and parses a subtitle file. Its content decides its format,
// whatever its name says: a page, a script or a program renamed "movie.srt"
// is refused (ErrUnsupported), and so is a file that gives no line.
func (Files) Load(path string) (subtitle.Format, media.SubtitleContent, error) {
	name := filepath.Base(path)
	data, err := readLimited(path)
	if err != nil {
		return "", media.SubtitleContent{}, err
	}
	format, ok := subtitleformat.DetectFormat(path, data)
	if !ok {
		return "", media.SubtitleContent{}, fmt.Errorf("%w: %s", ErrUnsupported, name)
	}
	parser, err := subtitleformat.NewParser(format)
	if err != nil {
		return "", media.SubtitleContent{}, fmt.Errorf("reading %s: %w", name, err)
	}
	result, err := parser.Parse(data)
	if err != nil {
		return "", media.SubtitleContent{}, fmt.Errorf("reading %s: %w", name, err)
	}
	// A file that only looked like a subtitle at its start gives no line.
	if len(result.Events) == 0 {
		return "", media.SubtitleContent{}, fmt.Errorf("%w: %s has no subtitle line", ErrUnsupported, name)
	}
	return format, media.SubtitleContent{Events: result.Events, Skipped: result.Skipped}, nil
}

// readLimited reads a regular file of at most the parsers' input limit.
func readLimited(path string) ([]byte, error) {
	data, err := capped.ReadFile(path, subtitleformat.MaxInputBytes)
	switch {
	case errors.Is(err, capped.ErrNotFile):
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	case errors.Is(err, capped.ErrTooLarge):
		return nil, fmt.Errorf("%w: %w", ErrTooLarge, err)
	}
	return data, err
}
