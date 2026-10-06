//go:build !windows

package ffmpeg

import (
	"os/exec"
	"syscall"
)

// background needs nothing on systems where a program opens no console.
func background() *syscall.SysProcAttr { return nil }

// candidates is the PATH until each system's usual places are added.
func candidates() []string {
	if onPath, err := exec.LookPath("ffmpeg"); err == nil {
		return []string{onPath}
	}
	return nil
}
