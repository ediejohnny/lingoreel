// Package spdev implements the commands of the spdev developer tool, which
// exercises LingoReel subsystems without the graphical application.
package spdev

import (
	"errors"
	"fmt"
	"io"

	"lingoreel/internal/buildinfo"
)

// ErrUnknownCommand reports a command name that spdev does not know.
var ErrUnknownCommand = errors.New("comando desconhecido")

// Run executes the command named by args[0], writing its output to stdout.
func Run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(stdout, buildinfo.Version)
		return err
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stdout)
		return fmt.Errorf("%w: %q", ErrUnknownCommand, args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Uso: spdev <comando>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Comandos:")
	fmt.Fprintln(w, "  version   mostra a versão desta compilação")
	fmt.Fprintln(w, "  help      mostra esta ajuda")
}
