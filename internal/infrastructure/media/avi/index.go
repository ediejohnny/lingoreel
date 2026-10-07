// Package avi reads what the app needs of an AVI file's layout: whether it
// carries the index a seek lands through. Only chunk headers and the header
// list are read, each within bounds.
package avi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNotAVI reports a file that does not start as an AVI.
var ErrNotAVI = errors.New("not an AVI file")

const (
	// maxChunks bounds the chunks walked at each level.
	maxChunks = 4096
	// maxHeader bounds the header list, read whole: it holds a few streams'
	// descriptions.
	maxHeader = 1 << 20
)

// Indexed tells whether an AVI file has an index: the idx1 chunk after its
// frames, or the OpenDML index of a stream in its header. Without one, a
// seek reads the file from its start.
func Indexed(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return false, fmt.Errorf("inspecting %s: %w", path, err)
	}
	if !stat.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	var head [12]byte
	if _, err := file.ReadAt(head[:], 0); err != nil || string(head[:4]) != "RIFF" || string(head[8:]) != "AVI " {
		return false, fmt.Errorf("%w: %s", ErrNotAVI, path)
	}
	end := min(8+int64(binary.LittleEndian.Uint32(head[4:8])), stat.Size())
	at := int64(len(head))
	for range maxChunks {
		var chunk [12]byte
		n, err := file.ReadAt(chunk[:], at)
		if n < 8 || at+8 > end {
			break
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("reading %s: %w", path, err)
		}
		id, length := string(chunk[:4]), int64(binary.LittleEndian.Uint32(chunk[4:8]))
		switch {
		case id == "idx1" && length > 0:
			return true, nil
		case id == "LIST" && n == len(chunk) && string(chunk[8:]) == "hdrl" && length > 4 && length <= maxHeader:
			header := make([]byte, length-4)
			n, _ := file.ReadAt(header, at+int64(len(chunk)))
			if streamIndexed(header[:n]) {
				return true, nil
			}
		}
		at += 8 + length + length&1
	}
	return false, nil
}

// streamIndexed tells whether a header list's stream lists hold an OpenDML
// index (indx).
func streamIndexed(header []byte) bool {
	return walk(header, func(id string, body []byte) bool {
		return id == "LIST" && len(body) >= 4 && string(body[:4]) == "strl" &&
			walk(body[4:], func(id string, body []byte) bool { return id == "indx" && len(body) > 0 })
	})
}

// walk visits the chunks laid one after another in data, a truncated last
// one with what is there, until visit is true; it tells whether it was.
func walk(data []byte, visit func(id string, body []byte) bool) bool {
	for range maxChunks {
		if len(data) < 8 {
			return false
		}
		length := min(int(binary.LittleEndian.Uint32(data[4:8])), len(data)-8)
		if visit(string(data[:4]), data[8:8+length]) {
			return true
		}
		next := 8 + length + length&1
		if next >= len(data) {
			return false
		}
		data = data[next:]
	}
	return false
}
