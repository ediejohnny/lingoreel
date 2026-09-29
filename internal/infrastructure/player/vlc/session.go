//go:build windows

package vlc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"lingoreel/internal/domain/playback"
	"lingoreel/internal/infrastructure/windows/job"
)

// ErrNotSupported reports a command VLC cannot perform.
var ErrNotSupported = errors.New("not supported by VLC")

const (
	pollInterval = 100 * time.Millisecond
	// unreachableLimit is how long the endpoint may fail before the session
	// is considered lost.
	unreachableLimit = 3 * time.Second
	quitTimeout      = time.Second
	exitWait         = 2 * time.Second
)

// Session is a running VLC playing one video.
type Session struct {
	cmd         *exec.Cmd
	job         *job.Job
	client      *client
	endpointDir string

	samples   chan playback.Sample
	exited    chan struct{}
	stop      context.CancelFunc
	pollCtx   context.Context
	pollDone  chan struct{}
	closeOnce sync.Once
}

func newSession(cmd *exec.Cmd, killer *job.Job, control *client, endpointDir string) *Session {
	ctx, stop := context.WithCancel(context.Background())
	s := &Session{
		cmd: cmd, job: killer, client: control, endpointDir: endpointDir,
		samples: make(chan playback.Sample, 1), exited: make(chan struct{}),
		stop: stop, pollCtx: ctx, pollDone: make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(s.exited)
	}()
	return s
}

// Capabilities reports what VLC offers: no backward frame step, and positions
// that are estimates updated a few times per second.
func (s *Session) Capabilities() playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, Speed: true,
		FrameStepForward: true, FrameStepBackward: false,
		AudioTrackSelection: true, SubtitleSuppression: true,
		Precision: playback.PrecisionEstimated,
	}
}

// ProcessID identifies the VLC process, whose window the overlay follows.
func (s *Session) ProcessID() int {
	return s.cmd.Process.Pid
}

// Samples delivers the latest observation roughly every 100 ms. It is closed
// when VLC exits or stops answering.
func (s *Session) Samples() <-chan playback.Sample {
	return s.samples
}

// Play resumes playback.
func (s *Session) Play() error { return s.send(command("play")) }

// Pause pauses playback.
func (s *Session) Pause() error { return s.send(command("pause")) }

// SeekTo moves to an absolute position.
func (s *Session) SeekTo(position time.Duration) error {
	return s.send(command("seek", "us", strconv.FormatInt(position.Microseconds(), 10)))
}

// SetSpeed changes the playback rate.
func (s *Session) SetSpeed(speed playback.Speed) error {
	return s.send(command("rate", "value", strconv.FormatFloat(float64(speed), 'f', 4, 64)))
}

// StepFrame advances one frame; VLC cannot step backward.
func (s *Session) StepFrame(forward bool) error {
	if !forward {
		return ErrNotSupported
	}
	return s.send(command("frame"))
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

// waitUntilReady polls the endpoint until VLC answers, exits or time runs out.
func (s *Session) waitUntilReady(ctx context.Context) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()
	for {
		attempt, cancel := context.WithTimeout(ctx, requestTimeout)
		_, err := s.client.call(attempt, nil)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.exited:
			return errors.New("VLC closed before its control interface started")
		case <-deadline.C:
			return ErrStartTimeout
		case <-time.After(startRetry):
		}
	}
}

func (s *Session) startPolling() {
	go s.poll()
}

// poll reads the status on a fixed interval and publishes it as a sample,
// replacing a sample the consumer has not read yet.
func (s *Session) poll() {
	defer close(s.pollDone)
	defer close(s.samples)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	lastSuccess := time.Now()

	for {
		select {
		case <-s.pollCtx.Done():
			return
		case <-s.exited:
			return
		case <-ticker.C:
		}
		started := time.Now()
		ctx, cancel := context.WithTimeout(s.pollCtx, requestTimeout)
		current, err := s.client.call(ctx, nil)
		cancel()
		finished := time.Now()
		if err != nil {
			if finished.Sub(lastSuccess) > unreachableLimit {
				return
			}
			continue
		}
		lastSuccess = finished
		s.publish(toSample(current, started.Add(finished.Sub(started)/2)))
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
		Position:   time.Duration(current.Time) * time.Microsecond,
		Duration:   time.Duration(current.Length) * time.Microsecond,
		Speed:      playback.Speed(current.Rate),
		Status:     playback.StatusStopped,
		ObservedAt: observedAt,
	}
	switch current.Status {
	case "playing":
		sample.Status = playback.StatusPlaying
	case "paused":
		sample.Status = playback.StatusPaused
	}
	return sample
}

// Close asks VLC to quit, then ends it through the job object if needed, and
// removes the temporary endpoint folder.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		quit, cancel := context.WithTimeout(context.Background(), quitTimeout)
		_, _ = s.client.call(quit, command("quit"))
		cancel()
		s.stop()
		select {
		case <-s.exited:
		case <-time.After(exitWait):
		}
		_ = s.job.Close()
		<-s.exited
		os.RemoveAll(s.endpointDir)
	})
	return nil
}
