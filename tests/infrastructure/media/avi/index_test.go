package avi_test

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/avi"
)

// chunk lays out a RIFF chunk, padded to an even length.
func chunk(id string, body ...[]byte) []byte {
	var data []byte
	for _, part := range body {
		data = append(data, part...)
	}
	out := binary.LittleEndian.AppendUint32([]byte(id), uint32(len(data)))
	out = append(out, data...)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func list(kind string, children ...[]byte) []byte {
	return chunk("LIST", append([][]byte{[]byte(kind)}, children...)...)
}

func riff(children ...[]byte) []byte {
	return chunk("RIFF", append([][]byte{[]byte("AVI ")}, children...)...)
}

func header(streamChunks ...[]byte) []byte {
	return list("hdrl", chunk("avih", make([]byte, 56)), list("strl", append([][]byte{chunk("strh", make([]byte, 56))}, streamChunks...)...))
}

var frames = list("movi", chunk("00dc", []byte("frame one")), chunk("00dc", []byte("frame two!")))

// An AVI seeks through its idx1 or the OpenDML index of a stream; without
// either, or with an empty one, a seek reads the file from its start. Only
// a real AVI is read.
func TestAnAVIsIndexIsFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put := func(name string, data []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	broken := riff(header(), chunk("JUNK", []byte("odd")))
	broken = append(broken, []byte("LIST\xff\xff\xff\x7fmovi")...)
	binary.LittleEndian.PutUint32(broken[4:], uint32(len(broken)-8))
	for _, c := range []struct {
		name string
		data []byte
		want bool
	}{
		{"idx1.avi", riff(header(), chunk("JUNK", []byte("odd")), frames, chunk("idx1", make([]byte, 32))), true},
		{"opendml.avi", riff(header(chunk("strf", make([]byte, 40)), chunk("indx", make([]byte, 24))), frames), true},
		{"none.avi", riff(header(chunk("strf", make([]byte, 40))), frames), false},
		{"empty.avi", riff(header(), frames, chunk("idx1")), false},
		{"broken.avi", broken, false},
	} {
		if got, err := avi.Indexed(put(c.name, c.data)); err != nil || got != c.want {
			t.Errorf("%s: indexed %v (%v), want %v", c.name, got, err, c.want)
		}
	}
	for _, name := range []string{"riff.avi", "short.avi"} {
		data := []byte("RIFF\x04\x00\x00\x00WAVE")
		if name == "short.avi" {
			data = []byte("RIFF")
		}
		if _, err := avi.Indexed(put(name, data)); !errors.Is(err, avi.ErrNotAVI) {
			t.Errorf("%s: %v, want not an AVI", name, err)
		}
	}
}
