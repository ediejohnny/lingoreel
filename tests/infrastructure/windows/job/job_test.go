//go:build windows

package job_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
)

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
