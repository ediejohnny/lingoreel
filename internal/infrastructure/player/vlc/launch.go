//go:build windows

// Package vlc drives VLC 3 through its HTTP interface, bound to the loopback
// address with a random port and password, using the LingoReel's own
// Lua endpoint for microsecond positions.
package vlc

import (
	"bytes"
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

	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/player"
	"lingoreel/internal/infrastructure/windows/job"
)

//go:embed http
var endpointFiles embed.FS

// ErrStartTimeout reports a VLC that did not answer on its control port, or
// did not open the video at the requested position, in time.
var ErrStartTimeout = errors.New("VLC did not open the video in time")

const (
	startTimeout = 15 * time.Second
	startRetry   = 100 * time.Millisecond
	secretBytes  = 16
	// maxPortAttempts bounds the ports tried when another program answers on
	// the one picked (it was free when chosen, then taken before VLC bound it).
	maxPortAttempts = 3
)

// Placeholders of the endpoint template, filled for each session.
const (
	tokenPlaceholder   = "{{SESSION_TOKEN}}"
	maxRatePlaceholder = "{{MAX_RATE}}"
)

// Options tunes a launch; the zero value is what the app uses.
type Options struct {
	// PickPort returns the loopback port for VLC's control interface; nil
	// picks a free one.
	PickPort func() (int, error)
}

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
	if window := request.Window; window.Set() {
		// VLC opens its window at the video's size times --zoom, which may
		// not fit the monitor (a 1080p movie on a 900-pixel screen); its
		// --width/--height do not size the window.
		args = append(args, "--video-x", strconv.Itoa(window.X), "--video-y", strconv.Itoa(window.Y))
		if request.VideoWidth > 0 && window.Width < request.VideoWidth {
			zoom := float64(window.Width) / float64(request.VideoWidth)
			args = append(args, "--zoom", strconv.FormatFloat(zoom, 'f', 4, 64))
		}
	}
	return append(args, request.File)
}

// Launch starts VLC on a video and waits until its control interface answers.
func Launch(ctx context.Context, executable string, request player.LaunchRequest) (*Session, error) {
	return LaunchWith(ctx, executable, request, Options{})
}

// LaunchWith is Launch with options. When another program answers on the
// control port, VLC is ended and started again on another port.
func LaunchWith(ctx context.Context, executable string, request player.LaunchRequest, options Options) (*Session, error) {
	if !filepath.IsAbs(executable) || !isRegularFile(executable) {
		return nil, fmt.Errorf("VLC executable %q not found", executable)
	}
	file, err := filepath.Abs(request.File)
	if err != nil || !isRegularFile(file) {
		return nil, fmt.Errorf("video %q not found", request.File)
	}
	// An absolute path starts with a drive or a share, never with "-", so
	// VLC cannot take a file name for one of its options.
	request.File = file
	pickPort := options.PickPort
	if pickPort == nil {
		pickPort = freeLoopbackPort
	}

	var lastErr error
	for range maxPortAttempts {
		session, err := launchOnce(ctx, executable, request, pickPort)
		if !errors.Is(err, errForeignEndpoint) {
			return session, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("starting VLC on %d ports: %w", maxPortAttempts, lastErr)
}

func launchOnce(ctx context.Context, executable string, request player.LaunchRequest, pickPort func() (int, error)) (*Session, error) {
	password, err := randomSecret()
	if err != nil {
		return nil, fmt.Errorf("creating the VLC password: %w", err)
	}
	token, err := randomSecret()
	if err != nil {
		return nil, fmt.Errorf("creating the VLC session token: %w", err)
	}
	port, err := pickPort()
	if err != nil {
		return nil, err
	}
	endpointDir, err := writeEndpoint(token)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(executable, Arguments(request, port, password, endpointDir)...)
	if err := cmd.Start(); err != nil {
		removeEndpoint(endpointDir)
		return nil, fmt.Errorf("starting VLC: %w", err)
	}
	killer, err := job.NewKillOnClose()
	if err == nil {
		err = killer.Assign(cmd.Process.Pid)
	}
	if err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil {
			err = errors.Join(err, fmt.Errorf("ending VLC: %w", killErr))
		}
		removeEndpoint(endpointDir)
		return nil, fmt.Errorf("tying VLC to the app: %w", err)
	}

	session := newSession(cmd, killer, newClient(port, password, token), endpointDir)
	if err := session.waitUntilReady(ctx, request.Start); err != nil {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, err
	}
	session.startPolling()
	return session, nil
}

// removeEndpoint deletes the endpoint folder of a launch that failed; the
// folder is in the user's temporary directory, so a failure only leaves it
// for the system's cleanup.
func removeEndpoint(dir string) {
	_ = os.RemoveAll(dir)
}

// writeEndpoint copies the embedded Lua endpoint to a private temporary
// folder that VLC serves through --http-src, filling in the session token
// and the highest speed the app uses.
func writeEndpoint(token string) (string, error) {
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
		data = bytes.ReplaceAll(data, []byte(tokenPlaceholder), []byte(token))
		data = bytes.ReplaceAll(data, []byte(maxRatePlaceholder), []byte(strconv.FormatFloat(float64(playback.MaxSpeed), 'f', -1, 64)))
		target := filepath.Join(dir, filepath.FromSlash(path[len("http/"):]))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		removeEndpoint(dir)
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

// randomSecret returns a random hex string, safe inside a Lua string literal.
func randomSecret() (string, error) {
	buffer := make([]byte, secretBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
