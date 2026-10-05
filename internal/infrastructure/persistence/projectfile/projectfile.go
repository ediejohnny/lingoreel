// Package projectfile keeps each video's project (project.Project) in a file
// beside the video, "Movie.dualsub.json", or in the app's folder when the
// video's folder takes no writing. The video itself is never touched.
package projectfile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
)

// Extension ends a project file's name.
const Extension = ".dualsub.json"

// fallbackFolder is where the projects go inside the app's folder.
const fallbackFolder = "projects"

// Store finds, reads and writes the project files.
type Store struct {
	fallbackDir string
}

// Open uses dir, the app's own folder, for the videos whose folder takes no
// writing; empty leaves them without a file.
func Open(dir string) *Store {
	if dir != "" {
		dir = filepath.Join(dir, fallbackFolder)
	}
	return &Store{fallbackDir: dir}
}

// Beside is the project file beside a video: its name with Extension.
func Beside(video string) string {
	return strings.TrimSuffix(video, filepath.Ext(video)) + Extension
}

// Fallback is the project file of a video in the app's folder, named by the
// video's full path; empty without a folder.
func (s *Store) Fallback(video string) string {
	if s.fallbackDir == "" {
		return ""
	}
	absolute, err := filepath.Abs(video)
	if err != nil {
		absolute = video
	}
	// Windows paths ignore case: one file per video however it is typed.
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(absolute))))
	return filepath.Join(s.fallbackDir, hex.EncodeToString(sum[:8])+Extension)
}

func storeAt(path string) *jsonstore.Store[project.Project] {
	return jsonstore.New(path, project.Version, project.New, migrations)
}

// migrations upgrade older project files, one version at a time.
var migrations = map[int]jsonstore.Migration{
	1: fromVersion1,
}

// fromVersion1 (version 1 to 2) needs no new value: a passage without a
// plan pauses at the end with no helps, and a video without "last" starts
// from the beginning with the profile's speed and Modo Karaokê. A passage
// that is not an object is still refused when the file is decoded.
func fromVersion1(map[string]any) error { return nil }

// Remove forgets a video's project: the file beside it and the one in the
// app's folder. A file that is not there is no error; a folder of that name
// is left alone, since it is no project.
func (s *Store) Remove(video string) error {
	var errs []error
	for _, path := range []string{Beside(video), s.Fallback(video)} {
		if path == "" {
			continue
		}
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			continue
		}
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("removing %s: %w", filepath.Base(path), err))
		}
	}
	return errors.Join(errs...)
}

// Load reads a video's project: the file beside it, or else the one in the
// app's folder; a video without either has an empty project. A file of a
// newer version is reported with project.ErrNewerVersion and must not be
// saved over.
func (s *Store) Load(video string) (project.Project, error) {
	for _, path := range []string{Beside(video), s.Fallback(video)} {
		if path == "" {
			continue
		}
		// Only a file counts: a folder of that name is no project.
		if info, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			continue
		}
		loaded, err := storeAt(path).Load()
		if errors.Is(err, jsonstore.ErrNewerVersion) {
			return project.New(), fmt.Errorf("%w: %w", project.ErrNewerVersion, err)
		}
		if err != nil {
			return project.New(), err
		}
		return withMaps(loaded), nil
	}
	return project.New(), nil
}

// Save writes a video's project beside it, or in the app's folder when that
// fails (a read-only folder, a disc, a network share without rights).
func (s *Store) Save(video string, p project.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	besideErr := storeAt(Beside(video)).Save(p)
	if besideErr == nil {
		return nil
	}
	fallback := s.Fallback(video)
	if fallback == "" {
		return besideErr
	}
	if err := storeAt(fallback).Save(p); err != nil {
		return errors.Join(besideErr, err)
	}
	return nil
}

// withMaps gives a loaded project the maps a file may leave out, so it can
// be changed at once.
func withMaps(p project.Project) project.Project {
	if p.Tracks == nil {
		p.Tracks = map[string]project.Track{}
	}
	for key, track := range p.Tracks {
		if track.Edits == nil {
			track.Edits = map[string]project.Edit{}
			p.Tracks[key] = track
		}
	}
	if p.Passages == nil {
		p.Passages = []project.Passage{}
	}
	return p
}

// TrackKey is the package's TrackKey, for the app's port.
func (s *Store) TrackKey(video string, track media.Track) string {
	return TrackKey(video, track)
}

// TrackKey names a track of a video so the name survives a new analysis: a
// file by its path from the video's folder, a track inside the video by its
// UID (stable when the file is remuxed), or by its number when it has none.
func TrackKey(video string, track media.Track) string {
	if track.External() {
		relative, err := filepath.Rel(filepath.Dir(video), track.Path)
		if err != nil || strings.HasPrefix(relative, "..") {
			relative = filepath.Base(track.Path)
		}
		return "file:" + filepath.ToSlash(relative)
	}
	if track.UID != 0 {
		return "uid:" + strconv.FormatUint(track.UID, 10)
	}
	return "track:" + strconv.FormatUint(track.Number, 10)
}
