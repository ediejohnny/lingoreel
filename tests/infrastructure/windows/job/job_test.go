//go:build windows

package job_test

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// childMode makes the test binary a child that waits for a byte on its
// standard input, then allocates or spins and ends.
const childMode = "LINGOREEL_JOB_CHILD"

// spinWork is the processor time a spinning child takes. At a fiftieth of a
// processor it lasts some 2.5 s, where a free child takes a fraction of a
// second.
const (
	spinWork = 50 * time.Millisecond
	spinCap  = 0.02
)

func TestMain(m *testing.M) {
	switch os.Getenv(childMode) {
	case "":
		os.Exit(m.Run())
	case "allocate":
		_, _ = io.ReadFull(os.Stdin, make([]byte, 1))
		memory := make([]byte, 256<<20)
		for i := 0; i < len(memory); i += 4096 {
			memory[i] = 1
		}
	case "spin":
		_, _ = io.ReadFull(os.Stdin, make([]byte, 1))
		for spent() < spinWork {
			for i := range 1_000_000 {
				_ = i * i
			}
		}
	}
	os.Exit(0)
}

// spent is the processor time this process took.
func spent() time.Duration {
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(windows.CurrentProcess(), &created, &exited, &kernel, &user) != nil {
		return time.Hour
	}
	// A FILETIME here counts hundreds of nanoseconds, not a date.
	ticks := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	return time.Duration(ticks(kernel)+ticks(user)) * 100
}

// runChild runs a child in the job (nil: in none), lets it go, and tells
// how long it took from then and how it ended.
func runChild(t *testing.T, mode string, in *job.Job) (time.Duration, error) {
	t.Helper()
	child := exec.Command(os.Args[0], "-test.run=^$")
	child.Env = append(os.Environ(), childMode+"="+mode)
	ahead, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatalf("starting the child: %v", err)
	}
	t.Cleanup(func() { _ = child.Process.Kill() })
	if in != nil {
		if err := in.Assign(child.Process.Pid); err != nil {
			t.Fatalf("Assign: %v", err)
		}
	}
	began := time.Now()
	if _, err := ahead.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	err = child.Wait()
	return time.Since(began), err
}

// startLongProcess starts a harmless command that would run for a minute.
func startLongProcess(t *testing.T) (*exec.Cmd, chan error) {
	t.Helper()
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting helper process: %v", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, exited
}

func TestClosingTheJobEndsItsProcesses(t *testing.T) {
	killer, err := job.NewKillOnClose()
	if err != nil {
		t.Fatalf("NewKillOnClose: %v", err)
	}
	cmd, exited := startLongProcess(t)
	if err := killer.Assign(cmd.Process.Pid); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	select {
	case <-exited:
		t.Fatal("the process ended before the job was closed")
	case <-time.After(300 * time.Millisecond):
	}

	if err := killer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("closing the job did not end the process")
	}
	if err := killer.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestAssignFailsForAMissingProcess(t *testing.T) {
	killer, err := job.NewKillOnClose()
	if err != nil {
		t.Fatalf("NewKillOnClose: %v", err)
	}
	defer killer.Close()

	cmd, exited := startLongProcess(t)
	_ = cmd.Process.Kill()
	<-exited
	if err := killer.Assign(cmd.Process.Pid); err == nil {
		t.Fatal("Assign accepted a process that already ended")
	}
}

// A job's limits hold its processes: past its memory a process's
// allocations fail, and together they take no more than their share of a
// processor, even while the machine is busy.
func TestAJobHoldsItsProcessesToItsLimits(t *testing.T) {
	testkit.Slow(t)
	held, err := job.New(job.Limits{Memory: 64 << 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer held.Close()
	if _, err := runChild(t, "allocate", nil); err != nil {
		t.Fatalf("free, the child could not allocate 256 MB: %v", err)
	}
	if _, err := runChild(t, "allocate", held); err == nil {
		t.Fatal("held to 64 MB, the child allocated 256 MB")
	}

	capped, err := job.New(job.Limits{CPU: spinCap})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer capped.Close()
	slow, err := runChild(t, "spin", capped)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v of work took %v at a fiftieth of a processor", spinWork, slow)
	// spinWork / spinCap is 2.5 s, which Windows enforces over intervals, so
	// a run may come in somewhat under it; a busy machine only makes it
	// longer, so the floor holds however loaded it is.
	if slow < time.Second {
		t.Fatalf("at a fiftieth of a processor %v of work took %v", spinWork, slow)
	}
}
