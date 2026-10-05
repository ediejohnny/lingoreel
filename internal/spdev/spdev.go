// Package spdev implements the commands of the spdev developer tool, which
// exercises LingoReel subsystems without the graphical application.
package spdev

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ediejohnny/lingoreel/internal/buildinfo"
)

// ErrUnknownCommand reports a command name that spdev does not know.
var ErrUnknownCommand = errors.New("unknown command")

// Run executes the command named by args[0], writing its output to stdout.
// Long-running commands stop when ctx ends.
func Run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(stdout, buildinfo.Version)
		return err
	case "inspect":
		return inspect(ctx, args[1:], stdout)
	case "play":
		return play(ctx, args[1:], stdout, false)
	case "overlay":
		return play(ctx, args[1:], stdout, true)
	case "diag":
		return diag(stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stdout)
		return fmt.Errorf("%w: %q", ErrUnknownCommand, args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: spdev <command>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version                         shows this build's version")
	fmt.Fprintln(w, "  inspect <video> [--subtitles]   shows the tracks and the study suggestion")
	fmt.Fprintln(w, "  play <video> [options]          opens the video in the player and prints the subtitles in time")
	fmt.Fprintln(w, "      --player <id>                   preferred player (vlc, potplayer)")
	fmt.Fprintln(w, "      --start <seconds>               starts from this position")
	fmt.Fprintln(w, "      --paused                        opens paused")
	fmt.Fprintln(w, "      --step <ms>                     step of the sync shortcuts (10 to 1000, 50 by default)")
	fmt.Fprintln(w, "  overlay <video> [options]       like play, drawing the subtitles over the video")
	fmt.Fprintln(w, "  diag                            shows the monitors, their scaling and a checklist")
	fmt.Fprintln(w, "  help                            shows this help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Sync (play and overlay, with the player in front):")
	fmt.Fprintln(w, "  Shift+, / Shift+.   top line earlier / later")
	fmt.Fprintln(w, "  Ctrl+, / Ctrl+.     bottom line earlier / later")
	fmt.Fprintln(w, "  holding a key speeds it up; on the console, z + Enter resets both")
}
