// Package jsonstore persists versioned JSON documents atomically, upgrading old
// documents step by step through migrations.
package jsonstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxDocumentBytes bounds the size of a document accepted by Load.
const MaxDocumentBytes = 8 << 20

// versionField is the top-level field that stores the document version.
const versionField = "version"

// Renames can fail briefly on Windows while another process (an antivirus,
// for instance) holds the target open.
const (
	renameAttempts = 5
	renameBackoff  = 20 * time.Millisecond
)

const (
	directoryPermissions = 0o700
	indentation          = "  "
)

// StaleTemporary is how old a temporary file must be to be taken for one an
// app that stopped while writing left behind: a write never takes that long.
const StaleTemporary = time.Hour

var (
	ErrCorrupt          = errors.New("corrupt document")
	ErrNewerVersion     = errors.New("document written by a newer version")
	ErrMissingMigration = errors.New("missing migration")
	ErrReservedField    = errors.New("value uses the reserved version field")
	// ErrInvalid reports a document whose values its own Validate rejects.
	ErrInvalid = errors.New("document has invalid values")
)

// validator is implemented by values that check themselves, such as the
// settings; Load runs it so a hand-edited file cannot slip in bad values.
type validator interface {
	Validate() error
}

// Migration upgrades a decoded document from one version to the next, in place.
type Migration func(document map[string]any) error

// Store reads and writes one JSON document holding a value of type T.
type Store[T any] struct {
	path       string
	version    int
	defaults   func() T
	migrations map[int]Migration
}

// New creates a store for the document at path. version is the current
// document version; migrations[n] upgrades a document from version n to n+1.
func New[T any](path string, version int, defaults func() T, migrations map[int]Migration) *Store[T] {
	return &Store[T]{path: path, version: version, defaults: defaults, migrations: migrations}
}

// Path returns the location of the document.
func (s *Store[T]) Path() string {
	return s.path
}

// Load returns the stored value, or the defaults when the document does not
// exist yet. Fields missing from the document keep their default values.
func (s *Store[T]) Load() (T, error) {
	value := s.defaults()
	SweepTemporaries(s.path, time.Now())

	data, err := readLimited(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return value, nil
	}
	if err != nil {
		return value, err
	}

	document, version, err := decodeDocument(data)
	if err != nil {
		return value, fmt.Errorf("%s: %w", s.path, err)
	}
	if version > s.version {
		return value, fmt.Errorf("%s: %w (version %d, supported %d)", s.path, ErrNewerVersion, version, s.version)
	}
	for current := version; current < s.version; current++ {
		migrate, found := s.migrations[current]
		if !found {
			return value, fmt.Errorf("%s: %w from version %d", s.path, ErrMissingMigration, current)
		}
		if err := migrate(document); err != nil {
			return value, fmt.Errorf("%s: migrating from version %d: %w", s.path, current, err)
		}
	}

	delete(document, versionField)
	upgraded, err := json.Marshal(document)
	if err != nil {
		return s.defaults(), fmt.Errorf("%s: %w: %v", s.path, ErrCorrupt, err)
	}
	if err := json.Unmarshal(upgraded, &value); err != nil {
		return s.defaults(), fmt.Errorf("%s: %w: %v", s.path, ErrCorrupt, err)
	}
	if check, validates := any(value).(validator); validates {
		if err := check.Validate(); err != nil {
			return s.defaults(), fmt.Errorf("%s: %w: %w", s.path, ErrInvalid, err)
		}
	}
	return value, nil
}

// Save writes the value with the current version. The document is written to
// a temporary file in the same directory and renamed over the target, so a
// crash never leaves a half-written file.
func (s *Store[T]) Save(value T) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", s.path, err)
	}
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("encoding %s: value must be a JSON object: %w", s.path, err)
	}
	if _, taken := document[versionField]; taken {
		return fmt.Errorf("encoding %s: %w", s.path, ErrReservedField)
	}
	document[versionField] = s.version

	body, err := json.MarshalIndent(document, "", indentation)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", s.path, err)
	}
	return writeAtomically(s.path, append(body, '\n'))
}

// SweepTemporaries takes out the temporary files ("."+name+".*.tmp") an
// app that stopped while writing path left beside it, StaleTemporary old or
// more at now; a write on its way is younger and stays. Failures are left
// for the next time.
func SweepTemporaries(path string, now time.Time) {
	directory, prefix := filepath.Dir(path), "."+filepath.Base(path)+"."
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".tmp") {
			continue
		}
		if info, err := entry.Info(); err == nil && now.Sub(info.ModTime()) >= StaleTemporary {
			_ = os.Remove(filepath.Join(directory, name))
		}
	}
}

func readLimited(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(data) > MaxDocumentBytes {
		return nil, fmt.Errorf("%s: %w: larger than %d bytes", path, ErrCorrupt, MaxDocumentBytes)
	}
	return data, nil
}

// decodeDocument parses a JSON object and its positive integer version.
func decodeDocument(data []byte) (map[string]any, int, error) {
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil || document == nil {
		return nil, 0, fmt.Errorf("%w: not a JSON object", ErrCorrupt)
	}
	if decoder.More() {
		return nil, 0, fmt.Errorf("%w: trailing data after the document", ErrCorrupt)
	}

	number, isNumber := document[versionField].(json.Number)
	if !isNumber {
		return nil, 0, fmt.Errorf("%w: missing numeric %q field", ErrCorrupt, versionField)
	}
	version, err := number.Int64()
	if err != nil || version < 1 {
		return nil, 0, fmt.Errorf("%w: invalid version %q", ErrCorrupt, number)
	}
	return document, int(version), nil
}

func writeAtomically(path string, body []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, directoryPermissions); err != nil {
		return fmt.Errorf("creating %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file for %s: %w", path, err)
	}
	renamed := false
	defer func() {
		if !renamed {
			os.Remove(temporary.Name())
		}
	}()

	if _, err := temporary.Write(body); err != nil {
		temporary.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("flushing %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	if err := renameWithRetry(temporary.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	renamed = true
	return nil
}

func renameWithRetry(from, to string) error {
	var err error
	for attempt := range renameAttempts {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		if attempt+1 < renameAttempts {
			time.Sleep(renameBackoff * time.Duration(attempt+1))
		}
	}
	return err
}
