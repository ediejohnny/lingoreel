package matroska_test

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

const (
	idCues                = 0x1C53BB6B
	idCuePoint            = 0xBB
	idCueTime             = 0xB3
	idCueTrackPositions   = 0xB7
	idCueTrack            = 0xF7
	idCueClusterPosition  = 0xF1
	idCueRelativePosition = 0xF0
)

type testCluster struct {
	timestamp uint64
	blocks    [][]byte
}

// cueEntry indexes one block; the deltas corrupt the recorded positions.
type cueEntry struct {
	track         uint64
	cluster       int
	block         int
	noRelative    bool
	clusterDelta  int64
	relativeDelta int64
}

func indexedTracks() []byte {
	return tracksEl(
		trackEntry(1, typeVideo, "V_MPEG4/ISO/AVC"),
		trackEntry(2, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng")),
		trackEntry(3, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "por")),
	)
}

// seekHead points at the Cues with a fixed-width position, so its size does
// not depend on where the Cues end up.
func seekHead(cuesPosition int64) []byte {
	return el(idSeekHead, el(idSeek, el(idSeekID, encodeID(idCues)), fixedUintEl(idSeekPosition, uint64(cuesPosition))))
}

// indexedFile lays out SeekHead, Info, Tracks, the clusters and a Cues
// element built from entries, computing every position as a muxer would.
func indexedFile(clusters []testCluster, entries []cueEntry) []byte {
	info := segmentInfo(1_000_000, 60_000)
	tracks := indexedTracks()
	offset := int64(len(seekHead(0)) + len(info) + len(tracks))

	var built [][]byte
	var clusterAt []int64
	var blockAt [][]int64
	for _, cl := range clusters {
		relative := int64(len(uintEl(idClusterTimestamp, cl.timestamp)))
		var positions []int64
		for _, block := range cl.blocks {
			positions = append(positions, relative)
			relative += int64(len(block))
		}
		bytes := cluster(cl.timestamp, cl.blocks...)
		built = append(built, bytes)
		clusterAt = append(clusterAt, offset)
		blockAt = append(blockAt, positions)
		offset += int64(len(bytes))
	}

	var points [][]byte
	for _, e := range entries {
		fields := [][]byte{
			uintEl(idCueTrack, e.track),
			uintEl(idCueClusterPosition, uint64(clusterAt[e.cluster]+e.clusterDelta)),
		}
		if !e.noRelative {
			fields = append(fields, uintEl(idCueRelativePosition, uint64(blockAt[e.cluster][e.block]+e.relativeDelta)))
		}
		points = append(points, el(idCuePoint, uintEl(idCueTime, 0), el(idCueTrackPositions, fields...)))
	}
	children := append([][]byte{seekHead(offset), info, tracks}, built...)
	return matroskaFile(append(children, el(idCues, points...))...)
}

// plainFile has the same clusters and no index.
func plainFile(clusters []testCluster) []byte {
	children := [][]byte{segmentInfo(1_000_000, 60_000), indexedTracks()}
	for _, cl := range clusters {
		children = append(children, cluster(cl.timestamp, cl.blocks...))
	}
	return matroskaFile(children...)
}

var sampleClusters = []testCluster{
	{0, [][]byte{
		simpleBlock(1, 0, "video frame"),
		blockGroup(2, 1000, 1500, []byte("Hello there.")),
		simpleBlock(3, 1200, "Olá."),
		simpleBlock(1, 40, "video frame"),
	}},
	{5000, [][]byte{
		simpleBlock(1, 0, "video frame"),
		blockGroup(2, 500, 1000, []byte("Second line.")),
		blockGroup(3, 700, 800, []byte("Segunda fala.")),
	}},
}

// completeIndex indexes every subtitle block, like mkvmerge, plus video
// keyframes that must be ignored.
var completeIndex = []cueEntry{
	{track: 1, cluster: 0, block: 0},
	{track: 2, cluster: 0, block: 1},
	{track: 3, cluster: 0, block: 2},
	{track: 1, cluster: 1, block: 0},
	{track: 2, cluster: 1, block: 1},
	{track: 3, cluster: 1, block: 2},
}

// readWithProgress returns the tracks and the last progress total: the number
// of indexed blocks when the index was used, the file size after a scan.
func readWithProgress(t *testing.T, data []byte, tracks ...uint64) (map[uint64]media.SubtitleContent, int64) {
	t.Helper()
	container := analyze(t, data)
	var total int64
	content, err := matroska.ReadSubtitles(context.Background(), bytes.NewReader(data), int64(len(data)), container, tracks,
		func(_, all int64) { total = all })
	if err != nil {
		t.Fatalf("ReadSubtitles: %v", err)
	}
	return content, total
}

func assertSameAsScan(t *testing.T, got map[uint64]media.SubtitleContent, tracks ...uint64) {
	t.Helper()
	want := readTracks(t, plainFile(sampleClusters), tracks...)
	for _, track := range tracks {
		if !reflect.DeepEqual(got[track], want[track]) {
			t.Fatalf("track %d:\n got  %+v\n want %+v (full scan)", track, got[track], want[track])
		}
	}
	if len(want[2].Events)+len(want[3].Events) == 0 {
		t.Fatal("the reference scan found nothing; the test would prove nothing")
	}
}

func TestIndexedReadMatchesTheFullScan(t *testing.T) {
	t.Parallel()
	data := indexedFile(sampleClusters, completeIndex)
	got, total := readWithProgress(t, data, 2, 3)
	if total != 4 {
		t.Fatalf("progress total = %d, want 4 indexed subtitle blocks (the index was not used)", total)
	}
	assertSameAsScan(t, got, 2, 3)
}

func TestIndexWithoutRelativePositionsScansOnlyTheClusters(t *testing.T) {
	t.Parallel()
	entries := make([]cueEntry, len(completeIndex))
	copy(entries, completeIndex)
	for i := range entries {
		entries[i].noRelative = true
	}
	got, total := readWithProgress(t, indexedFile(sampleClusters, entries), 2, 3)
	if total != 4 {
		t.Fatalf("progress total = %d, want 4 (index used, clusters scanned)", total)
	}
	assertSameAsScan(t, got, 2, 3)
}

// An index may mix entries with and without a relative position inside one
// cluster. The entry without one makes the reader scan the whole cluster, so
// the other entries of that cluster must not deliver their blocks again.
func TestMixedEntriesInOneClusterDoNotDuplicateLines(t *testing.T) {
	t.Parallel()
	entries := []cueEntry{
		{track: 2, cluster: 0, block: 1, noRelative: true},
		{track: 3, cluster: 0, block: 2},
		{track: 2, cluster: 1, block: 1},
		{track: 3, cluster: 1, block: 2, noRelative: true},
	}
	got, total := readWithProgress(t, indexedFile(sampleClusters, entries), 2, 3)
	if total != 4 {
		t.Fatalf("progress total = %d, want 4 (the index was not used)", total)
	}
	assertSameAsScan(t, got, 2, 3)
}

// An index that covers every wanted track but skips whole clusters of
// subtitles would silently lose lines: the reader samples the clusters left
// out of the index and falls back to the full scan when they hold subtitles.
func TestIndexSkippingSubtitleClustersFallsBackToTheFullScan(t *testing.T) {
	t.Parallel()
	firstClusterOnly := []cueEntry{
		{track: 2, cluster: 0, block: 1},
		{track: 3, cluster: 0, block: 2},
	}
	data := indexedFile(sampleClusters, firstClusterOnly)
	got, total := readWithProgress(t, data, 2, 3)
	if total != int64(len(data)) {
		t.Fatalf("progress total = %d, want the file size %d (full scan)", total, len(data))
	}
	assertSameAsScan(t, got, 2, 3)
}

// Clusters left out of the index that hold only video are fine: the index
// stays in use.
func TestIndexSkippingVideoOnlyClustersIsKept(t *testing.T) {
	t.Parallel()
	clusters := append([]testCluster{}, sampleClusters...)
	clusters = append(clusters, testCluster{9000, [][]byte{simpleBlock(1, 0, "video frame")}})
	data := indexedFile(clusters, completeIndex)
	_, total := readWithProgress(t, data, 2, 3)
	if total != 4 {
		t.Fatalf("progress total = %d, want 4 (a video-only cluster must not reject the index)", total)
	}
}

// A broken or partial index must never lose lines: the reader falls back to
// the full scan, whose progress total is the file size.
func TestUnusableIndexFallsBackToTheFullScan(t *testing.T) {
	t.Parallel()
	onlyEnglish := []cueEntry{{track: 2, cluster: 0, block: 1}, {track: 2, cluster: 1, block: 1}}
	cases := map[string][]cueEntry{
		"no entry for the track asked": onlyEnglish,
		"cluster shifted":              {{track: 2, cluster: 0, block: 1, clusterDelta: 1}, {track: 3, cluster: 0, block: 2}},
		"block shifted":                {{track: 2, cluster: 0, block: 1, relativeDelta: 2}, {track: 3, cluster: 0, block: 2}},
		"points to the video":          {{track: 2, cluster: 0, block: 0}, {track: 3, cluster: 0, block: 2}},
		"points to the other track":    {{track: 2, cluster: 0, block: 2}, {track: 3, cluster: 0, block: 1}},
		"absurd position":              {{track: 2, cluster: 0, block: 1, clusterDelta: 1 << 40}, {track: 3, cluster: 0, block: 2}},
		"empty index":                  nil,
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			data := indexedFile(sampleClusters, entries)
			got, total := readWithProgress(t, data, 2, 3)
			if total != int64(len(data)) {
				t.Fatalf("progress total = %d, want the file size %d (full scan)", total, len(data))
			}
			assertSameAsScan(t, got, 2, 3)
		})
	}
}

// "Aponta para a outra" swaps the tracks: each entry leads to a block of the
// other wanted track. The block exists and is wanted, so only the track check
// catches it; reading one track alone must reject it too.
func TestIndexPointingAtAnotherWantedTrackIsRejected(t *testing.T) {
	t.Parallel()
	swapped := []cueEntry{{track: 2, cluster: 0, block: 2}, {track: 3, cluster: 0, block: 1}}
	data := indexedFile(sampleClusters, swapped)
	got, total := readWithProgress(t, data, 2)
	if total != int64(len(data)) {
		t.Fatalf("progress total = %d, want the file size (full scan)", total)
	}
	assertSameAsScan(t, got, 2)
}
