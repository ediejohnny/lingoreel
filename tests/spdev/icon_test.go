package spdev_test

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/spdev"
)

// The icon file Windows reads from the executable: every picture where its
// entry says, the small ones as bitmaps twice their height (color and mask),
// the 256-pixel one as a PNG; and the picture Wails takes, 1024 pixels.
func TestTheIconIsAWellFormedWindowsIcon(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	var log bytes.Buffer
	if err := spdev.Run(t.Context(), []string{"icon", "--out", out}, &log); err != nil {
		t.Fatalf("icon: %v (%s)", err, log.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "windows", "icon.ico"))
	if err != nil {
		t.Fatal(err)
	}
	var header struct{ Reserved, Type, Count uint16 }
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &header); err != nil || header.Type != 1 || header.Count < 8 {
		t.Fatalf("header %+v, %v", header, err)
	}
	sizes := map[int]bool{}
	for i := range int(header.Count) {
		var entry struct {
			Width, Height, Colors, Reserved uint8
			Planes, BitCount                uint16
			Size, Offset                    uint32
		}
		if err := binary.Read(bytes.NewReader(data[6+16*i:]), binary.LittleEndian, &entry); err != nil {
			t.Fatal(err)
		}
		size := int(entry.Width)
		if size == 0 {
			size = 256
		}
		end := int(entry.Offset) + int(entry.Size)
		if entry.Width != entry.Height || entry.BitCount != 32 || end > len(data) {
			t.Fatalf("entry %d: %+v in a file of %d bytes", i, entry, len(data))
		}
		picture := data[entry.Offset:end]
		if size == 256 {
			img, err := png.Decode(bytes.NewReader(picture))
			if err != nil || img.Bounds().Dx() != 256 {
				t.Fatalf("the 256-pixel picture: %v", err)
			}
		} else {
			var bitmap struct {
				Size          uint32
				Width, Height int32
			}
			_ = binary.Read(bytes.NewReader(picture), binary.LittleEndian, &bitmap)
			maskRow := (size + 31) / 32 * 4
			if bitmap.Size != 40 || int(bitmap.Width) != size || int(bitmap.Height) != 2*size || len(picture) != 40+size*size*4+maskRow*size {
				t.Fatalf("the %d-pixel bitmap: %+v, %d bytes", size, bitmap, len(picture))
			}
		}
		sizes[size] = true
	}
	for _, want := range []int{16, 32, 48, 256} {
		if !sizes[want] {
			t.Errorf("no %d-pixel picture", want)
		}
	}
	file, err := os.Open(filepath.Join(out, "appicon.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if img, err := png.Decode(file); err != nil || img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatalf("appicon.png: %v", err)
	}
}
