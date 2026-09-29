package session_test

import (
	"sync"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/subtitle"
)

var epoch = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

// fakeTime is a manual clock with timers that fire only when time advances.
type fakeTime struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*fakeTimer
	created chan struct{}
}

type fakeTimer struct {
	owner   *fakeTime
	at      time.Time
	channel chan time.Time
	done    bool
}

func newFakeTime() *fakeTime {
	return &fakeTime{now: epoch, created: make(chan struct{}, 1024)}
}

func (f *fakeTime) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeTime) NewTimer(d time.Duration) session.Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	timer := &fakeTimer{owner: f, at: f.now.Add(d), channel: make(chan time.Time, 1)}
	f.timers = append(f.timers, timer)
	f.created <- struct{}{}
	return timer
}

func (t *fakeTimer) C() <-chan time.Time { return t.channel }

func (t *fakeTimer) Stop() bool {
	t.owner.mu.Lock()
	defer t.owner.mu.Unlock()
	wasActive := !t.done
	t.done = true
	return wasActive
}

// Advance moves time forward and fires every timer that became due.
func (f *fakeTime) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	for _, timer := range f.timers {
		if !timer.done && !timer.at.After(f.now) {
			timer.done = true
			timer.channel <- f.now
		}
	}
}

// waitForTimer blocks until the code under test created a new timer, which
// means it is idle and waiting for time to pass.
func (f *fakeTime) waitForTimer(t *testing.T) {
	t.Helper()
	select {
	case <-f.created:
	case <-time.After(2 * time.Second):
		t.Fatal("the runner never went back to waiting")
	}
}

// fakePlayer records the commands it receives.
type fakePlayer struct {
	capabilities playback.Capabilities
	err          error
	calls        []string
	seekTarget   time.Duration
	speed        playback.Speed
	audioIndex   int
}

func (p *fakePlayer) Capabilities() playback.Capabilities { return p.capabilities }
func (p *fakePlayer) Play() error                         { return p.record("play") }
func (p *fakePlayer) Pause() error                        { return p.record("pause") }

func (p *fakePlayer) SeekTo(position time.Duration) error {
	p.seekTarget = position
	return p.record("seek")
}

func (p *fakePlayer) SetSpeed(speed playback.Speed) error {
	p.speed = speed
	return p.record("speed")
}

func (p *fakePlayer) StepFrame(forward bool) error {
	if forward {
		return p.record("frame+")
	}
	return p.record("frame-")
}

func (p *fakePlayer) SelectAudioTrack(index int) error {
	p.audioIndex = index
	return p.record("audio")
}

func (p *fakePlayer) record(call string) error {
	p.calls = append(p.calls, call)
	return p.err
}

func allCapabilities(precision playback.Precision) playback.Capabilities {
	return playback.Capabilities{
		Pause: true, Seek: true, Speed: true, FrameStepForward: true, FrameStepBackward: true,
		AudioTrackSelection: true, SubtitleSuppression: true, Precision: precision,
	}
}

func ms(value int) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func sample(position time.Duration, status playback.Status, speed playback.Speed, at time.Time) playback.Sample {
	return playback.Sample{Position: position, Duration: 10 * time.Minute, Speed: speed, Status: status, ObservedAt: at}
}

func timeline(t *testing.T, events ...subtitle.Event) *subtitle.Timeline {
	t.Helper()
	built, err := subtitle.NewTimeline(events)
	if err != nil {
		t.Fatalf("NewTimeline: %v", err)
	}
	return built
}

func cue(id string, start, end int) subtitle.Event {
	return subtitle.Event{ID: id, Start: ms(start), End: ms(end), OriginalText: "text " + id}
}

func ids(events []subtitle.Event) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.ID)
	}
	return out
}
