package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
)

// ErrNotTheContainer reports a file that does not start as the video its
// name says: a page, a script or a program renamed "video.mp4".
var ErrNotTheContainer = errors.New("the file is not the video its name says")

// SignatureBytes is how much of a file's start MatchesContainer needs.
const SignatureBytes = 16

var (
	ebmlMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}
	asfHeader = []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	// isoFirstBoxes are the boxes an MP4 or a QuickTime movie starts with.
	isoFirstBoxes = map[string]bool{"ftyp": true, "moov": true, "mdat": true, "free": true, "skip": true, "wide": true, "pnot": true}
)

// MatchesContainer reports whether a file's start (head, its first
// SignatureBytes) is the container its extension names; size is the file's,
// which an MP4's first box must fit in. Only the containers the app opens
// match: Matroska and WebM, MP4 and QuickTime, AVI and ASF (WMV).
func MatchesContainer(extension string, head []byte, size int64) bool {
	switch strings.ToLower(extension) {
	case ".mkv", ".webm":
		return bytes.HasPrefix(head, ebmlMagic)
	case ".mp4", ".m4v", ".mov":
		return isoStart(head, size)
	case ".avi":
		return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "AVI "
	case ".wmv":
		return bytes.HasPrefix(head, asfHeader)
	}
	return false
}

// isoStart reports a known first box whose size fits the file: 0 runs to the
// end, 1 has its size in the next 8 bytes.
func isoStart(head []byte, size int64) bool {
	if len(head) < 8 || !isoFirstBoxes[string(head[4:8])] {
		return false
	}
	box := int64(binary.BigEndian.Uint32(head[:4]))
	return box == 0 || box == 1 || (box >= 8 && box <= size)
}
