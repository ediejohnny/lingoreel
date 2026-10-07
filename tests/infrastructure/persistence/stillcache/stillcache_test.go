package stillcache_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/filmkey"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/stillcache"
)

// film writes a video file of its own content.
func film(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(content, 1000)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// stills makes n kept stills of size bytes each, asked each 10 s.
func stills(n, size int) []thumbnail.Kept {
	var out []thumbnail.Kept
	for i := range n {
		jpeg := append([]byte{0xFF, 0xD8}, bytes.Repeat([]byte{byte(i)}, size-2)...)
		asked := time.Duration(i) * 10 * time.Second
		out = append(out, thumbnail.Kept{Asked: asked, Picture: thumbnail.Picture{At: asked - time.Second/2*time.Duration(i%2),
			Width: 384, Height: 216, JPEG: jpeg}})
	}
	return out
}

func packageOf(t *testing.T, dir, file string) string {
	t.Helper()
	key, err := filmkey.Of(file)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, key+".stills")
}

// A film's stills come back as they were kept, also when the film was
// renamed and moved; another film has none, and a film kept again keeps
// only its new stills, never a package half written.
func TestAFilmsStillsComeBackWhereverItIs(t *testing.T) {
	t.Parallel()
	videos, cache := t.TempDir(), stillcache.Cache{Dir: filepath.Join(t.TempDir(), "thumbnails")}
	file := film(t, videos, "Film.mkv", "frames ")
	kept := stills(5, 300)
	if err := cache.Save(file, kept); err != nil {
		t.Fatal(err)
	}
	moved := film(t, videos, filepath.Join("moved", "Renamed.mkv"), "frames ")
	got, err := cache.Load(moved)
	if err != nil || !reflect.DeepEqual(got, kept) {
		t.Fatalf("the moved film's stills: %d (%v), want the 5 kept", len(got), err)
	}
	if none, err := cache.Load(film(t, videos, "Other.mkv", "other ")); err != nil || len(none) != 0 {
		t.Fatalf("another film: %d stills (%v)", len(none), err)
	}
	if err := cache.Save(file, kept[:2]); err != nil {
		t.Fatal(err)
	}
	if got, _ := cache.Load(file); len(got) != 2 {
		t.Fatalf("kept again with 2 stills, it has %d", len(got))
	}
	entries, _ := os.ReadDir(cache.Dir)
	if len(entries) != 1 {
		t.Fatalf("the stills' folder holds %d files, want the one package", len(entries))
	}
}

// One film's package holds up to its bound (the finest stills, taken last,
// left out); all of them up to theirs, the film opened longest ago going
// first, never the one just kept.
func TestTheKeptStillsStayWithinTheirBounds(t *testing.T) {
	t.Parallel()
	videos, dir := t.TempDir(), t.TempDir()
	// Each package of 4 stills of 1000 bytes takes 16 + 4*(24+1000) bytes.
	const pack = 16 + 4*(24+1000)
	cache := stillcache.Cache{Dir: dir, MaxFilm: pack, MaxTotal: 2 * pack}
	a, b, c := film(t, videos, "a.mkv", "a"), film(t, videos, "b.mkv", "b"), film(t, videos, "c.mkv", "c")
	if err := cache.Save(a, stills(6, 1000)); err != nil {
		t.Fatal(err)
	}
	if got, _ := cache.Load(a); len(got) != 4 || got[3].Asked != 30*time.Second {
		t.Fatalf("a film past its bound kept %d stills, want its first 4", len(got))
	}
	if err := cache.Save(b, stills(4, 1000)); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	for i, file := range []string{a, b} {
		at := old.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(packageOf(t, dir, file), at, at); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := cache.Load(a); len(got) != 4 { // a opened again: b is now the oldest
		t.Fatalf("a: %d stills", len(got))
	}
	if err := cache.Save(c, stills(4, 1000)); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]int{a: 4, b: 0, c: 4} {
		if got, err := cache.Load(file); err != nil || len(got) != want {
			t.Errorf("%s: %d stills (%v), want %d", filepath.Base(file), len(got), err, want)
		}
	}
	if size, err := cache.Size(); err != nil || size != 2*pack {
		t.Fatalf("the kept stills take %d bytes (%v), want %d", size, err, 2*pack)
	}
	tiny := stillcache.Cache{Dir: t.TempDir(), MaxTotal: 10}
	if err := tiny.Save(a, stills(2, 1000)); err != nil {
		t.Fatal(err)
	}
	if got, _ := tiny.Load(a); len(got) != 2 {
		t.Fatalf("a film larger than all the room left out what was just kept: %d stills", len(got))
	}
}

// A package that is not ours or is damaged gives no stills and says so,
// read only within its bounds: another format, too many stills, a still too
// large, of no size or not a JPEG, a package cut short or larger than a
// film's.
func TestAHostilePackageGivesNoStills(t *testing.T) {
	t.Parallel()
	videos, dir := t.TempDir(), t.TempDir()
	cache := stillcache.Cache{Dir: dir, MaxFilm: 1 << 20}
	file := film(t, videos, "film.mkv", "x")
	head := func(count uint32) []byte {
		out := []byte("SPSTILLS")
		out = binary.LittleEndian.AppendUint32(out, 1)
		return binary.LittleEndian.AppendUint32(out, count)
	}
	still := func(width, height uint16, size uint32, jpeg []byte) []byte {
		out := binary.LittleEndian.AppendUint64(nil, uint64(10*time.Second))
		if width == 1 { // a moment past two days
			out = binary.LittleEndian.AppendUint64(nil, uint64(100*time.Hour))
			width = 384
		}
		out = binary.LittleEndian.AppendUint64(out, uint64(10*time.Second))
		out = binary.LittleEndian.AppendUint16(out, width)
		out = binary.LittleEndian.AppendUint16(out, height)
		out = binary.LittleEndian.AppendUint32(out, size)
		return append(out, jpeg...)
	}
	jpeg := []byte{0xFF, 0xD8, 1, 2}
	for name, data := range map[string][]byte{
		"another format": append([]byte("SPSTILLX"), head(1)[8:]...),
		"too many":       head(0xFFFFFFFF),
		"too large":      append(head(1), still(384, 216, 600<<10, append(jpeg, make([]byte, 600<<10-len(jpeg))...))...),
		"too late":       append(head(1), still(1, 216, 4, jpeg)...),
		"no size":        append(head(1), still(0, 216, 4, jpeg)...),
		"not a JPEG":     append(head(1), still(384, 216, 4, []byte("<svg"))...),
		"cut short":      append(head(2), still(384, 216, 4, jpeg)...),
		"larger":         append(append(head(1), still(384, 216, 4, jpeg)...), make([]byte, 1<<20)...),
	} {
		if err := os.WriteFile(packageOf(t, dir, file), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := cache.Load(file); !errors.Is(err, stillcache.ErrNotStills) || len(got) != 0 {
			t.Errorf("%s: %d stills (%v), want none and ErrNotStills", name, len(got), err)
		}
	}
}

// Clearing takes every kept still off the disk, a package left half
// written too, and tells how much it freed; with nothing kept, nothing.
func TestClearingFreesTheDisk(t *testing.T) {
	t.Parallel()
	videos, dir := t.TempDir(), t.TempDir()
	cache := stillcache.Cache{Dir: dir}
	for _, name := range []string{"a.mkv", "b.mkv"} {
		if err := cache.Save(film(t, videos, name, name), stills(3, 500)); err != nil {
			t.Fatal(err)
		}
	}
	left := filepath.Join(dir, "0123.stills.partial-abcd")
	if err := os.WriteFile(left, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	kept, _ := cache.Size()
	if freed, err := cache.Clear(); err != nil || freed != kept+100 {
		t.Fatalf("freed %d bytes (%v), want %d", freed, err, kept+100)
	}
	if size, _ := cache.Size(); size != 0 {
		t.Fatalf("%d bytes left after clearing", size)
	}
	empty := stillcache.Cache{Dir: filepath.Join(dir, "never made")}
	if freed, err := empty.Clear(); err != nil || freed != 0 {
		t.Fatalf("clearing nothing: %d (%v)", freed, err)
	}
	if size, err := empty.Size(); err != nil || size != 0 {
		t.Fatalf("nothing kept: %d (%v)", size, err)
	}
}

// The kept stills always leave room on the disk: to keep a film, the films
// opened longest ago go first; with no room even so, nothing is written,
// what was kept for the film stays, and the app is told the disk is full.
func TestKeptStillsLeaveRoomOnTheDisk(t *testing.T) {
	t.Parallel()
	videos, dir := t.TempDir(), t.TempDir()
	const pack = 16 + 4*(24+1000)
	room := int64(1 << 30)
	cache := stillcache.Cache{Dir: dir, MinFree: 10_000, Free: func(string) (int64, error) { return room, nil }}
	a, b, c := film(t, videos, "a.mkv", "a"), film(t, videos, "b.mkv", "b"), film(t, videos, "c.mkv", "c")
	for i, file := range []string{a, b} {
		if err := cache.Save(file, stills(4, 1000)); err != nil {
			t.Fatal(err)
		}
		at := time.Now().Add(time.Duration(i-2) * time.Hour)
		if err := os.Chtimes(packageOf(t, dir, file), at, at); err != nil {
			t.Fatal(err)
		}
	}
	room = 10_000 + pack - 1000 // c fits once a, the oldest, is out
	if err := cache.Save(c, stills(4, 1000)); err != nil {
		t.Fatalf("keeping a film with room made: %v", err)
	}
	for file, want := range map[string]int{a: 0, b: 4, c: 4} {
		if got, _ := cache.Load(file); len(got) != want {
			t.Errorf("%s: %d stills, want %d", filepath.Base(file), len(got), want)
		}
	}
	room = 0
	if err := cache.Save(c, stills(2, 1000)); !errors.Is(err, thumbnail.ErrNoRoom) {
		t.Fatalf("keeping on a full disk: %v, want no room", err)
	}
	if got, _ := cache.Load(c); len(got) != 4 {
		t.Fatalf("on a full disk the film's kept stills became %d", len(got))
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("the folder holds %d files after a full disk, want only c's package", len(entries))
	}
}

// The disk is told crowded below the room the stills leave free, also
// before their folder is there (its disk is the folder above), and never
// when its room cannot be told.
func TestACrowdedDiskIsTold(t *testing.T) {
	t.Parallel()
	room, err := int64(1<<30), error(nil)
	cache := stillcache.Cache{Dir: t.TempDir(), MinFree: 10_000, Free: func(string) (int64, error) { return room, err }}
	if cache.Crowded() {
		t.Fatal("a disk with room is crowded")
	}
	room = 9_999
	if !cache.Crowded() {
		t.Fatal("a disk below the room left free is not crowded")
	}
	err = errors.New("no answer")
	if cache.Crowded() {
		t.Fatal("a disk whose room is unknown is crowded")
	}
	// The real disk, through a folder not made yet: this computer's has room.
	fresh := stillcache.Cache{Dir: filepath.Join(t.TempDir(), "thumbnails"), MinFree: 1}
	if fresh.Crowded() {
		t.Fatal("a disk with room, under a folder not made yet, is crowded")
	}
	if full := (stillcache.Cache{Dir: filepath.Join(t.TempDir(), "thumbnails"), MinFree: 1 << 62}); !full.Crowded() {
		t.Fatal("the real disk's room was not read through the folder above")
	}
}
