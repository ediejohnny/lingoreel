//go:build windows

// Package potplayer drives PotPlayer through the messages it answers on its
// main window: times in milliseconds, play state and volume by code, and
// the rest (one frame forward, the speed) by the keys it listens to.
// PotPlayer draws in its own window, so the subtitles are put over the
// child window that holds the picture, leaving its playlist and its own bar
// uncovered.
package potplayer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"

	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/windows/job"
	"lingoreel/internal/infrastructure/windows/win32"
)

// ErrStartTimeout reports a PotPlayer that did not show its window, or did
// not open the video, in time.
var ErrStartTimeout = errors.New("PotPlayer did not open the video in time")

const (
	// startTimeout bounds the whole opening: the window, then the video.
	// PotPlayer showed its window in about 3 s and the picture 3 to 8 s
	// later on the machine it was measured on.
	startTimeout = 30 * time.Second
	startRetry   = 50 * time.Millisecond
	// startTolerance absorbs a position PotPlayer refreshes twice a second.
	startTolerance = 700 * time.Millisecond
	// seekSettle is how long a seek has to show in the reported position
	// before the start is asked for again.
	seekSettle = 400 * time.Millisecond
)

// blankSubtitle is a valid subtitle file with a single blank line at an hour
// no video reaches. Given with /sub= it is the subtitle PotPlayer shows, so
// the one inside the video stays hidden and the app draws the only one the
// user sees (measured with a screen capture).
const blankSubtitle = "1\r\n99:00:00,000 --> 99:00:01,000\r\n \r\n"

// Options tunes a launch; the zero value is what the app uses.
type Options struct {
	// Logger receives what has no caller to report to, such as a PotPlayer
	// that stopped answering; nil discards it.
	Logger *slog.Logger
}

// Arguments returns the command line given to PotPlayer: the video, a window
// of its own (/new leaves a PotPlayer the user already opened alone) and the
// blank subtitle that hides its own.
func Arguments(request player.LaunchRequest, blankPath string) []string {
	args := []string{request.File, "/new"}
	if blankPath != "" {
		args = append(args, "/sub="+blankPath)
	}
	return args
}

// Launch starts PotPlayer on a video and waits until it is playing it.
func Launch(ctx context.Context, executable string, request player.LaunchRequest) (*Session, error) {
	return LaunchWith(ctx, executable, request, Options{})
}

// LaunchWith is Launch with options.
func LaunchWith(ctx context.Context, executable string, request player.LaunchRequest, options Options) (*Session, error) {
	if !filepath.IsAbs(executable) || !isRegularFile(executable) {
		return nil, fmt.Errorf("PotPlayer executable %q not found", executable)
	}
	file, err := filepath.Abs(request.File)
	if err != nil || !isRegularFile(file) {
		return nil, fmt.Errorf("video %q not found", request.File)
	}
	// An absolute path starts with a drive or a share, never with "/", so
	// PotPlayer cannot take the file name for one of its options.
	request.File = file
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	dir, blank, err := writeBlankSubtitle()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(executable, Arguments(request, blank)...)
	if err := cmd.Start(); err != nil {
		removeTemp(dir)
		return nil, fmt.Errorf("starting PotPlayer: %w", err)
	}
	killer, err := job.NewKillOnClose()
	if err == nil {
		err = killer.Assign(cmd.Process.Pid)
	}
	if err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil {
			err = errors.Join(err, fmt.Errorf("ending PotPlayer: %w", killErr))
		}
		removeTemp(dir)
		return nil, fmt.Errorf("tying PotPlayer to the app: %w", err)
	}

	session := newSession(cmd, killer, dir, logger)
	if err := session.waitUntilReady(ctx, request); err != nil {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, err
	}
	logger.Info("PotPlayer connected", "pid", cmd.Process.Pid)
	session.startPolling()
	return session, nil
}

// waitUntilReady waits for PotPlayer's window, places it, waits for the
// video, and leaves it paused at the requested start. Publishing a sample
// before that would show the subtitles of the wrong moment.
func (s *Session) waitUntilReady(ctx context.Context, request player.LaunchRequest) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()

	if _, err := s.waitFor(ctx, deadline.C, "its window", func() bool {
		hwnd, found := findMainWindow(uint32(s.cmd.Process.Pid))
		s.setWindow(hwnd)
		return found
	}); err != nil {
		return err
	}
	if window := request.Window; window.Set() {
		// PotPlayer refuses a position off the screen (measured): it is
		// asked for the place the app picked inside the work area.
		bounds := win32.Rect{Left: int32(window.X), Top: int32(window.Y),
			Right: int32(window.X + window.Width), Bottom: int32(window.Y + window.Height)}
		if err := win32.SetWindowPos(s.mainWindow(), 0, bounds, win32.SWP_NOZORDER|win32.SWP_NOACTIVATE); err != nil {
			s.logger.Warn("PotPlayer did not take the window position", "error", err)
		}
	}
	if _, err := s.waitFor(ctx, deadline.C, "the video", func() bool {
		total, ok := ask(s.mainWindow(), codeGetTotalTime, 0)
		return ok && total > 0
	}); err != nil {
		return err
	}
	// Paused first, then the seek: a PotPlayer still playing moves on from
	// where the seek landed before the first sample is taken.
	if request.StartPaused {
		if err := s.waitUntilPaused(ctx, deadline.C); err != nil {
			return err
		}
	}
	if err := s.seekToStart(ctx, deadline.C, request.Start); err != nil {
		return err
	}
	s.refreshArea()
	return nil
}

// waitUntilPaused pauses and waits until PotPlayer says it is paused. A
// pause sent while it was still opening the file is lost (measured: the
// first sample then came in playing), so it is asked again.
func (s *Session) waitUntilPaused(ctx context.Context, deadline <-chan time.Time) error {
	if err := s.Pause(); err != nil {
		return fmt.Errorf("pausing PotPlayer: %w", err)
	}
	asked := time.Now()
	_, err := s.waitFor(ctx, deadline, "the pause", func() bool {
		state, ok := ask(s.mainWindow(), codeGetStatus, 0)
		if ok && state == statusPaused {
			return true
		}
		if time.Since(asked) >= seekSettle {
			asked = time.Now()
			_ = s.Pause()
		}
		return false
	})
	return err
}

// seekToStart moves to the requested start and waits for PotPlayer to
// report it. A start at or past the end is ignored, as PotPlayer ignores it.
func (s *Session) seekToStart(ctx context.Context, deadline <-chan time.Time, start time.Duration) error {
	if start <= 0 {
		return nil
	}
	total, ok := ask(s.mainWindow(), codeGetTotalTime, 0)
	if !ok || start >= milliseconds(total) {
		return nil
	}
	if err := s.SeekTo(start); err != nil {
		return fmt.Errorf("opening PotPlayer at %v: %w", start, err)
	}
	asked := time.Now()
	_, err := s.waitFor(ctx, deadline, "the start position", func() bool {
		current, ok := ask(s.mainWindow(), codeGetCurrentTime, 0)
		if ok && milliseconds(current) >= start-startTolerance {
			return true
		}
		// A seek sent while PotPlayer was still opening the file is lost;
		// it is asked again, never so fast as to undo one that is landing.
		if time.Since(asked) >= seekSettle {
			asked = time.Now()
			_ = s.SeekTo(start)
		}
		return false
	})
	return err
}

// waitFor polls until ready, the player exits, the context ends or the
// deadline passes; what names the step goes into the timeout's message.
func (s *Session) waitFor(ctx context.Context, deadline <-chan time.Time, step string, ready func() bool) (bool, error) {
	for {
		if ready() {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-s.exited:
			return false, fmt.Errorf("PotPlayer closed before opening the video")
		case <-deadline:
			return false, fmt.Errorf("%w (waiting for %s)", ErrStartTimeout, step)
		case <-time.After(startRetry):
		}
	}
}

// writeBlankSubtitle puts the blank subtitle in a private temporary folder
// and returns the folder and the file.
func writeBlankSubtitle() (dir, path string, err error) {
	dir, err = os.MkdirTemp("", "lingoreel-potplayer-*")
	if err != nil {
		return "", "", fmt.Errorf("creating PotPlayer's temporary folder: %w", err)
	}
	path = filepath.Join(dir, "blank.srt")
	if err := os.WriteFile(path, []byte(blankSubtitle), 0o600); err != nil {
		removeTemp(dir)
		return "", "", fmt.Errorf("writing the blank subtitle: %w", err)
	}
	return dir, path, nil
}

// removeTemp deletes the folder of a launch that failed; it is in the
// user's temporary directory, so a failure only leaves it for the system.
func removeTemp(dir string) {
	_ = os.RemoveAll(dir)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// windowAlive reports a window that still exists.
func windowAlive(hwnd windows.HWND) bool {
	return hwnd != 0 && windows.IsWindow(hwnd)
}
