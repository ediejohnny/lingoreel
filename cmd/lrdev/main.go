// Command lrdev is the developer tool that exercises LingoReel subsystems
// without the graphical application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
	"github.com/ediejohnny/lingoreel/internal/lrdev"
)

func main() {
	// Before any window: screen sizes (the work area that places the player)
	// must come in physical pixels, the unit the players use at any scaling.
	win32.UsePerMonitorDPIForProcess()
	// Ctrl+C ends long-running commands cleanly, closing the player they opened.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := lrdev.Run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lrdev:", err)
		os.Exit(1)
	}
}
