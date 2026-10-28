// Package projectfile keeps each video's project (project.Project) in the
// app's own folder, named by the film's content (filmkey), so it survives
// the film being renamed or moved. Nothing is written beside the film: a
// project an older version left there ("Movie.dualsub.json") or in the app's
// folder by the film's path is moved in on the first opening. The video
// itself is never touched.
package projectfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/filmkey"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
)

// Extension ends a project file's name.
const Extension = ".dualsub.json"

// markExtension ends the file that holds, for a film's path, the key of the
// film last kept from it.
const markExtension = ".film"

// folder is where the projects go inside the app's folder.
const folder = "projects"

// markBytes bounds what is read of a mark: a key and a little more.
const markBytes = 64

// ErrNoFolder reports a store without the app's folder: it keeps nothing.
var ErrNoFolder = errors.New("no folder for the projects")

// Store finds, reads and writes the project files.
type Store struct {
	dir string

	mu    sync.Mutex
	films map[string]film   // by path, the key last found
	marks map[string]string // by path, the key its mark is known to hold
}

// film is a film's key with what it was found for: a film written since has
// its key found again.
type film struct {
	size     int64
	modified time.Time
	key      string
}

// Open keeps the projects in dir, the app's own folder; empty keeps none.
func Open(dir string) *Store {
	if dir != "" {
		dir = filepath.Join(dir, folder)
	}
	return &Store{dir: dir, films: map[string]film{}, marks: map[string]string{}}
}

// Beside is where an older version kept a video's project: beside it, with
// its name and Extension. It is only read, to move the project in.
func Beside(video string) string {
	return strings.TrimSuffix(video, filepath.Ext(video)) + Extension
}

// File is the project file of a video in the app's folder, named by the
// film's content; it fails when the film cannot be read.
func (s *Store) File(video string) (string, error) {
	key, err := s.key(video)
	if err != nil {
		return "", err
	}
	return s.keyed(key), nil
}

func (s *Store) keyed(key string) string {
	return filepath.Join(s.dir, key+Extension)
}

// pathName names the files the app's folder keeps by a film's full path.
// Windows paths ignore case: one name however the path is typed.
func (s *Store) pathName(video, extension string) string {
	if s.dir == "" {
		return ""
	}
	absolute, err := filepath.Abs(video)
	if err != nil {
		absolute = video
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(absolute))))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:8])+extension)
}

// legacy is where an older version kept a video's project in the app's
// folder, named by its path, when the video's folder took no writing.
func (s *Store) legacy(video string) string {
	return s.pathName(video, Extension)
}

// key names the film by its content, found again only when the film changed.
func (s *Store) key(video string) (string, error) {
	if s.dir == "" {
		return "", ErrNoFolder
	}
	info, err := os.Stat(video)
	if err != nil {
		return "", err
	}
	id := strings.ToLower(filepath.Clean(video))
	s.mu.Lock()
	known, found := s.films[id]
	s.mu.Unlock()
	if found && known.size == info.Size() && known.modified.Equal(info.ModTime()) {
		return known.key, nil
	}
	key, err := filmkey.Of(video)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.films[id] = film{size: info.Size(), modified: info.ModTime(), key: key}
	s.mu.Unlock()
	return key, nil
}

// mark reads what the film's path last kept: the key, or "" for a path
// whose project was forgotten (or a mark that is not a key). A path with a
// mark no longer reads the files an older version left.
func (s *Store) mark(video string) (key string, marked bool) {
	path := s.pathName(video, markExtension)
	if path == "" {
		return "", false
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, markBytes))
	if err != nil {
		return "", true
	}
	key = strings.TrimSpace(string(data))
	if _, err := hex.DecodeString(key); err != nil || len(key) != 32 {
		return "", true
	}
	return key, true
}

// setMark writes the key the film's path last kept. It is too small to be
// torn, and a mark that is not a key reads as a forgotten project.
func (s *Store) setMark(video, key string) error {
	id := strings.ToLower(filepath.Clean(video))
	s.mu.Lock()
	known, found := s.marks[id]
	s.mu.Unlock()
	if found && known == key {
		return nil
	}
	if current, marked := s.mark(video); !marked || current != key {
		path := s.pathName(video, markExtension)
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.marks[id] = key
	s.mu.Unlock()
	return nil
}

func storeAt(path string) *jsonstore.Store[project.Project] {
	return jsonstore.New(path, project.Version, project.New, migrations)
}

// migrations upgrade older project files, one version at a time.
var migrations = map[int]jsonstore.Migration{
	1: fromVersion1,
	2: fromVersion2,
	3: fromVersion3,
}

// fromVersion1 (version 1 to 2) needs no new value: a passage without a
// plan pauses at the end with no helps, and a video without "last" starts
// from the beginning with the profile's speed and karaoke mode. A passage
// that is not an object is still refused when the file is decoded.
func fromVersion1(map[string]any) error { return nil }

// fromVersion2 (version 2 to 3) needs no new value either: a video without
// "addedFiles" has none.
func fromVersion2(map[string]any) error { return nil }

// fromVersion3 (version 3 to 4) needs no new value either: an audio track
// without "hearing" was not heard yet.
func fromVersion3(map[string]any) error { return nil }

// Load reads a video's project: the file of its content in the app's folder;
// else, for a film changed since its path last kept one, the project of what
// it was; else a file an older version left (beside the video, or in the
// app's folder by its path), which is moved in. A video without any has an
// empty project. A file of a newer version is reported with
// project.ErrNewerVersion and must not be saved over.
func (s *Store) Load(video string) (project.Project, error) {
	key, keyErr := s.key(video)
	if keyErr == nil {
		if p, found, err := read(s.keyed(key)); found {
			return p, err
		}
	}
	if marked, found := s.mark(video); found {
		if marked != "" && marked != key {
			if p, found, err := read(s.keyed(marked)); found {
				return p, err
			}
		}
		return project.New(), nil
	}
	for _, path := range []string{Beside(video), s.legacy(video)} {
		p, found, err := read(path)
		if !found {
			continue
		}
		if err == nil && keyErr == nil {
			// The project stays in use from where it was if it cannot move.
			_ = s.moveIn(video, key, p)
		}
		return p, err
	}
	return project.New(), nil
}

// read loads a project file; found is false when there is none (a folder of
// that name is no project).
func read(path string) (p project.Project, found bool, err error) {
	if path == "" {
		return project.New(), false, nil
	}
	if info, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return project.New(), false, nil
	}
	loaded, err := storeAt(path).Load()
	if errors.Is(err, jsonstore.ErrNewerVersion) {
		return project.New(), true, fmt.Errorf("%w: %w", project.ErrNewerVersion, err)
	}
	if err != nil {
		return project.New(), true, err
	}
	return withMaps(loaded), true, nil
}

// moveIn writes a project an older version left into the app's folder, reads
// it back, and only when it came back whole marks the path and removes the
// old files. One that cannot be removed (a read-only folder) stays, unread.
func (s *Store) moveIn(video, key string, p project.Project) error {
	target := s.keyed(key)
	if err := storeAt(target).Save(p); err != nil {
		return err
	}
	back, found, err := read(target)
	if err == nil && (!found || !same(p, back)) {
		err = fmt.Errorf("%s came back changed", filepath.Base(target))
	}
	if err != nil {
		_ = os.Remove(target)
		return err
	}
	if err := s.setMark(video, key); err != nil {
		return err
	}
	for _, path := range []string{Beside(video), s.legacy(video)} {
		removeFile(path)
	}
	return nil
}

// same tells two projects that encode alike.
func same(a, b project.Project) bool {
	first, err := json.Marshal(a)
	if err != nil {
		return false
	}
	second, err := json.Marshal(b)
	return err == nil && bytes.Equal(first, second)
}

// removeFile removes a regular file; anything else, or nothing, stays.
func removeFile(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return nil
	}
	if err == nil {
		err = os.Remove(path)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", filepath.Base(path), err)
	}
	return nil
}

// Save writes a video's project in the app's folder, by the film's content,
// and marks the film's path with it.
func (s *Store) Save(video string, p project.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	key, err := s.key(video)
	if err != nil {
		return fmt.Errorf("naming %s: %w", filepath.Base(video), err)
	}
	if err := storeAt(s.keyed(key)).Save(p); err != nil {
		return err
	}
	// Kept without the mark, the project is still found by the film's content.
	_ = s.setMark(video, key)
	return nil
}

// Remove forgets a video's project. Its path is marked as forgotten, so a
// file an older version left and that cannot be removed is no longer read.
// A file that is not there is no error.
func (s *Store) Remove(video string) error {
	if s.dir == "" {
		return nil
	}
	var errs []error
	key, err := s.key(video)
	if err == nil {
		if err := removeFile(s.keyed(key)); err != nil {
			errs = append(errs, err)
		}
	}
	var left []error
	for _, path := range []string{Beside(video), s.legacy(video)} {
		if err := removeFile(path); err != nil {
			left = append(left, err)
		}
	}
	if err := s.setMark(video, ""); err != nil {
		errs = append(append(errs, err), left...)
	}
	return errors.Join(errs...)
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

// AddedPath is the package's AddedPath, for the app's port.
func (s *Store) AddedPath(video, path string) string {
	return AddedPath(video, path)
}

// ResolveAdded is the package's ResolveAdded, for the app's port.
func (s *Store) ResolveAdded(video, added string) string {
	return ResolveAdded(video, added)
}

// TrackKey names a track of a video so the name survives a new analysis: a
// file by its path (AddedPath), a track inside the video by its UID (stable
// when the file is remuxed), or by its number when it has none.
func TrackKey(video string, track media.Track) string {
	if track.External() {
		return "file:" + AddedPath(video, track.Path)
	}
	if track.UID != 0 {
		return "uid:" + strconv.FormatUint(track.UID, 10)
	}
	return "track:" + strconv.FormatUint(track.Number, 10)
}

// AddedPath is how the project writes a subtitle file's path: from the
// video's folder ("Subs/Filme.en.srt", "../Legendas/Filme.en.srt"), or whole
// on another drive, always with forward slashes.
func AddedPath(video, path string) string {
	relative, err := filepath.Rel(filepath.Dir(video), path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(relative)
}

// ResolveAdded is the file a path written by AddedPath names, for a video.
func ResolveAdded(video, added string) string {
	path := filepath.FromSlash(added)
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(filepath.Dir(video), path)
}
