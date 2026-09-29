//go:build windows

// Package vlc drives VLC 3 through its HTTP interface, bound to the loopback
// address with a random port and password, using the LingoReel's own
// Lua endpoint for microsecond positions.
package vlc

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/windows/job"
)

//go:embed http
var endpointFiles embed.FS

// ErrStartTimeout reports a VLC that never answered on its control port.
var ErrStartTimeout = errors.New("VLC did not start its control interface in time")

const (
	startTimeout  = 15 * time.Second
	startRetry    = 100 * time.Millisecond
	passwordBytes = 16
)

// Arguments returns the command line given to VLC: the bare video window
// (no VLC interface), its own subtitles disabled, no network lookups, and the
// control interface bound to 127.0.0.1.
func Arguments(request player.LaunchRequest, port int, password, endpointDir string) []string {
	args := []string{
		"--intf", "dummy", "--dummy-quiet",
		"--extraintf", "http",
		"--http-host", "127.0.0.1",
		"--http-port", strconv.Itoa(port),
		"--http-password", password,
		"--http-src", endpointDir,
		"--no-one-instance",
		"--no-spu", "--no-sub-autodetect-file",
		"--no-video-title-show",
		"--no-metadata-network-access",
		"--play-and-pause",
	}
	if request.StartPaused {
		args = append(args, "--start-paused")
	}
	if request.AudioIndex >= 0 {
		args = append(args, "--audio-track", strconv.Itoa(request.AudioIndex))
	}
	if request.Start > 0 {
		args = append(args, "--start-time", strconv.FormatFloat(request.Start.Seconds(), 'f', 3, 64))
	}
	return append(args, request.File)
}

// Launch starts VLC on a video and waits until its control interface answers.
func Launch(ctx context.Context, executable string, request player.LaunchRequest) (*Session, error) {
	if !filepath.IsAbs(executable) || !isRegularFile(executable) {
		return nil, fmt.Errorf("VLC executable %q not found", executable)
	}
	if !isRegularFile(request.File) {
		return nil, fmt.Errorf("video %q not found", request.File)
	}

	endpointDir, err := writeEndpoint()
	if err != nil {
		return nil, err
	}
	port, err := freeLoopbackPort()
	if err != nil {
		os.RemoveAll(endpointDir)
		return nil, err
	}
	password, err := randomPassword()
	if err != nil {
		os.RemoveAll(endpointDir)
		return nil, err
	}

	cmd := exec.Command(executable, Arguments(request, port, password, endpointDir)...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(endpointDir)
		return nil, fmt.Errorf("starting VLC: %w", err)
	}
	killer, err := job.NewKillOnClose()
	if err == nil {
		err = killer.Assign(cmd.Process.Pid)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		os.RemoveAll(endpointDir)
		return nil, err
	}

	session := newSession(cmd, killer, newClient(port, password), endpointDir)
	if err := session.waitUntilReady(ctx); err != nil {
		session.Close()
		return nil, err
	}
	session.startPolling()
	return session, nil
}

// writeEndpoint copies the embedded Lua endpoint to a private temporary folder
// that VLC serves through --http-src.
func writeEndpoint() (string, error) {
	dir, err := os.MkdirTemp("", "lingoreel-vlc-*")
	if err != nil {
		return "", fmt.Errorf("creating the VLC endpoint folder: %w", err)
	}
	err = fs.WalkDir(endpointFiles, "http", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := endpointFiles.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(path[len("http/"):]))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("writing the VLC endpoint: %w", err)
	}
	return dir, nil
}

func freeLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("finding a free local port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func randomPassword() (string, error) {
	buffer := make([]byte, passwordBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("creating the VLC password: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
