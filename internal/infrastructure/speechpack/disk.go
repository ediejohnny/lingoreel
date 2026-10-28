package speechpack

import (
	"errors"
	"os"
	"path/filepath"
)

// Disk keeps the speech packs' files on this computer's disk.
type Disk struct{}

// FileSize tells a regular file's size; false when there is none.
func (Disk) FileSize(path string) (int64, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0, false
	}
	return info.Size(), true
}

// Exists tells something is at the path.
func (Disk) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteFile writes a small file whole, making its folder.
func (Disk) WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// RemoveAll removes a folder and what it holds; a missing one is no error.
func (Disk) RemoveAll(path string) error { return os.RemoveAll(path) }

// Remove removes a file; a missing one is no error.
func (Disk) Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
