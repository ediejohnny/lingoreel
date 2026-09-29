package matroska

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"
)

// defaultTimestampScale is the spec default: one timestamp tick is 1 ms.
const defaultTimestampScale = 1_000_000

// Limits that keep a hostile file from exhausting memory.
const (
	maxTracks        = 256
	maxSeekEntries   = 4096
	maxStringBytes   = 64 << 10
	maxDocTypeBytes  = 64
	maxSettingsBytes = 64 << 10
)

// Container is what Analyze learns about a Matroska file.
type Container struct {
	DocType string
	// TimestampScale is the number of nanoseconds in one timestamp tick.
	TimestampScale uint64
	Duration       time.Duration
	Tracks         []TrackEntry

	segmentDataStart int64
	segmentEnd       int64
	firstCluster     int64
}

// Analyze reads the header, segment information and tracks of a Matroska or
// WebM file without reading its clusters.
func Analyze(source io.ReaderAt, size int64) (Container, error) {
	c := newCursor(source, size)
	docType, err := readEBMLHeader(c)
	if err != nil {
		return Container{}, err
	}

	segment, err := c.readElement()
	if err != nil || segment.id != idSegment {
		return Container{}, fmt.Errorf("%w: missing Segment", ErrNotMatroska)
	}
	container := Container{
		DocType:          docType,
		TimestampScale:   defaultTimestampScale,
		segmentDataStart: segment.dataStart,
		segmentEnd:       size,
	}
	if !segment.unknownSize {
		container.segmentEnd = segment.end()
	}

	if err := container.readTopLevel(c); err != nil {
		return Container{}, err
	}
	if container.Tracks == nil {
		return Container{}, fmt.Errorf("%w: no Tracks element", ErrCorrupt)
	}
	return container, nil
}

// readEBMLHeader checks the file signature and returns the document type.
func readEBMLHeader(c *cursor) (string, error) {
	header, err := c.readElement()
	if err != nil || header.id != idEBML {
		return "", fmt.Errorf("%w: missing EBML header", ErrNotMatroska)
	}
	docType := ""
	err = c.forEachChild(header, func(child element) error {
		if child.id != idDocType {
			return nil
		}
		value, err := c.readString(child, maxDocTypeBytes)
		docType = value
		return err
	})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotMatroska, err)
	}
	if docType != "matroska" && docType != "webm" {
		return "", fmt.Errorf("%w: document type %q", ErrNotMatroska, docType)
	}
	return docType, nil
}

// readTopLevel walks the segment until Info and Tracks are known. When they
// come after the clusters, the SeekHead index is used to jump to them.
func (container *Container) readTopLevel(c *cursor) error {
	seekTargets := map[uint32]int64{}
	haveInfo := false

	for c.position() < container.segmentEnd {
		child, err := c.readElement()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		switch child.id {
		case idSeekHead:
			if err := container.readSeekHead(c, child, seekTargets); err != nil {
				return err
			}
		case idInfo:
			if err := container.readInfo(c, child); err != nil {
				return err
			}
			haveInfo = true
		case idTracks:
			if err := container.readTracks(c, child); err != nil {
				return err
			}
		case idCluster:
			if container.firstCluster == 0 {
				container.firstCluster = child.start
			}
			if (haveInfo && container.Tracks != nil) || child.unknownSize {
				return container.readIndexedElements(c, seekTargets, haveInfo)
			}
		default:
			if child.unknownSize {
				return fmt.Errorf("%w: element 0x%X cannot have an unknown size", ErrCorrupt, child.id)
			}
		}
		if err := c.seek(child.end()); err != nil {
			return err
		}
	}
	return container.readIndexedElements(c, seekTargets, haveInfo)
}

// readIndexedElements loads Info and Tracks through the SeekHead when the
// linear walk stopped before finding them.
func (container *Container) readIndexedElements(c *cursor, targets map[uint32]int64, haveInfo bool) error {
	if !haveInfo {
		if offset, found := targets[idInfo]; found {
			if err := container.readElementAt(c, offset, idInfo, container.readInfo); err != nil {
				return err
			}
		}
	}
	if container.Tracks == nil {
		if offset, found := targets[idTracks]; found {
			return container.readElementAt(c, offset, idTracks, container.readTracks)
		}
	}
	return nil
}

func (container *Container) readElementAt(c *cursor, offset int64, id uint32, read func(*cursor, element) error) error {
	if err := c.seek(offset); err != nil {
		return err
	}
	target, err := c.readElement()
	if err != nil {
		return err
	}
	if target.id != id {
		return fmt.Errorf("%w: SeekHead points to 0x%X instead of 0x%X", ErrCorrupt, target.id, id)
	}
	return read(c, target)
}

func (container *Container) readSeekHead(c *cursor, seekHead element, targets map[uint32]int64) error {
	entries := 0
	return c.forEachChild(seekHead, func(seek element) error {
		if seek.id != idSeek {
			return nil
		}
		if entries++; entries > maxSeekEntries {
			return fmt.Errorf("%w: too many SeekHead entries", ErrTooLarge)
		}
		var id uint32
		var position uint64
		err := c.forEachChild(seek, func(field element) error {
			switch field.id {
			case idSeekID:
				raw, err := c.readBytes(field, maxIDLength)
				for _, b := range raw {
					id = id<<8 | uint32(b)
				}
				return err
			case idSeekPos:
				value, err := c.readUint(field)
				position = value
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		if offset := container.segmentDataStart + int64(position); position < math.MaxInt64/2 && offset < c.size {
			targets[id] = offset
		}
		return nil
	})
}

func (container *Container) readInfo(c *cursor, info element) error {
	var durationTicks float64
	err := c.forEachChild(info, func(field element) error {
		switch field.id {
		case idTimescale:
			value, err := c.readUint(field)
			if err == nil && value > 0 {
				container.TimestampScale = value
			}
			return err
		case idDuration:
			value, err := c.readFloat(field)
			durationTicks = value
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	container.Duration = ticksToDuration(durationTicks, container.TimestampScale)
	return nil
}

// ticksToDuration converts a tick count to a duration, returning zero for
// values that are not finite, negative or too large to represent.
func ticksToDuration(ticks float64, scale uint64) time.Duration {
	nanoseconds := ticks * float64(scale)
	if math.IsNaN(nanoseconds) || nanoseconds < 0 || nanoseconds >= math.MaxInt64 {
		return 0
	}
	return time.Duration(nanoseconds)
}

func (container *Container) readTracks(c *cursor, tracks element) error {
	container.Tracks = []TrackEntry{}
	return c.forEachChild(tracks, func(entry element) error {
		if entry.id != idTrackEntry {
			return nil
		}
		if len(container.Tracks) >= maxTracks {
			return fmt.Errorf("%w: more than %d tracks", ErrTooLarge, maxTracks)
		}
		track, err := readTrackEntry(c, entry)
		if err != nil {
			return err
		}
		if track.Number != 0 {
			container.Tracks = append(container.Tracks, track)
		}
		return nil
	})
}
