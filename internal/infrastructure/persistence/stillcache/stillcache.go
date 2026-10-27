// Package stillcache keeps films' seek-bar stills in the app's own folder,
// one package per film named by its content (filmkey), so a film's stills
// show at once when it opens again, wherever it was moved. What it reads
// back is data only, checked against bounds before anything is kept.
package stillcache

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/domain/fault"
	"github.com/ediejohnny/lingoreel/internal/domain/storyboard"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/diskfault"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/filmkey"
)

// The defaults: a film's stills take up to some 35 MB (1000 of 20 to 35
// KB), so one always fits; all films together take up to 4 GB, and never
// leave the disk with less than 1 GB free.
const (
	DefaultMaxFilm  = 64 << 20
	DefaultMaxTotal = 4 << 30
	DefaultMinFree  = 1 << 30
)

// ErrNotStills reports a package that is not one of ours, or is damaged.
var ErrNotStills = errors.New("not a package of stills")

const (
	magic     = "SPSTILLS"
	version   = 1
	extension = ".stills"
	// partial marks a package being written; it becomes the package by a
	// rename, so a package is always whole.
	partial = ".partial-"
	// Bounds on what a package may hold.
	maxJPEG    = 512 << 10
	maxSide    = 4096
	maxMoment  = 48 * time.Hour
	headerSize = len(magic) + 4 + 4
	stillHead  = 8 + 8 + 2 + 2 + 4
)

var _ thumbnail.Store = Cache{}

// Cache keeps the stills in Dir. MaxFilm bounds one film's package (the
// stills past it, the finest, are left out); MaxTotal bounds them all, the
// films opened longest ago going first; MinFree is the room always left
// free on the disk, other films going first to keep it. Zero takes the
// defaults. Free tells the disk's room (nil: the system's).
type Cache struct {
	Dir                        string
	MaxFilm, MaxTotal, MinFree int64
	Free                       func(dir string) (int64, error)
}

func (c Cache) minFree() int64 {
	if c.MinFree > 0 {
		return c.MinFree
	}
	return DefaultMinFree
}

func (c Cache) free() (int64, error) {
	if c.Free != nil {
		return c.Free(c.Dir)
	}
	return freeRoom(c.Dir)
}

// Crowded tells the disk of the stills has less room left than they always
// leave free: the oldest kept films leave first, and new stills stay in
// memory until there is room. Before the first still the folder may not be
// there yet: its disk is the folder above.
func (c Cache) Crowded() bool {
	room, err := c.free()
	if err != nil && c.Free == nil {
		room, err = freeRoom(filepath.Dir(c.Dir))
	}
	return err == nil && room < c.minFree()
}

func (c Cache) maxFilm() int64 {
	if c.MaxFilm > 0 {
		return c.MaxFilm
	}
	return DefaultMaxFilm
}

func (c Cache) maxTotal() int64 {
	if c.MaxTotal > 0 {
		return c.MaxTotal
	}
	return DefaultMaxTotal
}

func (c Cache) path(file string) (string, error) {
	key, err := filmkey.Of(file)
	if err != nil {
		return "", err
	}
	return filepath.Join(c.Dir, key+extension), nil
}

// Load gives the stills kept for a film, and marks it as just opened; none
// when there are none.
func (c Cache) Load(file string) ([]thumbnail.Kept, error) {
	path, err := c.path(file)
	if err != nil {
		return nil, err
	}
	stored, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening the kept stills: %w", err)
	}
	defer stored.Close()
	stat, err := stored.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > c.maxFilm() {
		return nil, fmt.Errorf("%w: %s", ErrNotStills, filepath.Base(path))
	}
	stills, err := read(bufio.NewReader(io.LimitReader(stored, c.maxFilm())))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrNotStills, filepath.Base(path), err)
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return stills, nil
}

func read(r io.Reader) ([]thumbnail.Kept, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if string(header[:len(magic)]) != magic || binary.LittleEndian.Uint32(header[len(magic):]) != version {
		return nil, errors.New("another format")
	}
	count := binary.LittleEndian.Uint32(header[len(magic)+4:])
	if count > storyboard.MaxStills {
		return nil, fmt.Errorf("%d stills", count)
	}
	stills := make([]thumbnail.Kept, 0, count)
	head := make([]byte, stillHead)
	for range count {
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, err
		}
		asked := time.Duration(binary.LittleEndian.Uint64(head))
		at := time.Duration(binary.LittleEndian.Uint64(head[8:]))
		width, height := int(binary.LittleEndian.Uint16(head[16:])), int(binary.LittleEndian.Uint16(head[18:]))
		size := binary.LittleEndian.Uint32(head[20:])
		switch {
		case asked < 0 || asked > maxMoment || at < 0 || at > maxMoment:
			return nil, fmt.Errorf("a still at %v asked at %v", at, asked)
		case width < 1 || width > maxSide || height < 1 || height > maxSide:
			return nil, fmt.Errorf("a still of %dx%d", width, height)
		case size < 2 || size > maxJPEG:
			return nil, fmt.Errorf("a still of %d bytes", size)
		}
		jpeg := make([]byte, size)
		if _, err := io.ReadFull(r, jpeg); err != nil {
			return nil, err
		}
		if jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
			return nil, errors.New("a still that is not a JPEG")
		}
		stills = append(stills, thumbnail.Kept{Asked: asked, Picture: thumbnail.Picture{At: at, Width: width, Height: height, JPEG: jpeg}})
	}
	return stills, nil
}

// Save keeps a film's stills in place of what was kept, whole, up to
// MaxFilm; then the films opened longest ago go until all fit in MaxTotal.
func (c Cache) Save(file string, stills []thumbnail.Kept) error {
	path, err := c.path(file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("making the stills' folder: %w", err)
	}
	var data bytes.Buffer
	data.WriteString(magic)
	_ = binary.Write(&data, binary.LittleEndian, uint32(version))
	data.Write(make([]byte, 4)) // the count, once known
	count := uint32(0)
	for _, kept := range stills {
		picture := kept.Picture
		size := len(picture.JPEG)
		if int64(data.Len()+stillHead+size) > c.maxFilm() || count == storyboard.MaxStills {
			break // taken coarse to fine: the finest are left out
		}
		if size < 2 || size > maxJPEG || picture.Width < 1 || picture.Width > maxSide || picture.Height < 1 || picture.Height > maxSide {
			continue
		}
		_ = binary.Write(&data, binary.LittleEndian, uint64(kept.Asked))
		_ = binary.Write(&data, binary.LittleEndian, uint64(picture.At))
		_ = binary.Write(&data, binary.LittleEndian, uint16(picture.Width))
		_ = binary.Write(&data, binary.LittleEndian, uint16(picture.Height))
		_ = binary.Write(&data, binary.LittleEndian, uint32(size))
		data.Write(picture.JPEG)
		count++
	}
	binary.LittleEndian.PutUint32(data.Bytes()[len(magic)+4:], count)
	if err := c.makeRoom(path, int64(data.Len())); err != nil {
		return err
	}
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	temporary := path + partial + hex.EncodeToString(suffix)
	if err := os.WriteFile(temporary, data.Bytes(), 0o644); err != nil {
		_ = os.Remove(temporary)
		if diskfault.Of(err) == fault.DiskFull {
			return fmt.Errorf("%w: %w", thumbnail.ErrNoRoom, err)
		}
		return fmt.Errorf("keeping the stills: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("keeping the stills: %w", err)
	}
	return c.trim(path)
}

// pack is a kept film's package.
type pack struct {
	path     string
	size     int64
	modified time.Time
}

func (c Cache) packages() ([]pack, error) {
	entries, err := os.ReadDir(c.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the stills' folder: %w", err)
	}
	var found []pack
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), extension) {
			continue
		}
		if info, err := entry.Info(); err == nil {
			found = append(found, pack{filepath.Join(c.Dir, entry.Name()), info.Size(), info.ModTime()})
		}
	}
	return found, nil
}

// makeRoom makes sure writing size bytes leaves MinFree on the disk, taking
// out the films opened longest ago (never this one) as needed; a disk whose
// room cannot be told is left to its limits.
func (c Cache) makeRoom(keep string, size int64) error {
	room, err := c.free()
	if err != nil {
		return nil
	}
	need := size + c.minFree() - room
	if need <= 0 {
		return nil
	}
	found, err := c.packages()
	if err != nil {
		return err
	}
	slices.SortFunc(found, func(a, b pack) int { return a.modified.Compare(b.modified) })
	for _, p := range found {
		if need <= 0 {
			break
		}
		if p.path != keep && os.Remove(p.path) == nil {
			need -= p.size
		}
	}
	if need > 0 {
		return fmt.Errorf("%w: %d MB free, %d MB always left free", thumbnail.ErrNoRoom, room>>20, c.minFree()>>20)
	}
	return nil
}

// trim takes the films opened longest ago out until all fit; the film just
// kept stays.
func (c Cache) trim(kept string) error {
	found, err := c.packages()
	if err != nil {
		return err
	}
	total := int64(0)
	for _, p := range found {
		total += p.size
	}
	slices.SortFunc(found, func(a, b pack) int { return a.modified.Compare(b.modified) })
	for _, p := range found {
		if total <= c.maxTotal() {
			break
		}
		if p.path != kept && os.Remove(p.path) == nil {
			total -= p.size
		}
	}
	return nil
}

// Size is how much the kept stills take on the disk, packages being written
// included.
func (c Cache) Size() (int64, error) {
	found, err := c.packages()
	total := int64(0)
	for _, p := range found {
		total += p.size
	}
	return total + c.partials(time.Now(), false), err
}

// staleWrite is how old a package left half written must be to be taken for
// one an app that stopped while writing it left: a write never takes that
// long.
const staleWrite = time.Hour

// Sweep takes out the packages left half written by an app that stopped
// while writing them, as the app opens.
func (c Cache) Sweep() {
	c.partials(time.Now(), true)
}

// partials is how much the packages being written take; sweep takes out
// those staleWrite old or more at now, which no write is on.
func (c Cache) partials(now time.Time, sweep bool) int64 {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return 0
	}
	total := int64(0)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !entry.Type().IsRegular() || !strings.Contains(entry.Name(), extension+partial) {
			continue
		}
		if sweep && now.Sub(info.ModTime()) >= staleWrite && os.Remove(filepath.Join(c.Dir, entry.Name())) == nil {
			continue
		}
		total += info.Size()
	}
	return total
}

// Clear takes every kept still off the disk (and any package left half
// written), and tells how much it freed.
func (c Cache) Clear() (int64, error) {
	entries, err := os.ReadDir(c.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading the stills' folder: %w", err)
	}
	freed := int64(0)
	var problems []error
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !(strings.HasSuffix(name, extension) || strings.Contains(name, extension+partial)) {
			continue
		}
		info, _ := entry.Info()
		if err := os.Remove(filepath.Join(c.Dir, name)); err != nil {
			problems = append(problems, err)
		} else if info != nil {
			freed += info.Size()
		}
	}
	return freed, errors.Join(problems...)
}
