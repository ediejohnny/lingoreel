//go:build windows

// Package plugintest is the conformance kit every player plugin must pass.
// A plugin's tests call Descriptor and Launch with its player; the kit
// checks the contract of players.Plugin: an honest descriptor, a launch that
// a cancelled context stops without leaving a process, capabilities that
// tell the truth (volume included), a player that draws into the app's window
// when it says so, and a bounded, repeatable Close that leaves nothing behind.
package plugintest

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/players"
	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
)

// Bounds of the contract.
const (
	// CancelBound is how fast a launch must give up on a cancelled context.
	CancelBound = 5 * time.Second
	// CloseBound is how long Close may take, including the samples closing.
	CloseBound = 10 * time.Second
	// settleTime lets a stopped launch's process go away before counting.
	settleTime = time.Second
	// HostBound is how long a player may take to draw into the app's window.
	HostBound = 10 * time.Second
)

var procEnumChildWindows = windows.NewLazySystemDLL("user32.dll").NewProc("EnumChildWindows")

// T is the part of testing.T the kit uses, so the kit itself can be tested.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Descriptor checks what the plugin says about its player.
func Descriptor(t T, plugin players.Plugin) {
	t.Helper()
	if err := plugin.Descriptor().Validate(); err != nil {
		t.Errorf("descriptor: %v", err)
	}
	if _, err := players.NewRegistry(plugin); err != nil {
		t.Errorf("the registry refuses the plugin: %v", err)
	}
}

// Launch checks the plugin against its real player, found at executable,
// opening video (a short file the player can play). A plugin that draws
// into the app's window gets one, off screen and without the focus, and
// must put its video inside it.
func Launch(t T, plugin players.Plugin, executable, video string) {
	t.Helper()
	request := playerRequest(video)
	if plugin.Descriptor().DrawsInHost {
		host, err := playerwindow.Open(context.Background(), playerwindow.Options{
			Title: "plugintest", Placement: player.Placement{X: -3000, Y: 100, Width: 640, Height: 360}, NoActivate: true,
		})
		if err != nil {
			t.Fatalf("opening a window to draw into: %v", err)
		}
		defer host.Close()
		request.Host = host.Handle()
	}

	// A cancelled context: fail fast, return nothing, leave no process.
	before := countProcesses(executable)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	running, err := plugin.Launch(cancelled, executable, request)
	if elapsed := time.Since(start); elapsed > CancelBound {
		t.Errorf("Launch with a cancelled context took %v, more than %v", elapsed, CancelBound)
	}
	if err == nil {
		if running != nil {
			running.Close()
		}
		t.Errorf("Launch with a cancelled context succeeded")
	}
	time.Sleep(settleTime)
	if after := countProcesses(executable); after > before {
		t.Errorf("a cancelled launch left %d %s process(es) running", after-before, filepath.Base(executable))
	}

	running, err = plugin.Launch(context.Background(), executable, request)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if running == nil {
		t.Fatalf("Launch returned no player and no error")
	}
	pid := running.ProcessID()
	if pid <= 0 {
		t.Errorf("ProcessID = %d, want the player's process", pid)
	}
	if request.Host != 0 && !drawsInto(windows.HWND(request.Host), pid) {
		t.Errorf("the descriptor says DrawsInHost but the player did not draw into the app's window within %v", HostBound)
	}
	honestCapabilities(t, running)
	closeCleanly(t, running, pid)
}

// drawsInto waits for a window of the player's process inside the host.
func drawsInto(host windows.HWND, pid int) bool {
	deadline := time.Now().Add(HostBound)
	for time.Now().Before(deadline) {
		found := false
		callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
			var owner uint32
			if _, err := windows.GetWindowThreadProcessId(hwnd, &owner); err == nil && int(owner) == pid {
				found = true
			}
			return 1
		})
		procEnumChildWindows.Call(uintptr(host), callback, 0)
		if found {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// playerRequest opens the video paused, keeping the player's audio choice.
func playerRequest(video string) player.LaunchRequest {
	return player.LaunchRequest{File: video, AudioIndex: -1, StartPaused: true}
}

// honestCapabilities runs every command: those the player declares must
// work, the others must fail instead of pretending.
func honestCapabilities(t T, running players.Running) {
	t.Helper()
	capabilities := running.Capabilities()
	checks := []struct {
		name     string
		declared bool
		run      func() error
	}{
		{"Pause", capabilities.Pause, running.Pause},
		{"Play", capabilities.Pause, running.Play},
		{"SeekTo", capabilities.Seek, func() error { return running.SeekTo(time.Second) }},
		{"SetSpeed", capabilities.Speed, func() error { return running.SetSpeed(playback.Speed(1)) }},
		{"StepFrame forward", capabilities.FrameStepForward, func() error { return running.StepFrame(true) }},
		// An emulated backward step is the session's work, not the player's.
		{"StepFrame backward", capabilities.FrameStepBackward && !capabilities.FrameStepBackwardEmulated, func() error { return running.StepFrame(false) }},
		{"SelectAudioTrack", capabilities.AudioTrackSelection, func() error { return running.SelectAudioTrack(0) }},
	}
	for _, check := range checks {
		err := check.run()
		switch {
		case check.declared && err != nil:
			t.Errorf("%s is declared but failed: %v", check.name, err)
		case !check.declared && err == nil:
			t.Errorf("%s is not declared but pretended to work", check.name)
		}
	}
	volume, hasVolume := running.(session.VolumePlayer)
	switch {
	case capabilities.Volume && !hasVolume:
		t.Errorf("declares Volume without a SetVolume method")
	case capabilities.Volume:
		if err := volume.SetVolume(playback.MaxVolume); err != nil {
			t.Errorf("SetVolume is declared but failed: %v", err)
		}
	case hasVolume && volume.SetVolume(playback.MaxVolume) == nil:
		t.Errorf("SetVolume is not declared but pretended to work")
	}
	if p := capabilities.Precision; p != playback.PrecisionExact && p != playback.PrecisionEstimated {
		t.Errorf("Precision %q is neither exact nor estimated", p)
	}
}

// closeCleanly closes the player and checks that everything ends: the call,
// the samples, the process, and a second Close.
func closeCleanly(t T, running players.Running, pid int) {
	t.Helper()
	start := time.Now()
	if err := running.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed > CloseBound {
		t.Errorf("Close took %v, more than %v", elapsed, CloseBound)
	}
	deadline := time.After(CloseBound)
drain:
	for {
		select {
		case _, open := <-running.Samples():
			if !open {
				break drain
			}
		case <-deadline:
			t.Errorf("Samples still open %v after Close", CloseBound)
			break drain
		}
	}
	if err := running.Err(); err != nil {
		t.Errorf("Err after a normal Close = %v, want nil (only a crash is %v)", err, playback.ErrPlayerLost)
	}
	if pid > 0 && processAlive(pid) {
		t.Errorf("process %d is still running after Close", pid)
	}
	if err := running.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// countProcesses counts the running processes with the executable's name.
func countProcesses(executable string) int {
	name := strings.ToLower(filepath.Base(executable))
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	count := 0
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if strings.ToLower(windows.UTF16ToString(entry.ExeFile[:])) == name {
			count++
		}
	}
	return count
}

// processAlive reports a process that has not exited.
func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER) // gone: no such process
	}
	defer windows.CloseHandle(handle)
	event, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && event == uint32(windows.WAIT_TIMEOUT)
}
