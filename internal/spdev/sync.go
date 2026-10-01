package spdev

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// resetCommand on the console puts both lines back at their files' timing.
const resetCommand = "z"

// readConsole calls run with each line typed on the console, trimmed and in
// lower case, until the context ends or the input closes. A pending read of
// the console cannot be interrupted: the goroutine ends with the process.
func readConsole(ctx context.Context, in io.Reader, run func(string)) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		run(strings.ToLower(strings.TrimSpace(scanner.Text())))
	}
}
