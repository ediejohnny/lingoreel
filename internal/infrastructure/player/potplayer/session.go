//go:build windows

package potplayer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// ErrNotSupported reports a command PotPlayer cannot perform.
var ErrNotSupported = errors.New("not supported by PotPlayer")

// ErrNotAnswering reports a PotPlayer that did not answer a message.
var ErrNotAnswering = errors.New("PotPlayer is not answering")

const (
	pollInterval = 100 * time.Millisecond
	// While PotPlayer runs but stops answering, the poller tries again
	// after firstRetry, doubling up to maxRetry; after reconnectLimit
	// without an answer the session is lost.
	firstRetry     = 250 * time.Millisecond
	maxRetry       = 2 * time.Second
	reconnectLimit = 10 * time.Second
	// closeWait is how long WM_CLOSE has before the job object ends it.
	closeWait = 3 * time.Second
	exitWait  = 2 * time.Second
	// speedStep is the only speed change PotPlayer takes: one key press.
	speedStep = 0.1
	// maxSpeedSteps bounds the key presses one speed change may need; the
	// whole supported range is well inside it.
	maxSpeedSteps = 32
	// areaInterval is how often the picture's window is looked for again,
	// for a playlist the user opens or closes while watching.
	areaInterval = time.Second
)

// Session is a running PotPlayer playing one video.
type Session struct {
	cmd     *exec.Cmd
	job     *job.Job
	tempDir string
	logger  *slog.Logger

	samples chan playback.Sample
	exited  chan struct{}
	// waitErr is the process exit error, set before exited is closed.
	waitErr error

	// window and area are PotPlayer's main window and the child holding
	// the picture; the overlay reads both while the session runs.
	window atomic.Uintptr
	area   atomic.Uintptr
	// areaCheckedAt is when the picture's window was last looked for.
	areaCheckedAt atomic.Int64

	// speed is the rate last asked for, rounded to what PotPlayer can do:
	// it has no message telling its speed back.
	speed atomic.Uint64

	// lost is set when PotPlayer stops answering while its process runs;
	// reconnecting while it is being tried again.
	lost         atomic.Bool
	reconnecting atomic.Bool
	// closing is set by Close, so a PotPlayer ended on purpose is not a crash.
	closing   atomic.Bool
	polling   atomic.Bool
	stop      context.CancelFunc
	pollCtx   context.Context
	pollDone  chan struct{}
	closeOnce sync.Once
}

func newSession(cmd *exec.Cmd, killer *job.Job, tempDir string, logger *slog.Logger) *Session {
	ctx, stop := context.WithCancel(context.Background())
	s := &Session{
		cmd: cmd, job: killer, tempDir: tempDir, logger: logger,
		samples: make(chan playback.Sample, 1), exited: make(chan struct{}),
		stop: stop, pollCtx: ctx, pollDone: make(chan struct{}),
	}
	s.speed.Store(math.Float64bits(float64(playback.NormalSpeed)))
	go func() {
		s.waitErr = cmd.Wait()
		close(s.exited)
	}()
	return s
}

// Capabilities reports what PotPlayer offers: exact seeks in milliseconds,
// one frame forward, speed in steps of 0.1, and no way to pick the audio
// track, which stays with PotPlayer's own menu.
func (s *Session) Capabilities() playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, ExactSeek: true, Speed: true,
		FrameStepForward: true, FrameStepBackward: false,
		AudioTrackSelection: false, SubtitleSuppression: true, Volume: true, MaxVolume: playback.FullVolume,
		Precision: playback.PrecisionEstimated,
	}
}

// ProcessID identifies the PotPlayer process.
func (s *Session) ProcessID() int {
	return s.cmd.Process.Pid
}

// Window is PotPlayer's main window, which the subtitles follow.
func (s *Session) Window() uintptr {
	return s.window.Load()
}

// VideoArea is the child window holding the picture, so PotPlayer's
// playlist and its own bar stay uncovered; zero while there is none.
func (s *Session) VideoArea() uintptr {
	s.refreshArea()
	return s.area.Load()
}

// Samples delivers the latest observation roughly every 100 ms. It is
// closed when PotPlayer exits or stops answering; Err tells the two apart.
func (s *Session) Samples() <-chan playback.Sample {
	return s.samples
}

// Err reports why the samples ended: playback.ErrPlayerLost when PotPlayer
// stopped answering or crashed, nil when it was closed normally.
func (s *Session) Err() error {
	if s.lost.Load() {
		return playback.ErrPlayerLost
	}
	select {
	case <-s.exited:
		if s.waitErr != nil && !s.closing.Load() {
			return fmt.Errorf("%w: %v", playback.ErrPlayerLost, s.waitErr)
		}
	default:
	}
	return nil
}

// Reconnecting reports PotPlayer running but not answering, while the
// poller tries again.
func (s *Session) Reconnecting() bool {
	return s.reconnecting.Load()
}

// Play resumes playback.
func (s *Session) Play() error { return s.command(codeSetStatus, statusPlaying) }

// Pause pauses playback.
func (s *Session) Pause() error { return s.command(codeSetStatus, statusPaused) }

// SeekTo moves to an absolute position; PotPlayer seeks exactly.
func (s *Session) SeekTo(position time.Duration) error {
	if position < 0 {
		position = 0
	}
	return s.command(codeSeek, uintptr(position.Milliseconds()))
}

// SetSpeed changes the playback rate. PotPlayer only steps by 0.1 from
// 1.00x, so the rate asked for goes to the nearest step and that is the
// rate the samples report.
func (s *Session) SetSpeed(speed playback.Speed) error {
	if !speed.Valid() {
		return fmt.Errorf("invalid speed %v", speed)
	}
	steps := int(math.Round((float64(speed) - 1) / speedStep))
	if steps > maxSpeedSteps || steps < -maxSpeedSteps {
		return fmt.Errorf("speed %v is too far from the normal rate", speed)
	}
	// Z first: the steps are counted from the normal rate, whatever the
	// rate PotPlayer is at.
	if err := s.press(keyNormal); err != nil {
		return err
	}
	key := uintptr(keyFaster)
	if steps < 0 {
		key, steps = keySlower, -steps
	}
	for range steps {
		if err := s.press(key); err != nil {
			return err
		}
	}
	s.speed.Store(math.Float64bits(1 + float64(steps)*stepSign(key)*speedStep))
	return nil
}

// stepSign turns the key used into the direction it moves the rate.
func stepSign(key uintptr) float64 {
	if key == keySlower {
		return -1
	}
	return 1
}

// StepFrame advances one frame; PotPlayer has no backward step (the session
// emulates it with an exact seek).
func (s *Session) StepFrame(forward bool) error {
	if !forward {
		return ErrNotSupported
	}
	return s.press(keyNextFrame)
}

// SelectAudioTrack is not offered: PotPlayer has no message for it, so the
// audio track stays the one it picked, changeable in its own menu.
func (s *Session) SelectAudioTrack(int) error {
	return fmt.Errorf("%w: choosing the audio track", ErrNotSupported)
}

// SetVolume changes the volume up to PotPlayer's 100%, the most its
// messages take.
func (s *Session) SetVolume(volume playback.Volume) error {
	if !volume.Valid() || volume > playback.FullVolume {
		return fmt.Errorf("invalid volume %v: PotPlayer goes up to 100%%", volume)
	}
	return s.command(codeSetVolume, uintptr(math.Round(float64(volume)*volumeScale)))
}

// Volume is PotPlayer's level, which it keeps between its own sessions.
func (s *Session) Volume() (playback.Volume, error) {
	level, ok := ask(s.mainWindow(), codeGetVolume, 0)
	if !ok {
		return 0, ErrNotAnswering
	}
	return playback.Volume(float64(level) / volumeScale).Within(playback.FullVolume), nil
}

// command sends one message and fails when PotPlayer does not answer.
func (s *Session) command(code, value uintptr) error {
	if _, ok := ask(windows.HWND(s.window.Load()), code, value); !ok {
		return ErrNotAnswering
	}
	return nil
}

// press sends a key PotPlayer listens to.
func (s *Session) press(key uintptr) error {
	return s.command(codeSendKey, key)
}

func (s *Session) mainWindow() windows.HWND {
	return windows.HWND(s.window.Load())
}

func (s *Session) setWindow(hwnd windows.HWND) {
	s.window.Store(uintptr(hwnd))
}

// refreshArea looks for the picture's window again, at most once every
// areaInterval: the overlay asks for it on every move of the player.
func (s *Session) refreshArea() {
	main := s.mainWindow()
	if !windowAlive(main) {
		s.area.Store(0)
		return
	}
	now := time.Now().UnixNano()
	last := s.areaCheckedAt.Load()
	if now-last < int64(areaInterval) {
		return
	}
	if !s.areaCheckedAt.CompareAndSwap(last, now) {
		return // another caller is looking right now
	}
	area, found := findVideoArea(main)
	if !found {
		area = 0
	}
	s.area.Store(area)
}

func (s *Session) startPolling() {
	s.polling.Store(true)
	go s.poll()
}

// poll reads PotPlayer's state on a fixed interval and publishes it as a
// sample, replacing one the consumer has not read yet. When PotPlayer stops
// answering it tries again with growing waits before giving the session up;
// a PotPlayer that exited ends the polling at once.
func (s *Session) poll() {
	defer close(s.pollDone)
	defer close(s.samples)
	defer s.reconnecting.Store(false)
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()
	lastSuccess := time.Now()
	retry := time.Duration(0) // zero while PotPlayer answers

	for {
		select {
		case <-s.pollCtx.Done():
			return
		case <-s.exited:
			return
		case <-timer.C:
		}
		started := time.Now()
		current, err := s.read()
		finished := time.Now()
		if err != nil {
			if s.pollCtx.Err() != nil {
				return
			}
			if finished.Sub(lastSuccess) > reconnectLimit {
				s.logger.Warn("PotPlayer stopped answering", "error", err)
				s.lost.Store(true)
				return
			}
			if retry == 0 {
				retry = firstRetry
				s.reconnecting.Store(true)
				s.logger.Warn("PotPlayer is not answering; reconnecting", "error", err)
			} else {
				retry = min(2*retry, maxRetry)
			}
			timer.Reset(retry)
			continue
		}
		if retry != 0 {
			s.logger.Info("PotPlayer answered again", "after", finished.Sub(lastSuccess).Round(time.Millisecond))
			retry = 0
			s.reconnecting.Store(false)
		}
		lastSuccess = finished
		current.ObservedAt = started.Add(finished.Sub(started) / 2)
		s.publish(current)
		timer.Reset(pollInterval)
	}
}

// read asks PotPlayer where it is. The position decides: without it there
// is no sample worth publishing.
func (s *Session) read() (playback.Sample, error) {
	window := s.mainWindow()
	position, ok := ask(window, codeGetCurrentTime, 0)
	if !ok {
		return playback.Sample{}, ErrNotAnswering
	}
	total, ok := ask(window, codeGetTotalTime, 0)
	if !ok {
		return playback.Sample{}, ErrNotAnswering
	}
	state, ok := ask(window, codeGetStatus, 0)
	if !ok {
		return playback.Sample{}, ErrNotAnswering
	}
	sample := playback.Sample{
		Position: milliseconds(position),
		Duration: milliseconds(total),
		Speed:    playback.Speed(math.Float64frombits(s.speed.Load())),
		Status:   playback.StatusStopped,
	}
	switch state {
	case statusPlaying:
		sample.Status = playback.StatusPlaying
	case statusPaused:
		sample.Status = playback.StatusPaused
	}
	return sample, nil
}

func (s *Session) publish(sample playback.Sample) {
	select {
	case <-s.samples:
	default:
	}
	select {
	case s.samples <- sample:
	default:
	}
}

// Close asks PotPlayer to close its window, ends it through the job object
// if it lingers, and removes the temporary folder. Every wait is bounded.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closing.Store(true)
		// The poller stops first: a PotPlayer on its way out stops
		// answering, which is not a player lost.
		s.stop()
		// PotPlayer closes on WM_CLOSE, unlike VLC; the job object below
		// ends it either way. The message is posted, never sent, so a hung
		// PotPlayer cannot hold this call.
		if window := s.mainWindow(); windowAlive(window) {
			_ = win32.PostMessage(window, win32.WM_CLOSE, 0, 0)
		}
		waitClosed(s.exited, closeWait)
		if jobErr := s.job.Close(); jobErr != nil {
			err = errors.Join(err, fmt.Errorf("ending PotPlayer: %w", jobErr))
		}
		if !waitClosed(s.exited, exitWait) {
			err = errors.Join(err, errors.New("PotPlayer did not exit"))
		}
		if s.polling.Load() && !waitClosed(s.pollDone, answerTimeout+pollInterval) {
			err = errors.Join(err, errors.New("the PotPlayer status reader did not stop"))
		}
		s.window.Store(0)
		s.area.Store(0)
		if removeErr := os.RemoveAll(s.tempDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("removing PotPlayer's temporary folder: %w", removeErr))
		}
	})
	return err
}

// waitClosed waits for a channel to close, at most for limit.
func waitClosed(done <-chan struct{}, limit time.Duration) bool {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
