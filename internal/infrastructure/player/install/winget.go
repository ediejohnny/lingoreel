// Package install installs players with winget, the Windows package manager.
// Only ids from the player plugins' descriptors are installed, always with
// exact matching, and the installer asks for elevation itself: the app never
// runs as administrator.
package install

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
)

var (
	// ErrNoWinget reports a computer without winget; the user gets the
	// official download page instead.
	ErrNoWinget = errors.New("winget is not available")
	// ErrInvalidPackage reports a package id that is not a winget id.
	ErrInvalidPackage = errors.New("invalid winget package id")
)

// Runner runs a program with separate arguments and waits for it.
type Runner func(ctx context.Context, program string, args ...string) error

// Winget installs packages through winget.
type Winget struct {
	// LookPath finds winget; nil uses exec.LookPath.
	LookPath func(file string) (string, error)
	// Run runs it; nil runs the real process.
	Run Runner
}

// Available reports whether winget can be used.
func (w Winget) Available() bool {
	_, err := w.lookPath("winget")
	return err == nil
}

// Install installs one package silently, accepting its agreements, without
// questions; it returns when winget finishes.
func (w Winget) Install(ctx context.Context, packageID string) error {
	if !player.ValidWingetID(packageID) {
		return fmt.Errorf("%w: %q", ErrInvalidPackage, packageID)
	}
	program, err := w.lookPath("winget")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoWinget, err)
	}
	run := w.Run
	if run == nil {
		run = runProcess
	}
	err = run(ctx, program, "install", "--id", packageID, "--exact", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity")
	if err != nil {
		return fmt.Errorf("installing %s with winget: %w", packageID, err)
	}
	return nil
}

func (w Winget) lookPath(file string) (string, error) {
	if w.LookPath != nil {
		return w.LookPath(file)
	}
	return exec.LookPath(file)
}

func runProcess(ctx context.Context, program string, args ...string) error {
	return exec.CommandContext(ctx, program, args...).Run()
}
