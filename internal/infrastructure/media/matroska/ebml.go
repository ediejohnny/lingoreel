package matroska

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strings"
	"unicode"
)

var (
	// ErrNotMatroska reports a file that is not Matroska or WebM.
	ErrNotMatroska = errors.New("not a matroska file")
	// ErrCorrupt reports a structure that breaks the EBML rules.
	ErrCorrupt = errors.New("corrupt matroska file")
	// ErrTooLarge reports an element larger than the reader accepts.
	ErrTooLarge = errors.New("matroska element too large")
	// ErrUnsupportedEncoding reports compression or encryption the reader cannot undo.
	ErrUnsupportedEncoding = errors.New("unsupported matroska content encoding")
)

const (
	maxIDLength   = 4
	maxSizeLength = 8
	maxUintLength = 8
	float32Length = 4
	float64Length = 8
)

// element is a parsed element header.
type element struct {
	id        uint32
	start     int64
	dataStart int64
	size      int64
	// unknownSize is set for live-style Segments and Clusters whose end is only
	// known when the next top-level element appears.
	unknownSize bool
}

func (e element) end() int64 {
	return e.dataStart + e.size
}

// readElement reads an element header at the current position.
func (c *cursor) readElement() (element, error) {
	start := c.pos
	id, err := c.readID()
	if err != nil {
		return element{}, err
	}
	size, unknown, err := c.readVint()
	if err != nil {
		return element{}, err
	}
	header := element{id: id, start: start, dataStart: c.pos, unknownSize: unknown}
	if !unknown {
		if size > uint64(c.size-c.pos) {
			return element{}, fmt.Errorf("%w: element 0x%X at %d runs past the end of the file", ErrCorrupt, id, start)
		}
		header.size = int64(size)
	}
	return header, nil
}

// readID reads an element ID keeping its length marker, as IDs are written in
// the specification.
func (c *cursor) readID() (uint32, error) {
	first, err := c.readByte()
	if err != nil {
		return 0, err
	}
	length := bits.LeadingZeros8(first) + 1
	if first == 0 || length > maxIDLength {
		return 0, fmt.Errorf("%w: invalid element ID at %d", ErrCorrupt, c.pos-1)
	}
	id := uint32(first)
	for range length - 1 {
		next, err := c.readByte()
		if err != nil {
			return 0, err
		}
		id = id<<8 | uint32(next)
	}
	return id, nil
}

// readVint reads a variable length integer without its marker. A value with
// every data bit set means "unknown", used for sizes of streaming elements.
func (c *cursor) readVint() (uint64, bool, error) {
	var raw [maxSizeLength]byte
	first, err := c.readByte()
	if err != nil {
		return 0, false, err
	}
	length := vintLength(first)
	if length == 0 {
		return 0, false, fmt.Errorf("%w: invalid variable size integer at %d", ErrCorrupt, c.pos-1)
	}
	raw[0] = first
	if err := c.readFull(raw[1:length]); err != nil {
		return 0, false, err
	}
	value, _, unknown := parseVint(raw[:length])
	return value, unknown, nil
}

// vintLength returns the encoded length announced by the first byte of a
// variable size integer, or 0 when the byte is invalid.
func vintLength(first byte) int {
	length := bits.LeadingZeros8(first) + 1
	if first == 0 || length > maxSizeLength {
		return 0
	}
	return length
}

// parseVint decodes the variable size integer at the start of data and
// returns its value without the marker, its encoded length (0 if data is too
// short or invalid) and whether every data bit was set.
func parseVint(data []byte) (uint64, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}
	length := vintLength(data[0])
	if length == 0 || length > len(data) {
		return 0, 0, false
	}
	value := uint64(data[0] & (0xFF >> length))
	allOnes := value == uint64(0xFF>>length)
	for _, next := range data[1:length] {
		value = value<<8 | uint64(next)
		allOnes = allOnes && next == 0xFF
	}
	return value, length, allOnes
}

func (c *cursor) readUint(e element) (uint64, error) {
	if e.size > maxUintLength {
		return 0, fmt.Errorf("%w: integer of %d bytes", ErrCorrupt, e.size)
	}
	var raw [maxUintLength]byte
	if err := c.readFull(raw[maxUintLength-e.size:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(raw[:]), nil
}

func (c *cursor) readFloat(e element) (float64, error) {
	switch e.size {
	case 0:
		return 0, nil
	case float32Length:
		var raw [float32Length]byte
		if err := c.readFull(raw[:]); err != nil {
			return 0, err
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(raw[:]))), nil
	case float64Length:
		var raw [float64Length]byte
		if err := c.readFull(raw[:]); err != nil {
			return 0, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(raw[:])), nil
	}
	return 0, fmt.Errorf("%w: float of %d bytes", ErrCorrupt, e.size)
}

func (c *cursor) readBytes(e element, limit int64) ([]byte, error) {
	if e.size > limit {
		return nil, fmt.Errorf("%w: element 0x%X has %d bytes (limit %d)", ErrTooLarge, e.id, e.size, limit)
	}
	data := make([]byte, e.size)
	if err := c.readFull(data); err != nil {
		return nil, err
	}
	return data, nil
}

// readString reads a UTF-8 string, dropping the zero padding Matroska allows
// and any control character.
func (c *cursor) readString(e element, limit int64) (string, error) {
	data, err := c.readBytes(e, limit)
	if err != nil {
		return "", err
	}
	text := strings.ToValidUTF8(strings.TrimRight(string(data), "\x00"), string(unicode.ReplacementChar))
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text), nil
}

// forEachChild visits the children of a master element of known size.
func (c *cursor) forEachChild(parent element, visit func(child element) error) error {
	if parent.unknownSize {
		return fmt.Errorf("%w: element 0x%X cannot have an unknown size", ErrCorrupt, parent.id)
	}
	if err := c.seek(parent.dataStart); err != nil {
		return err
	}
	for c.pos < parent.end() {
		child, err := c.readElement()
		if err != nil {
			return err
		}
		if child.unknownSize || child.end() > parent.end() {
			return fmt.Errorf("%w: element 0x%X overflows its parent 0x%X", ErrCorrupt, child.id, parent.id)
		}
		if err := visit(child); err != nil {
			return err
		}
		// Visitors may read the child or not; always continue after it.
		if err := c.seek(child.end()); err != nil {
			return err
		}
	}
	return nil
}
