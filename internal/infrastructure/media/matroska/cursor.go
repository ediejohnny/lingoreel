package matroska

import (
	"errors"
	"fmt"
	"io"
)

// windowSize is how much the cursor reads at once. It is small on purpose:
// video frames are skipped without being read, so a large buffer would only
// pull frame data from disk that is never used.
const windowSize = 8 << 10

// cursor reads a file front to back through a small window. Skipping moves the
// position without I/O.
type cursor struct {
	source      io.ReaderAt
	size        int64
	pos         int64
	window      []byte
	windowStart int64
	buffer      []byte
}

func newCursor(source io.ReaderAt, size int64) *cursor {
	return &cursor{source: source, size: size, buffer: make([]byte, windowSize)}
}

func (c *cursor) position() int64 {
	return c.pos
}

func (c *cursor) atEnd() bool {
	return c.pos >= c.size
}

// seek moves to an absolute offset inside the file.
func (c *cursor) seek(offset int64) error {
	if offset < 0 || offset > c.size {
		return fmt.Errorf("%w: offset %d outside the file", ErrCorrupt, offset)
	}
	c.pos = offset
	return nil
}

// skip moves forward by n bytes.
func (c *cursor) skip(n int64) error {
	if n < 0 || n > c.size-c.pos {
		return fmt.Errorf("%w: element runs past the end of the file", ErrCorrupt)
	}
	c.pos += n
	return nil
}

func (c *cursor) readByte() (byte, error) {
	var one [1]byte
	if err := c.readFull(one[:]); err != nil {
		return 0, err
	}
	return one[0], nil
}

// readFull fills p or fails with io.EOF when nothing was left, or
// io.ErrUnexpectedEOF when the file ended in the middle.
func (c *cursor) readFull(p []byte) error {
	read := 0
	for read < len(p) {
		offset := c.pos - c.windowStart
		if offset < 0 || offset >= int64(len(c.window)) {
			if err := c.fill(); err != nil {
				if read > 0 && errors.Is(err, io.EOF) {
					return io.ErrUnexpectedEOF
				}
				return err
			}
			continue
		}
		n := copy(p[read:], c.window[offset:])
		read += n
		c.pos += int64(n)
	}
	return nil
}

func (c *cursor) fill() error {
	if c.pos >= c.size {
		return io.EOF
	}
	length := min(int64(len(c.buffer)), c.size-c.pos)
	n, err := c.source.ReadAt(c.buffer[:length], c.pos)
	if n == 0 {
		if err == nil || errors.Is(err, io.EOF) {
			return io.ErrUnexpectedEOF
		}
		return fmt.Errorf("reading at offset %d: %w", c.pos, err)
	}
	c.window = c.buffer[:n]
	c.windowStart = c.pos
	return nil
}
