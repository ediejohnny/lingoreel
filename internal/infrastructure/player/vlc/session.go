//go:build windows

package vlc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
)

// ErrNotSupported reports a command VLC cannot perform.
var ErrNotSupported = errors.New("not supported by VLC")

const (
	pollInterval = 100 * time.Millisecond
	// While VLC runs but stops answering, the poller tries again after
	// firstRetry, doubling up to maxRetry; after reconnectLimit without an
	// answer the session is lost.
	firstRetry     = 250 * time.Millisecond
	maxRetry       = 2 * time.Second
	reconnectLimit = 10 * time.Second
	quitTimeout    = time.Second
	exitWait       = 2 * time.Second
	// startTolerance absorbs the estimate error of VLC's first position.
	startTolerance = 500 * time.Millisecond
	// startSeekGrace is how long an open video may sit at 0 s before Launch
	// seeks to the start itself. VLC usually applies --start-time a moment
	// after opening; a seek sent meanwhile arrived late and undid the user's
	// first seek.
	startSeekGrace = time.Second
)

// Session is a running VLC playing one video.
type Session struct {
	cmd         *exec.Cmd
	job         *job.Job
	client      *client
	endpointDir string
	logger      *slog.Logger

	samples chan playback.Sample
	exited  chan struct{}
	// waitErr is the process exit error, set before exited is closed.
	waitErr error
	// lost is set when VLC stops answering while its process still runs;
	// reconnecting while it is being tried again.
	lost         atomic.Bool
	reconnecting atomic.Bool
	// closing is set by Close, so a VLC ended on purpose is not a crash.
	closing   atomic.Bool
	polling   atomic.Bool
	stop      context.CancelFunc
	pollCtx   context.Context
	pollDone  chan struct{}
	closeOnce sync.Once
}

func newSession(cmd *exec.Cmd, killer *job.Job, control *client, endpointDir string, logger *slog.Logger) *Session {
	ctx, stop := context.WithCancel(context.Background())
	s := &Session{
		cmd: cmd, job: killer, client: control, endpointDir: endpointDir, logger: logger,
		samples: make(chan playback.Sample, 1), exited: make(chan struct{}),
		stop: stop, pollCtx: ctx, pollDone: make(chan struct{}),
	}
	go func() {
		s.waitErr = cmd.Wait()
		close(s.exited)
	}()
	return s
}

// Capabilities reports what VLC offers: exact seeks but no backward frame
// step, and positions that are estimates updated a few times per second.
func (s *Session) Capabilities() playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, ExactSeek: true, Speed: true,
		FrameStepForward: true, FrameStepBackward: false,
		AudioTrackSelection: true, SubtitleSuppression: true, Volume: true, MaxVolume: playback.MaxVolume,
		Precision: playback.PrecisionEstimated,
	}
}

// vlcFullVolume is 100% on VLC's volume scale.
const vlcFullVolume = 256

// SetVolume changes the volume; VLC takes up to 200% (512), amplifying the
// sound above 100%.
func (s *Session) SetVolume(volume playback.Volume) error {
	if !volume.Valid() {
		return fmt.Errorf("invalid volume %v", volume)
	}
	level := math.Round(float64(volume) * vlcFullVolume)
	return s.send(command("volume", "value", strconv.FormatFloat(level, 'f', 0, 64)))
}

// ReportedVolume is the level VLC says it plays at, for diagnostics; it is
// not a session.VolumeReader on purpose: before its sound output exists VLC
// reports none, so a session starts by asking for 100% instead.
func (s *Session) ReportedVolume() (playback.Volume, bool, error) {
	ctx, cancel := context.WithTimeout(s.pollCtx, requestTimeout)
	defer cancel()
	current, err := s.client.call(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	if current.Volume < 0 {
		return 0, false, nil
	}
	return playback.Volume(current.Volume / vlcFullVolume), true, nil
}

// ReportedAudioTrack is the playing audio track by its position among the
// audio tracks, as SelectAudioTrack counts them; false for none.
func (s *Session) ReportedAudioTrack() (int, bool, error) {
	ctx, cancel := context.WithTimeout(s.pollCtx, requestTimeout)
	defer cancel()
	current, err := s.client.call(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	return current.Audio, current.Audio >= 0, nil
}

// ProcessID identifies the VLC process, whose window the overlay follows.
func (s *Session) ProcessID() int {
	return s.cmd.Process.Pid
}

// Samples delivers the latest observation roughly every 100 ms. It is closed
// when VLC exits or stops answering; Err then tells the two apart.
func (s *Session) Samples() <-chan playback.Sample {
	return s.samples
}

// Err reports why the samples ended: playback.ErrPlayerLost when VLC stopped
// answering or crashed, nil when it was closed normally or is still running.
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

// Play resumes playback.
func (s *Session) Play() error { return s.send(command("play")) }

// Pause pauses playback.
func (s *Session) Pause() error { return s.send(command("pause")) }

// SeekTo moves to an absolute position.
func (s *Session) SeekTo(position time.Duration) error {
	return s.send(command("seek", "us", strconv.FormatInt(position.Microseconds(), 10)))
}

// SeekAndPlay goes to a position and plays from there in one request, so
// VLC starts the picture and the sound together (session.LapPlayer).
func (s *Session) SeekAndPlay(position time.Duration) error {
	return s.send(command("lap", "us", strconv.FormatInt(position.Microseconds(), 10)))
}

// Prepare pauses at a position and shows its picture, so a Play after
// starts at once; lrdev seek-probe measures it against SeekAndPlay.
func (s *Session) Prepare(position time.Duration) error {
	return s.send(command("prepare", "us", strconv.FormatInt(position.Microseconds(), 10)))
}

// SetSpeed changes the playback rate.
func (s *Session) SetSpeed(speed playback.Speed) error {
	return s.send(command("rate", "value", strconv.FormatFloat(float64(speed), 'f', 4, 64)))
}

// StepFrame advances one frame; VLC cannot step backward (the session
// emulates it with an exact seek).
func (s *Session) StepFrame(forward bool) error {
	if !forward {
		return ErrNotSupported
	}
	return s.send(command("frame"))
}

// SetFullscreen puts the video window in or out of fullscreen.
func (s *Session) SetFullscreen(on bool) error {
	value := "0"
	if on {
		value = "1"
	}
	return s.send(command("fullscreen", "value", value))
}

// SelectAudioTrack picks the audio track by its position among audio tracks.
func (s *Session) SelectAudioTrack(index int) error {
	return s.send(command("audio", "index", strconv.Itoa(index)))
}

func (s *Session) send(values map[string][]string) error {
	ctx, cancel := context.WithTimeout(s.pollCtx, requestTimeout)
	defer cancel()
	_, err := s.client.call(ctx, values)
	return err
}

// waitUntilReady polls the endpoint until the video is open at the requested
// start, VLC exits or time runs out. VLC answers before it opens the file and
// reports 0 s before applying --start-time; publishing those states would show
// the subtitles of the wrong moment. An answer that is not our endpoint's means
// another program holds the port.
func (s *Session) waitUntilReady(ctx context.Context, start time.Duration) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()
	var last status
	answered, seeked := false, false
	var openedAt time.Time
	for {
		attempt, cancel := context.WithTimeout(ctx, requestTimeout)
		current, err := s.client.call(attempt, nil)
		cancel()
		if errors.Is(err, errForeignEndpoint) {
			return err
		}
		if err == nil {
			if positioned(current, start) {
				return nil
			}
			last, answered = current, true
			open := current.Status != statusStopped && current.Length > 0
			if open && openedAt.IsZero() {
				openedAt = time.Now()
			}
			// With --start-paused VLC sometimes pauses before applying
			// --start-time and stays at 0 s; an exact seek finishes the job, but
			// only once VLC had its chance to apply the start itself.
			if !seeked && open && start > 0 && start < microseconds(current.Length) && time.Since(openedAt) >= startSeekGrace {
				seeked = s.SeekTo(start) == nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.exited:
			return errors.New("VLC closed before opening the video")
		case <-deadline.C:
			if answered {
				return fmt.Errorf("%w (last state: %s at %d of %d µs)", ErrStartTimeout, last.Status, last.Time, last.Length)
			}
			return ErrStartTimeout
		case <-time.After(startRetry):
		}
	}
}

// positioned reports whether VLC has opened the video and reached the start
// position. VLC ignores a start at or beyond the end and stays at 0 s.
func positioned(current status, start time.Duration) bool {
	if current.Status == statusStopped || current.Length <= 0 {
		return false
	}
	if start >= microseconds(current.Length) {
		start = 0
	}
	return microseconds(current.Time) >= start-startTolerance
}

func (s *Session) startPolling() {
	s.polling.Store(true)
	go s.poll()
}

// Reconnecting reports VLC running but not answering, while the poller
// tries again.
func (s *Session) Reconnecting() bool {
	return s.reconnecting.Load()
}

// poll reads the status on a fixed interval and publishes it as a sample,
// replacing a sample the consumer has not read yet. When VLC stops
// answering it tries again with growing waits before giving the session up;
// a VLC that exited ends the polling at once.
func (s *Session) poll() {
	defer close(s.pollDone)
	defer close(s.samples)
	defer s.reconnecting.Store(false)
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()
	lastSuccess := time.Now()
	retry := time.Duration(0) // zero while VLC answers

	for {
		select {
		case <-s.pollCtx.Done():
			return
		case <-s.exited:
			return
		case <-timer.C:
		}
		started := time.Now()
		ctx, cancel := context.WithTimeout(s.pollCtx, requestTimeout)
		current, err := s.client.call(ctx, nil)
		cancel()
		finished := time.Now()
		if err != nil {
			if s.pollCtx.Err() != nil {
				return
			}
			if finished.Sub(lastSuccess) > reconnectLimit {
				s.logger.Warn("VLC stopped answering", "error", err)
				s.lost.Store(true)
				return
			}
			if retry == 0 {
				retry = firstRetry
				s.reconnecting.Store(true)
				s.logger.Warn("VLC is not answering; reconnecting", "error", err)
			} else {
				retry = min(2*retry, maxRetry)
			}
			timer.Reset(retry)
			continue
		}
		if retry != 0 {
			s.logger.Info("VLC answered again", "after", finished.Sub(lastSuccess).Round(time.Millisecond))
			retry = 0
			s.reconnecting.Store(false)
		}
		lastSuccess = finished
		s.publish(toSample(current, started.Add(finished.Sub(started)/2)))
		timer.Reset(pollInterval)
	}
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

func toSample(current status, observedAt time.Time) playback.Sample {
	sample := playback.Sample{
		Position:   microseconds(current.Time),
		Duration:   microseconds(current.Length),
		Speed:      playback.Speed(current.Rate),
		Status:     playback.StatusStopped,
		ObservedAt: observedAt,
	}
	switch current.Status {
	case statusPlaying:
		sample.Status = playback.StatusPlaying
	case statusPaused:
		sample.Status = playback.StatusPaused
	}
	return sample
}

// microseconds converts the endpoint's integer microseconds.
func microseconds(value int64) time.Duration {
	return time.Duration(value) * time.Microsecond
}

// Close asks VLC to quit, ends it through the job object if it lingers, and
// removes the temporary endpoint folder. Every wait is bounded.
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closing.Store(true)
		quit, cancel := context.WithTimeout(context.Background(), quitTimeout)
		// VLC may already be gone; the job object below ends it either way.
		_, _ = s.client.call(quit, command("quit"))
		cancel()
		s.stop()
		waitClosed(s.exited, exitWait)
		if jobErr := s.job.Close(); jobErr != nil {
			err = errors.Join(err, fmt.Errorf("ending VLC: %w", jobErr))
		}
		if !waitClosed(s.exited, exitWait) {
			err = errors.Join(err, errors.New("VLC did not exit"))
		}
		if s.polling.Load() && !waitClosed(s.pollDone, requestTimeout) {
			err = errors.Join(err, errors.New("the VLC status reader did not stop"))
		}
		if removeErr := os.RemoveAll(s.endpointDir); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("removing the VLC endpoint folder: %w", removeErr))
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
