package logfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/logfile"
)

// clock is a settable time for the writer.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func day(y int, m time.Month, d, hour, minute int) time.Time {
	return time.Date(y, m, d, hour, minute, 0, 0, time.UTC)
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, w *logfile.Writer, line string) {
	t.Helper()
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
}

func TestOneFilePerDayAppending(t *testing.T) {
	dir := t.TempDir()
	c := &clock{now: day(2026, 10, 3, 23, 59)}
	w, err := logfile.Open(dir, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	write(t, w, "first\n")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// A second start the same day appends; it never clears the morning's lines.
	w, err = logfile.Open(dir, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	write(t, w, "second\n")
	october3 := filepath.Join(dir, "lingoreel-2026-10-03.log")
	if w.Path() != october3 {
		t.Fatalf("path = %s", w.Path())
	}
	// Past midnight the next line starts the next day's file.
	c.set(day(2026, 10, 4, 0, 0))
	write(t, w, "third\n")
	if got := read(t, october3); got != "first\nsecond\n" {
		t.Fatalf("the 3rd holds %q", got)
	}
	if got := read(t, filepath.Join(dir, "lingoreel-2026-10-04.log")); got != "third\n" {
		t.Fatalf("the 4th holds %q", got)
	}
}

func TestOldDaysAreRemoved(t *testing.T) {
	dir := t.TempDir()
	keep := []string{
		"lingoreel-2026-09-20.log", // 13 days before: the 14th day kept
		"lingoreel-notes.log",      // not a day's log
		"other.log",
		"lingoreel-2026-9-01.log",
	}
	gone := []string{"lingoreel-2026-09-19.log", "lingoreel-2025-12-31.log", "lingoreel.log"}
	for _, name := range append(append([]string{}, keep...), gone...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A folder with a day's name is not a log.
	folder := "lingoreel-2000-01-01.log"
	if err := os.Mkdir(filepath.Join(dir, folder), 0o700); err != nil {
		t.Fatal(err)
	}
	c := &clock{now: day(2026, 10, 3, 8, 0)}
	w, err := logfile.Open(dir, c.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, name := range append(keep, folder) {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
	for _, name := range gone {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s is still there", name)
		}
	}

	// A new day removes the day that just got too old.
	c.set(day(2026, 10, 4, 0, 1))
	write(t, w, "x\n")
	if _, err := os.Stat(filepath.Join(dir, "lingoreel-2026-09-20.log")); !os.IsNotExist(err) {
		t.Fatal("the 20th stayed after the 4th began")
	}
}

func TestADayIsCapped(t *testing.T) {
	dir := t.TempDir()
	c := &clock{now: day(2026, 10, 3, 12, 0)}
	w, err := logfile.OpenLimited(dir, c.Now, 100)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("a", 59) + "\n"
	write(t, w, line)
	write(t, w, line) // over the cap: dropped, with one notice
	write(t, w, line)
	got := read(t, w.Path())
	if !strings.HasPrefix(got, line) || strings.Count(got, line) != 1 || strings.Count(got, "limit was reached") != 1 {
		t.Fatalf("capped file = %q", got)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// Starting again the same day, the full file stays as it is.
	w, err = logfile.OpenLimited(dir, c.Now, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	write(t, w, line)
	if again := read(t, w.Path()); again != got {
		t.Fatalf("a full day took more: %q", again)
	}
	// The next day starts empty.
	c.set(day(2026, 10, 4, 0, 0))
	write(t, w, line)
	if next := read(t, w.Path()); next != line {
		t.Fatalf("the next day holds %q", next)
	}
}

func TestConcurrentLinesStayWhole(t *testing.T) {
	c := &clock{now: day(2026, 10, 3, 12, 0)}
	w, err := logfile.Open(t.TempDir(), c.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	for g := range 20 {
		wg.Go(func() {
			for i := range 100 {
				fmt.Fprintf(w, "goroutine %02d line %03d\n", g, i)
			}
		})
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSuffix(read(t, w.Path()), "\n"), "\n")
	if len(lines) != 2000 {
		t.Fatalf("%d lines, want 2000", len(lines))
	}
	for _, line := range lines {
		if len(line) != len("goroutine 00 line 000") {
			t.Fatalf("broken line %q", line)
		}
	}
}

func TestAFolderThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "taken")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := logfile.Open(file, time.Now); err == nil {
		t.Fatal("Open on a file succeeded")
	}
}
