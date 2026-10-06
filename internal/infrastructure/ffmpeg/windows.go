//go:build windows

package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// background runs FFmpeg with no console window and at the idle priority:
// it only takes what the video leaves free.
func background() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.IDLE_PRIORITY_CLASS}
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
