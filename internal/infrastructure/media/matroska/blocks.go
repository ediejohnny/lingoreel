package matroska

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// maxFrameBytes bounds a subtitle block read into memory; larger blocks are
// reported as dropped.
const maxFrameBytes = 1 << 20

// blockHeaderBytes are the relative timestamp (2 bytes) and the flags (1 byte)
// that follow the track number in every block.
const blockHeaderBytes = 3

// rawFrame is one frame of a wanted track, in file order.
type rawFrame struct {
	track uint64
	// ticks is the absolute block timestamp in TimestampScale units.
	ticks int64
	// blockDuration is in ticks; zero when the block declares none.
	blockDuration int64
	lace          int
	laces         int
	// data is nil when the frame was dropped (too large or badly laced).
	data []byte
}

// scanFrames walks the clusters from the first one to the end of the segment
// and hands every frame of the wanted tracks to visit. Frames of other tracks
// are skipped without being read.
func (container Container) scanFrames(ctx context.Context, c *cursor, wanted map[uint64]bool, progress func(done, total int64), visit func(rawFrame) error) error {
	start := container.firstCluster
	if start == 0 {
		start = container.segmentDataStart
	}
	if err := c.seek(start); err != nil {
		return err
	}
	for c.position() < container.segmentEnd {
		if err := ctx.Err(); err != nil {
			return err
		}
		child, err := c.readElement()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if child.id == idCluster {
			if err := container.scanCluster(c, child, wanted, visit); err != nil {
				return err
			}
			reportProgress(progress, c.position(), c.size)
			continue
		}
		if child.unknownSize {
			return fmt.Errorf("%w: element 0x%X cannot have an unknown size", ErrCorrupt, child.id)
		}
		if err := c.seek(child.end()); err != nil {
			return err
		}
	}
	reportProgress(progress, c.size, c.size)
	return nil
}

func reportProgress(progress func(done, total int64), done, total int64) {
	if progress != nil {
		progress(done, total)
	}
}

func (container Container) scanCluster(c *cursor, cluster element, wanted map[uint64]bool, visit func(rawFrame) error) error {
	end := cluster.end()
	if cluster.unknownSize {
		end = container.segmentEnd
	}
	var clusterTicks int64
	for c.position() < end {
		childStart := c.position()
		child, err := c.readElement()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if cluster.unknownSize && topLevelIDs[child.id] {
			// The next top-level element closes an unknown-size cluster.
			return c.seek(childStart)
		}
		if child.unknownSize || child.end() > end {
			return fmt.Errorf("%w: cluster child 0x%X overflows the cluster", ErrCorrupt, child.id)
		}

		switch child.id {
		case idClusterTimestamp:
			value, err := c.readUint(child)
			if err != nil {
				return err
			}
			clusterTicks = int64(min(value, math.MaxInt64/2))
		case idSimpleBlock:
			if err := readBlock(c, child, clusterTicks, 0, wanted, visit); err != nil {
				return err
			}
		case idBlockGroup:
			if err := readBlockGroup(c, child, clusterTicks, wanted, visit); err != nil {
				return err
			}
		}
		if err := c.seek(child.end()); err != nil {
			return err
		}
	}
	return nil
}

// readBlockGroup finds the group's Block and BlockDuration, in any order, and
// reads the block with its duration.
func readBlockGroup(c *cursor, group element, clusterTicks int64, wanted map[uint64]bool, visit func(rawFrame) error) error {
	var block element
	var duration uint64
	found := false
	err := c.forEachChild(group, func(child element) error {
		switch child.id {
		case idBlock:
			block, found = child, true
		case idBlockDuration:
			value, err := c.readUint(child)
			duration = value
			return err
		}
		return nil
	})
	if err != nil || !found {
		return err
	}
	if err := c.seek(block.dataStart); err != nil {
		return err
	}
	return readBlock(c, block, clusterTicks, int64(min(duration, math.MaxInt64/2)), wanted, visit)
}

// readBlock reads one Block or SimpleBlock. The payload of unwanted tracks is
// never read; corrupt block headers are skipped like unwanted blocks.
func readBlock(c *cursor, block element, clusterTicks, durationTicks int64, wanted map[uint64]bool, visit func(rawFrame) error) error {
	track, _, err := c.readVint()
	if err != nil || !wanted[track] {
		return nil
	}
	var header [blockHeaderBytes]byte
	if err := c.readFull(header[:]); err != nil {
		return nil
	}
	relative := int64(int16(binary.BigEndian.Uint16(header[:2])))
	flags := header[2]
	frame := rawFrame{track: track, ticks: clusterTicks + relative, blockDuration: durationTicks, laces: 1}

	payloadSize := block.end() - c.position()
	if payloadSize < 0 || payloadSize > maxFrameBytes {
		return visit(frame)
	}
	payload := make([]byte, payloadSize)
	if err := c.readFull(payload); err != nil {
		return err
	}
	laces, ok := splitLaces(flags, payload)
	if !ok {
		return visit(frame)
	}
	frame.laces = len(laces)
	for index, data := range laces {
		frame.lace, frame.data = index, data
		if err := visit(frame); err != nil {
			return err
		}
	}
	return nil
}
