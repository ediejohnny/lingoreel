package matroska_test

import (
	"bytes"
	"context"
	"testing"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

// FuzzMatroska feeds arbitrary bytes to the analyzer and, when it accepts
// them, reads every readable subtitle track. Nothing may panic, and every
// produced event must be valid for a timeline.
func FuzzMatroska(f *testing.F) {
	f.Add(movieFile())
	f.Add(subtitleFile(cluster(0, simpleBlock(2, 0, "a"), blockGroup(2, 10, 5, []byte("b")))))
	f.Add(subtitleFile(cluster(0, lacedBlockGroup(2, 0, 30, 0x06, join([]byte{2}, encodeVint(1), []byte{0xC0}, []byte("abcdef"))))))
	f.Add(join(ebmlHeader("matroska"), unknownSizeEl(idSegment, tracksEl(trackEntry(2, typeSubtitle, "S_TEXT/ASS")),
		unknownSizeEl(idCluster, uintEl(idClusterTimestamp, 1), simpleBlock(2, 0, "1,0,a,,0,0,0,,x")))))
	// Files with a cue index, so the fuzzer mutates positions the reader
	// follows.
	f.Add(indexedFile(sampleClusters, completeIndex))
	f.Add(indexedFile(sampleClusters, []cueEntry{{track: 2, cluster: 0, block: 1, noRelative: true}, {track: 3, cluster: 1, block: 2}}))

	f.Fuzz(func(t *testing.T, data []byte) {
		container, err := matroska.Analyze(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		var readable []uint64
		for _, track := range container.MediaInfo().TracksOf(media.TrackSubtitle) {
			if track.Readable {
				readable = append(readable, track.Number)
			}
		}
		if len(readable) == 0 {
			return
		}
		content, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, readable, nil)
		if err != nil {
			return
		}
		for number, track := range content {
			for _, event := range track.Events {
				if !event.Valid() || !utf8.ValidString(event.OriginalText) {
					t.Fatalf("track %d produced an invalid event %+v", number, event)
				}
			}
			if _, err := subtitle.NewTimeline(track.Events); err != nil {
				t.Fatalf("track %d: timeline rejected the events: %v", number, err)
			}
		}
	})
}
