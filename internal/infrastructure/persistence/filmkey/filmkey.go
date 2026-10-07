// Package filmkey names a film by its content, so that what the app keeps
// for it in its own folder survives the film being renamed or moved: its
// size and a SHA-256 of its first and last 64 KiB. Only those are read.
package filmkey

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// edge is how much of each end of the film is read.
const edge = 64 << 10

// Of names the film at path by its content: 32 hexadecimal digits.
func Of(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspecting %s: %w", path, err)
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	size := stat.Size()
	hash := sha256.New()
	_ = binary.Write(hash, binary.LittleEndian, size)
	head := min(size, edge)
	if _, err := io.Copy(hash, io.NewSectionReader(file, 0, head)); err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	if tail := min(size-head, edge); tail > 0 {
		if _, err := io.Copy(hash, io.NewSectionReader(file, size-tail, tail)); err != nil {
			return "", fmt.Errorf("reading %s: %w", path, err)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)[:16]), nil
}
