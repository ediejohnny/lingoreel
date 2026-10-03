// Package logfile keeps the app's log as one file per day in a folder,
// "lingoreel-2026-10-03.log", appending to the day's file, capping its
// size and removing the files of days long gone.
package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const (
	// KeepDays is how many days of logs stay in the folder.
	KeepDays = 14
	// DayLimit caps one day's file; past it, lines are dropped.
	DayLimit int64 = 10 << 20

	prefix     = "lingoreel-"
	extension  = ".log"
	dateLayout = "2006-01-02"
	// legacyName is the single log of the versions before 0.13.
	legacyName = "lingoreel.log"
)

// namePattern matches only the files this package writes.
var namePattern = regexp.MustCompile(`^lingoreel-(\d{4}-\d{2}-\d{2})\.log$`)

// limitNotice ends a day's file that reached the cap.
const limitNotice = "level=WARN msg=\"the daily log limit was reached; later lines of the day are dropped\"\n"

// Writer is the log's destination, safe for concurrent writes.
type Writer struct {
	mu      sync.Mutex
	dir     string
	now     func() time.Time
	limit   int64
	day     string
	file    *os.File
	written int64
	full    bool
}

// Open starts the log in dir, creating it; now is the clock (time.Now for
// the app). The files of more than KeepDays days ago are removed.
func Open(dir string, now func() time.Time) (*Writer, error) {
	return OpenLimited(dir, now, DayLimit)
}

// OpenLimited is Open with another daily cap, for tests.
func OpenLimited(dir string, now func() time.Time, limit int64) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating the log folder: %w", err)
	}
	w := &Writer{dir: dir, now: now, limit: limit}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotate(); err != nil {
		return nil, err
	}
	// The single log of older versions is no longer written; one that
	// cannot go now goes at a later start.
	_ = os.Remove(filepath.Join(dir, legacyName))
	return w, nil
}

// Path is the file of the current day.
func (w *Writer) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pathOf(w.day)
}

// Write appends to the day's file, moving to a new file when the day
// changes. Past the daily cap the lines are dropped, after a notice.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.now().Format(dateLayout) != w.day {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	if w.full {
		return len(p), nil
	}
	if w.written+int64(len(p)) > w.limit {
		w.full = true
		n, err := w.file.WriteString(limitNotice)
		w.written += int64(n)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}
	n, err := w.file.Write(p)
	w.written += int64(n)
	return n, err
}

// Close closes the day's file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// rotate opens the file of today and removes the old ones. Called with the
// mutex held.
func (w *Writer) rotate() error {
	today := w.now()
	day := today.Format(dateLayout)
	file, err := os.OpenFile(w.pathOf(day), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening the log of %s: %w", day, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("reading the log of %s: %w", day, err)
	}
	if w.file != nil {
		w.file.Close()
	}
	w.file, w.day, w.written = file, day, info.Size()
	w.full = w.written >= w.limit
	w.removeOld(today)
	return nil
}

// removeOld deletes the files of days more than KeepDays before today; a
// file that cannot go stays for the next try.
func (w *Writer) removeOld(today time.Time) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	y, m, d := today.Date()
	oldest := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(KeepDays - 1))
	for _, entry := range entries {
		match := namePattern.FindStringSubmatch(entry.Name())
		if match == nil || !entry.Type().IsRegular() {
			continue
		}
		day, err := time.Parse(dateLayout, match[1])
		if err != nil || !day.Before(oldest) {
			continue
		}
		_ = os.Remove(filepath.Join(w.dir, entry.Name())) // retried at the next start
	}
}

func (w *Writer) pathOf(day string) string {
	return filepath.Join(w.dir, prefix+day+extension)
}
