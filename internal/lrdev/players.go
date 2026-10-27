package lrdev

import (
	"log/slog"
	"os"
	"strings"

	"github.com/ediejohnny/lingoreel/internal/application/players"
)

// stderrLogger writes the events that have no caller to the console.
func stderrLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// supportedNames lists the players with a plugin, as "VLC, mpv".
func supportedNames(registry *players.Registry) string {
	var names []string
	for _, descriptor := range registry.Descriptors() {
		names = append(names, descriptor.Name)
	}
	return strings.Join(names, ", ")
}
