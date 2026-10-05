package matroska

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

// ErrNotReadableTrack reports a track that is not a readable text subtitle.
var ErrNotReadableTrack = errors.New("track is not a readable text subtitle")

const (
	// maxInferredCueDuration caps a cue whose end is taken from the next cue.
	maxInferredCueDuration = 10 * time.Second
	// lastCueDuration is used for a final cue that declares no duration.
	lastCueDuration = 3 * time.Second
	// maxSubtitleEvents bounds the events kept for one track.
	maxSubtitleEvents = 500_000
)

// ReadSubtitles reads the events of the given text subtitle tracks. It jumps
// straight to their blocks through the cue index when the file has one for
// every wanted track, and otherwise scans the clusters in a single pass.
// progress, when set, receives the work done so far and its total.
func ReadSubtitles(ctx context.Context, source io.ReaderAt, size int64, container Container, trackNumbers []uint64, progress func(done, total int64)) (map[uint64]media.SubtitleContent, error) {
	collectors, err := newCollectors(container, trackNumbers)
	if err != nil {
		return nil, err
	}
	wanted := make(map[uint64]bool, len(collectors))
	for number := range collectors {
		wanted[number] = true
	}
	collect := func(frame rawFrame) error {
		return collectors[frame.track].add(frame)
	}

	c := newCursor(source, size)
	refs, err := container.subtitleRefs(c, wanted)
	if err == nil {
		err = container.checkCoverage(c, refs, wanted)
	}
	if err == nil {
		err = container.readIndexed(ctx, c, refs, wanted, progress, collect)
	}
	if errors.Is(err, errIndexUnusable) {
		// Start over: the index may have delivered some frames already.
		if collectors, err = newCollectors(container, trackNumbers); err != nil {
			return nil, err
		}
		err = container.scanFrames(ctx, c, wanted, progress, collect)
	}
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]media.SubtitleContent, len(collectors))
	for number, collector := range collectors {
		result[number] = collector.finish()
	}
	return result, nil
}

func newCollectors(container Container, trackNumbers []uint64) (map[uint64]*cueCollector, error) {
	collectors := make(map[uint64]*cueCollector, len(trackNumbers))
	for _, number := range trackNumbers {
		entry, found := container.track(number)
		if !found {
			return nil, fmt.Errorf("%w: track %d does not exist", ErrNotReadableTrack, number)
		}
		format, readable := subtitleFormatOf(entry)
		if !readable {
			return nil, fmt.Errorf("%w: track %d (%s)", ErrNotReadableTrack, number, entry.CodecID)
		}
		decoder, err := newFrameDecoder(entry.Encodings)
		if err != nil {
			return nil, err
		}
		collectors[number] = &cueCollector{format: format, decoder: decoder, entry: entry, scale: container.TimestampScale}
	}
	return collectors, nil
}

// pendingCue is a cue whose end may still be unknown.
type pendingCue struct {
	id    string
	start time.Duration
	end   time.Duration
	text  string
}

// cueCollector turns the frames of one track into subtitle events.
type cueCollector struct {
	format  subtitle.Format
	decoder frameDecoder
	entry   TrackEntry
	scale   uint64
	cues    []pendingCue
	skipped int
	frames  int
}

func (collector *cueCollector) add(frame rawFrame) error {
	collector.frames++
	if frame.data == nil || frame.ticks < 0 {
		collector.skipped++
		return nil
	}
	if len(collector.cues) >= maxSubtitleEvents {
		return fmt.Errorf("%w: more than %d subtitle events in track %d", ErrTooLarge, maxSubtitleEvents, frame.track)
	}
	payload, err := collector.decoder.decode(frame.data)
	if err != nil {
		collector.skipped++
		return nil
	}
	text, err := subtitleformat.BlockText(collector.format, payload)
	if err != nil || text == "" {
		collector.skipped++
		return nil
	}

	start, ok := collector.toDuration(frame.ticks)
	if !ok {
		collector.skipped++
		return nil
	}
	var end time.Duration
	if frame.blockDuration > 0 {
		// Laced frames share the block duration equally.
		total, ok := collector.toDuration(frame.blockDuration)
		if ok {
			share := total / time.Duration(frame.laces)
			start += share * time.Duration(frame.lace)
			end = start + share
		}
	} else if collector.entry.DefaultDuration > 0 && collector.entry.DefaultDuration < math.MaxInt64/2 {
		share := time.Duration(collector.entry.DefaultDuration)
		start += share * time.Duration(frame.lace)
		end = start + share
	}
	collector.cues = append(collector.cues, pendingCue{id: strconv.Itoa(collector.frames), start: start, end: end, text: text})
	return nil
}

// toDuration converts ticks to a duration, refusing values that overflow.
func (collector *cueCollector) toDuration(ticks int64) (time.Duration, bool) {
	if ticks < 0 || collector.scale == 0 || uint64(ticks) > math.MaxInt64/collector.scale {
		return 0, false
	}
	return time.Duration(uint64(ticks) * collector.scale), true
}

// finish infers missing ends from the next cue and returns the valid events.
func (collector *cueCollector) finish() media.SubtitleContent {
	order := make([]int, len(collector.cues))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return collector.cues[order[a]].start < collector.cues[order[b]].start
	})
	for position, index := range order {
		cue := &collector.cues[index]
		if cue.end != 0 {
			continue
		}
		cue.end = cue.start + lastCueDuration
		for _, later := range order[position+1:] {
			if next := collector.cues[later].start; next > cue.start {
				cue.end = min(next, cue.start+maxInferredCueDuration)
				break
			}
		}
	}

	content := media.SubtitleContent{Skipped: collector.skipped, Events: make([]subtitle.Event, 0, len(collector.cues))}
	for _, cue := range collector.cues {
		event := subtitle.Event{ID: cue.id, Start: cue.start, End: cue.end, OriginalText: cue.text}
		if !event.Valid() {
			content.Skipped++
			continue
		}
		content.Events = append(content.Events, event)
	}
	return content
}
