//go:build windows

package plugintest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"lingoreel/internal/application/players"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/tests/infrastructure/player/plugintest"
)

// childEnv turns the test binary into a stand-in player process that just
// waits to be ended.
const childEnv = "LINGOREEL_FAKE_PLAYER_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childEnv) == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// flaw is one way a plugin can break the contract.
type flaw int

const (
	honest flaw = iota
	seekDeclaredButFails
	backwardPretends
	leavesProcess
	errAfterClose
	ignoresCancel
	leavesProcessWhenCancelled
	samplesNeverClose
	volumeWithoutMethod
	claimsToDrawInHost
)

type fakePlugin struct {
	flaw       flaw
	descriptor player.Descriptor

	mu       sync.Mutex
	children []*exec.Cmd
}

// killAll ends every stand-in process, including those a flawed plugin left.
func (p *fakePlugin) killAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, child := range p.children {
		child.Process.Kill()
		child.Wait()
	}
}

func newFake(f flaw) *fakePlugin {
	return &fakePlugin{flaw: f, descriptor: player.Descriptor{
		ID: "fake", Name: "Fake",
		Install: player.Install{DownloadURL: "https://example.org/fake", Benefit: "Só para testes."},
		// The stand-in process has no window to put in the app's.
		DrawsInHost: f == claimsToDrawInHost,
	}}
}

func (p *fakePlugin) Descriptor() player.Descriptor { return p.descriptor }

func (p *fakePlugin) Launch(ctx context.Context, executable string, _ player.LaunchRequest) (players.Running, error) {
	if ctx.Err() != nil && p.flaw != ignoresCancel {
		if p.flaw == leavesProcessWhenCancelled {
			if _, err := p.startChild(executable); err != nil {
				return nil, err
			}
		}
		return nil, ctx.Err()
	}
	child, err := p.startChild(executable)
	if err != nil {
		return nil, err
	}
	samples := make(chan playback.Sample, 1)
	samples <- playback.Sample{Status: playback.StatusPaused, ObservedAt: time.Now()}
	return &fakeRunning{flaw: p.flaw, child: child, samples: samples}, nil
}

func (p *fakePlugin) startChild(executable string) (*exec.Cmd, error) {
	cmd := exec.Command(executable, "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.children = append(p.children, cmd)
	p.mu.Unlock()
	return cmd, nil
}

type fakeRunning struct {
	flaw    flaw
	child   *exec.Cmd
	samples chan playback.Sample
	once    sync.Once
}

var errUnsupported = errors.New("not supported")

func (r *fakeRunning) Capabilities() playback.Capabilities {
	return playback.Capabilities{Pause: true, Seek: true, Speed: true, FrameStepForward: true, Precision: playback.PrecisionEstimated,
		Volume: r.flaw == volumeWithoutMethod}
}
func (r *fakeRunning) Play() error  { return nil }
func (r *fakeRunning) Pause() error { return nil }
func (r *fakeRunning) SeekTo(time.Duration) error {
	if r.flaw == seekDeclaredButFails {
		return errors.New("seek failed")
	}
	return nil
}
func (r *fakeRunning) SetSpeed(playback.Speed) error { return nil }
func (r *fakeRunning) StepFrame(forward bool) error {
	if !forward && r.flaw != backwardPretends {
		return errUnsupported
	}
	return nil
}
func (r *fakeRunning) SelectAudioTrack(int) error      { return errUnsupported }
func (r *fakeRunning) Samples() <-chan playback.Sample { return r.samples }
func (r *fakeRunning) ProcessID() int                  { return r.child.Process.Pid }
func (r *fakeRunning) Err() error {
	if r.flaw == errAfterClose {
		return playback.ErrPlayerLost
	}
	return nil
}
func (r *fakeRunning) Close() error {
	r.once.Do(func() {
		if r.flaw != leavesProcess {
			r.child.Process.Kill()
			r.child.Wait()
		}
		if r.flaw != samplesNeverClose {
			close(r.samples)
		}
	})
	return nil
}

// recorder is a plugintest.T that keeps the failures.
type recorder struct {
	failures []string
}

type stopped struct{}

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}
func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	panic(stopped{})
}

func runKit(plugin players.Plugin) (failures []string) {
	r := &recorder{}
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, ok := recovered.(stopped); !ok {
				panic(recovered)
			}
		}
		failures = r.failures
	}()
	plugintest.Descriptor(r, plugin)
	plugintest.Launch(r, plugin, os.Args[0], "video.mkv")
	return r.failures
}

// fake creates a plugin whose stand-in processes end with the test.
func fake(t *testing.T, f flaw) *fakePlugin {
	plugin := newFake(f)
	t.Cleanup(plugin.killAll)
	return plugin
}

func TestTheKitPassesAnHonestPlugin(t *testing.T) {
	if failures := runKit(fake(t, honest)); len(failures) != 0 {
		t.Fatalf("honest plugin failed the kit:\n%s", strings.Join(failures, "\n"))
	}
}

func TestTheKitCatchesEveryFlaw(t *testing.T) {
	cases := map[string]struct {
		flaw flaw
		want string
	}{
		"declared seek fails":         {seekDeclaredButFails, "SeekTo is declared but failed"},
		"backward step pretends":      {backwardPretends, "StepFrame backward is not declared but pretended"},
		"process left after Close":    {leavesProcess, "still running after Close"},
		"normal close called a crash": {errAfterClose, "Err after a normal Close"},
		"cancelled launch succeeds":   {ignoresCancel, "cancelled context succeeded"},
		"cancelled launch leaves one": {leavesProcessWhenCancelled, "cancelled launch left"},
		"samples never close":         {samplesNeverClose, "Samples still open"},
		"volume without its method":   {volumeWithoutMethod, "declares Volume without"},
		"claims to draw in the app":   {claimsToDrawInHost, "did not draw into the app's window"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			failures := runKit(fake(t, c.flaw))
			for _, failure := range failures {
				if strings.Contains(failure, c.want) {
					return
				}
			}
			t.Fatalf("the kit missed %q; it reported:\n%s", c.want, strings.Join(failures, "\n"))
		})
	}
}

func TestTheKitCatchesABadDescriptor(t *testing.T) {
	plugin := fake(t, honest)
	plugin.descriptor.Install.DownloadURL = "http://example.org"
	r := &recorder{}
	plugintest.Descriptor(r, plugin)
	if len(r.failures) == 0 {
		t.Fatal("a plain http download page passed the kit")
	}
}
