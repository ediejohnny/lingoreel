// Command spdev is the developer tool that exercises LingoReel subsystems
// without the graphical application.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"lingoreel/internal/buildinfo"
)

var errUnknownCommand = errors.New("comando desconhecido")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "spdev:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
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
		return fmt.Errorf("%w: %q", errUnknownCommand, args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Uso: spdev <comando>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Comandos:")
	fmt.Fprintln(w, "  version   mostra a versão desta compilação")
	fmt.Fprintln(w, "  help      mostra esta ajuda")
}
