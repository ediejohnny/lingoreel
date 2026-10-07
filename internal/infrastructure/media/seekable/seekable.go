// Package seekable tells whether a video has the index a seek lands
// through; without it, a seek reads the file from its start.
package seekable

import (
	"path/filepath"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/avi"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

// Indexed tells whether a video has its index: the Cues of an MKV, the idx1
// or OpenDML index of an AVI; MP4 and WMV always carry theirs.
func Indexed(file string) (bool, error) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".mkv", ".webm":
		return matroska.FileReader{}.IndexesVideo(file)
	case ".avi":
		return avi.Indexed(file)
	}
	return true, nil
}
