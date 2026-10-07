//go:build !windows

package ffmpeg

import (
	"os"
	"os/exec"
	"syscall"
)

// background needs nothing on systems where a program opens no console.
func background() *syscall.SysProcAttr { return nil }

// contain leaves FFmpeg to its context until each system ties children to
// the app.
func contain(*os.Process) {}

// candidates is the PATH until each system's usual places are added.
func candidates() []string {
	if onPath, err := exec.LookPath("ffmpeg"); err == nil {
		return []string{onPath}
	}
	return nil
}
