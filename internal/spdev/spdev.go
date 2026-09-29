// Package spdev implements the commands of the spdev developer tool, which
// exercises LingoReel subsystems without the graphical application.
package spdev

import (
	"context"
	"errors"
	"fmt"
	"io"

	"lingoreel/internal/buildinfo"
)

// ErrUnknownCommand reports a command name that spdev does not know.
var ErrUnknownCommand = errors.New("comando desconhecido")

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
		return inspect(args[1:], stdout)
	case "play":
		return play(ctx, args[1:], stdout, false)
	case "overlay":
		return play(ctx, args[1:], stdout, true)
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
	fmt.Fprintln(w, "  version                        mostra a versão desta compilação")
	fmt.Fprintln(w, "  inspect <vídeo> [--legendas]   mostra as faixas e a sugestão de estudo")
	fmt.Fprintln(w, "  play <vídeo> [opções]          abre o vídeo no player e mostra as legendas no tempo certo")
	fmt.Fprintln(w, "      --player <mpv|vlc|potplayer>   player preferido (padrão: o melhor instalado)")
	fmt.Fprintln(w, "      --inicio <segundos>            começa a partir desta posição")
	fmt.Fprintln(w, "      --pausado                      abre pausado")
	fmt.Fprintln(w, "  overlay <vídeo> [opções]       como play, desenhando as legendas sobre o vídeo")
	fmt.Fprintln(w, "  help                           mostra esta ajuda")
}
