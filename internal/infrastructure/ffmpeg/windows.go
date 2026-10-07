//go:build windows

package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/job"
)

// background runs FFmpeg with no console window and at the idle priority:
// it only takes what the video leaves free.
func background() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.IDLE_PRIORITY_CLASS}
}

// reaper is the job every FFmpeg joins: its handle lives as long as the app,
// and Windows ends what is in it when the app ends.
var reaper struct {
	once sync.Once
	job  *job.Job
}

// contain puts a running FFmpeg in the reaper. One that ended at once is
// already gone, and a job that could not be made leaves it to its context.
func contain(process *os.Process) {
	reaper.once.Do(func() { reaper.job, _ = job.NewKillOnClose() })
	if reaper.job != nil {
		_ = reaper.job.Assign(process.Pid)
	}
}

// candidates are the places FFmpeg is usually installed on Windows: next to
// the app, on the PATH, where winget puts it, and the usual folders.
func candidates() []string {
	var found []string
	if app, err := os.Executable(); err == nil {
		found = append(found, filepath.Join(filepath.Dir(app), "ffmpeg.exe"))
	}
	if onPath, err := exec.LookPath("ffmpeg"); err == nil {
		found = append(found, onPath)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		found = append(found, filepath.Join(local, `Microsoft\WinGet\Links\ffmpeg.exe`))
		packages, _ := filepath.Glob(filepath.Join(local, `Microsoft\WinGet\Packages\*FFmpeg*\*\bin\ffmpeg.exe`))
		found = append(found, packages...)
	}
	if programs := os.Getenv("ProgramFiles"); programs != "" {
		found = append(found, filepath.Join(programs, `ffmpeg\bin\ffmpeg.exe`))
	}
	if drive := os.Getenv("SystemDrive"); drive != "" {
		found = append(found, filepath.Join(drive+`\`, `ffmpeg\bin\ffmpeg.exe`))
	}
	return found
}
