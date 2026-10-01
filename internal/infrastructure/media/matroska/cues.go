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
		// Entries without a relative position sort first in their cluster, so a
		// scanned cluster is scanned before its other entries come up; those
		// entries are skipped, since the scan already delivered their blocks.
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
		} else if !scanned[ref.cluster] {
			if err := readIndexedBlock(c, current, ref.relative, ticks, wanted, counting); err != nil {
				return err
			}
		}
		if frames == 0 && !scanned[ref.cluster] {
			return errIndexUnusable
		}
		reportProgress(progress, int64(i+1), int64(len(refs)))
	}
	return nil
}

// Coverage check limits: how many clusters outside the index are sampled,
// and how many elements are skipped looking for the cluster after a position.
const (
	maxCoverageSamples = 6
	maxElementsToNext  = 16
)

// errBlockOutsideIndex marks a wanted block found in a cluster the index
// leaves out.
var errBlockOutsideIndex = errors.New("subtitle block outside the cue index")

// checkCoverage samples clusters the index leaves out: the first cluster, the
// one after the last indexed cluster and the ones after the widest gaps. A
// wanted block in any of them means the index stops early or skips parts of
// the file, and reading through it would silently lose lines.
func (container Container) checkCoverage(c *cursor, refs []cueRef, wanted map[uint64]bool) error {
	indexed := map[int64]bool{}
	var positions []int64
	for _, ref := range refs {
		at := container.segmentDataStart + ref.cluster
		if !indexed[at] {
			indexed[at] = true
			positions = append(positions, at)
		}
	}
	if len(positions) == 0 {
		return errIndexUnusable
	}
	// refs are sorted by cluster, so positions are too.
	starts := []int64{positions[len(positions)-1]}
	gaps := make([]int, 0, len(positions))
	for i := 0; i+1 < len(positions); i++ {
		gaps = append(gaps, i)
	}
	sort.SliceStable(gaps, func(a, b int) bool {
		return positions[gaps[a]+1]-positions[gaps[a]] > positions[gaps[b]+1]-positions[gaps[b]]
	})
	for _, gap := range gaps[:min(len(gaps), maxCoverageSamples-2)] {
		starts = append(starts, positions[gap])
	}

	samples := map[int64]bool{}
	if first := container.firstCluster; first != 0 && !indexed[first] {
		samples[first] = true
	}
	for _, start := range starts {
		if next, found := container.nextCluster(c, start); found && !indexed[next] {
			samples[next] = true
		}
	}
	for at := range samples {
		if err := container.sampleCluster(c, at, wanted); err != nil {
			return errIndexUnusable
		}
	}
	return nil
}

// nextCluster returns the start of the first cluster after the cluster at a
// file position, skipping a few other top-level elements.
func (container Container) nextCluster(c *cursor, at int64) (int64, bool) {
	if err := c.seek(at); err != nil {
		return 0, false
	}
	current, err := c.readElement()
	if err != nil || current.unknownSize {
		return 0, false
	}
	position := current.end()
	for range maxElementsToNext {
		if position >= container.segmentEnd || c.seek(position) != nil {
			return 0, false
		}
		child, err := c.readElement()
		if err != nil || child.unknownSize {
			return 0, false
		}
		if child.id == idCluster {
			return child.start, true
		}
		position = child.end()
	}
	return 0, false
}

// sampleCluster fails when the cluster at a file position holds a block of a
// wanted track.
func (container Container) sampleCluster(c *cursor, at int64, wanted map[uint64]bool) error {
	if err := c.seek(at); err != nil {
		return err
	}
	cluster, err := c.readElement()
	if err != nil {
		return err
	}
	if cluster.id != idCluster {
		return fmt.Errorf("%w: 0x%X is not a cluster", ErrCorrupt, cluster.id)
	}
	return container.scanCluster(c, cluster, wanted, func(rawFrame) error {
		return errBlockOutsideIndex
	})
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
