// Package capped reads a file whole only up to a limit: what the app reads
// of the user's disk is data of unknown size, never trusted to be small.
package capped

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	// ErrTooLarge reports a file past the limit.
	ErrTooLarge = errors.New("the file is larger than allowed")
	// ErrNotFile reports a path that is not a regular file (a folder, a
	// device).
	ErrNotFile = errors.New("the path is not a file")
)

// ReadFile reads a regular file of at most limit bytes. The limit holds on
// the bytes read, not only on the size seen before, so a file that grows in
// between cannot pass it.
func ReadFile(path string, limit int64) ([]byte, error) {
	name := filepath.Base(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspecting %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		// Like a file that cannot be opened, a matter of the disk, not of what
		// the file holds.
		return nil, &fs.PathError{Op: "read", Path: name, Err: ErrNotFile}
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s, over %d bytes", ErrTooLarge, name, limit)
	}
	return data, nil
}
