// Command spdev is the developer tool that exercises LingoReel subsystems
// without the graphical application.
package main

import (
	"fmt"
	"os"

	"lingoreel/internal/spdev"
)

func main() {
	if err := spdev.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "spdev:", err)
		os.Exit(1)
	}
}
