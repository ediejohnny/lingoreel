package matroska

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
)

// errIndexUnusable sends ReadSubtitles back to the full scan: the file has
// no cue index for a wanted track, or the index does not point at its blocks.
var errIndexUnusable = errors.New("cue index unusable")

// maxCueRefs bounds the index entries kept for the wanted tracks.
const maxCueRefs = 2_000_000

// unknownRelative marks an entry without CueRelativePosition: the whole
// cluster is scanned for it.
const unknownRelative = -1

// cueRef locates one indexed block: its cluster, relative to the segment
// data, and the block, relative to the cluster data.
type cueRef struct {
	cluster  int64
	relative int64
	// track is the track the index says the block belongs to.
	track uint64
}

// subtitleRefs reads the Cues and returns the positions of the wanted tracks'
// blocks in file order. Muxers such as mkvmerge and FFmpeg index every
// subtitle block, which lets a subtitle track be read without scanning a
// multi-gigabyte file.
func (container Container) subtitleRefs(c *cursor, wanted map[uint64]bool) ([]cueRef, error) {
	if container.cuesOffset == 0 {
		return nil, errIndexUnusable
	}
	if err := c.seek(container.cuesOffset); err != nil {
		return nil, errIndexUnusable
	}
	cues, err := c.readElement()
	if err != nil || cues.id != idCues || cues.unknownSize {
		return nil, errIndexUnusable
	}

	seen := map[cueRef]bool{}
	covered := map[uint64]bool{}
	var refs []cueRef
	err = c.forEachChild(cues, func(point element) error {
		if point.id != idCuePoint {
			return nil
		}
		return c.forEachChild(point, func(positions element) error {
			if positions.id != idCueTrackPositions {
				return nil
			}
			ref, err := readCuePositions(c, positions)
			if err != nil || !wanted[ref.track] {
				return err
			}
			covered[ref.track] = true
			if seen[ref] {
				return nil
			}
			if len(refs) >= maxCueRefs {
				return fmt.Errorf("%w: too many cue entries", ErrTooLarge)
			}
			seen[ref] = true
			refs = append(refs, ref)
			return nil
		})
	})
	if err != nil {
		return nil, errIndexUnusable
	}
	for track := range wanted {
		if !covered[track] {
			return nil, errIndexUnusable
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].cluster != refs[j].cluster {
			return refs[i].cluster < refs[j].cluster
		}
		return refs[i].relative < refs[j].relative
	})
	return refs, nil
}

func readCuePositions(c *cursor, positions element) (cueRef, error) {
	ref := cueRef{cluster: -1, relative: unknownRelative}
	err := c.forEachChild(positions, func(field element) error {
		switch field.id {
		case idCueTrack, idCueClusterPosition, idCueRelativePosition:
			value, err := c.readUint(field)
			if err != nil {
				return err
			}
			if value > math.MaxInt64/2 {
				return fmt.Errorf("%w: cue position out of range", ErrCorrupt)
			}
			switch field.id {
			case idCueTrack:
				ref.track = value
			case idCueClusterPosition:
				ref.cluster = int64(value)
			default:
				ref.relative = int64(value)
			}
		}
		return nil
	})
	if err == nil && ref.cluster < 0 {
		err = fmt.Errorf("%w: cue without a cluster position", ErrCorrupt)
	}
	return ref, err
}

// readIndexed reads the indexed blocks. Every entry must lead to a block of
// a wanted track; anything else means the index cannot be trusted.
func (container Container) readIndexed(ctx context.Context, c *cursor, refs []cueRef, wanted map[uint64]bool, progress func(done, total int64), visit func(rawFrame) error) error {
	var current element
	var ticks int64
	currentCluster := int64(-1)
	scanned := map[int64]bool{}
	for i, ref := range refs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ref.cluster != currentCluster {
			cluster, clusterTicks, err := container.clusterAt(c, ref.cluster)
			if err != nil {
				return errIndexUnusable
			}
			current, ticks, currentCluster = cluster, clusterTicks, ref.cluster
		}

		// An entry must lead to a block of the track it names; a block of
		// another wanted track means the index is wrong and may skip lines.
		frames := 0
		counting := func(frame rawFrame) error {
			if frame.track == ref.track {
				frames++
			}
			return visit(frame)
		}
		if ref.relative == unknownRelative {
			if !scanned[ref.cluster] {
				scanned[ref.cluster] = true
				if err := c.seek(current.dataStart); err != nil {
					return errIndexUnusable
				}
				if err := container.scanCluster(c, current, wanted, counting); err != nil {
					return err
				}
			}
		} else if err := readIndexedBlock(c, current, ref.relative, ticks, wanted, counting); err != nil {
			return err
		}
		if frames == 0 && !scanned[ref.cluster] {
			return errIndexUnusable
		}
		reportProgress(progress, int64(i+1), int64(len(refs)))
	}
	return nil
}

func readIndexedBlock(c *cursor, cluster element, relative, ticks int64, wanted map[uint64]bool, visit func(rawFrame) error) error {
	position := cluster.dataStart + relative
	if !cluster.unknownSize && position >= cluster.end() {
		return errIndexUnusable
	}
	if err := c.seek(position); err != nil {
		return errIndexUnusable
	}
	block, err := c.readElement()
	if err != nil || block.unknownSize {
		return errIndexUnusable
	}
	switch block.id {
	case idSimpleBlock:
		return readBlock(c, block, ticks, 0, wanted, visit)
	case idBlockGroup:
		return readBlockGroup(c, block, ticks, wanted, visit)
	}
	return errIndexUnusable
}

// clusterAt reads the cluster header at a segment-relative offset and its
// timestamp, which precedes its blocks.
func (container Container) clusterAt(c *cursor, offset int64) (element, int64, error) {
	if err := c.seek(container.segmentDataStart + offset); err != nil {
		return element{}, 0, err
	}
	cluster, err := c.readElement()
	if err != nil {
		return element{}, 0, err
	}
	if cluster.id != idCluster {
		return element{}, 0, fmt.Errorf("%w: cue points to 0x%X, not a cluster", ErrCorrupt, cluster.id)
	}
	end := cluster.end()
	if cluster.unknownSize {
		end = container.segmentEnd
	}
	for c.position() < end {
		child, err := c.readElement()
		if err != nil {
			return element{}, 0, err
		}
		switch child.id {
		case idClusterTimestamp:
			value, err := c.readUint(child)
			return cluster, int64(min(value, math.MaxInt64/2)), err
		case idSimpleBlock, idBlockGroup:
			return element{}, 0, fmt.Errorf("%w: cluster without a timestamp", ErrCorrupt)
		}
		if child.unknownSize {
			return element{}, 0, fmt.Errorf("%w: unknown-size cluster child", ErrCorrupt)
		}
		if err := c.seek(child.end()); err != nil {
			return element{}, 0, err
		}
	}
	return element{}, 0, fmt.Errorf("%w: cluster without a timestamp", ErrCorrupt)
}
