// Command spdev is the developer tool that exercises LingoReel subsystems
// without the graphical application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"lingoreel/internal/spdev"
)

func main() {
	// Ctrl+C ends long-running commands cleanly, closing the player they opened.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := spdev.Run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "spdev:", err)
		os.Exit(1)
	}
}
