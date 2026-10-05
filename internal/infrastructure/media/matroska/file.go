package matroska

import (
	"context"
	"fmt"
	"os"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
)

// FileReader reads Matroska and WebM files from disk. Files are opened read
// only; the original is never modified.
type FileReader struct{}

// Analyze returns the tracks and duration of the file at path.
func (FileReader) Analyze(path string) (media.Info, error) {
	file, size, err := openReadOnly(path)
	if err != nil {
		return media.Info{}, err
	}
	defer file.Close()

	container, err := Analyze(file, size)
	if err != nil {
		return media.Info{}, fmt.Errorf("analyzing %s: %w", path, err)
	}
	return container.MediaInfo(), nil
}

// ReadSubtitles reads the events of the given subtitle tracks of the file at path.
func (FileReader) ReadSubtitles(ctx context.Context, path string, trackNumbers []uint64, progress func(done, total int64)) (map[uint64]media.SubtitleContent, error) {
	file, size, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	container, err := Analyze(file, size)
	if err != nil {
		return nil, fmt.Errorf("analyzing %s: %w", path, err)
	}
	content, err := ReadSubtitles(ctx, file, size, container, trackNumbers, progress)
	if err != nil {
		return nil, fmt.Errorf("reading subtitles of %s: %w", path, err)
	}
	return content, nil
}

// Check confirms that path is a regular file the player can open, for videos
// whose tracks this reader does not read.
func (FileReader) Check(path string) error {
	file, _, err := openReadOnly(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func openReadOnly(path string) (*os.File, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("opening %s: %w", path, err)
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("inspecting %s: %w", path, err)
	}
	if !stat.Mode().IsRegular() {
		file.Close()
		return nil, 0, fmt.Errorf("%s is not a regular file", path)
	}
	return file, stat.Size(), nil
}
