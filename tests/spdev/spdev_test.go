package spdev_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/buildinfo"
	"github.com/ediejohnny/lingoreel/internal/spdev"
)

// The version is swapped for a distinctive value so the test fails if Run
// prints a constant instead of the value injected at build time.
func TestRunVersionPrintsInjectedBuildVersion(t *testing.T) {
	original := buildinfo.Version
	buildinfo.Version = "7.8.9-injected"
	t.Cleanup(func() { buildinfo.Version = original })
	var out bytes.Buffer

	if err := spdev.Run(t.Context(), []string{"version"}, &out); err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}
	if got := out.String(); got != "7.8.9-injected\n" {
		t.Fatalf("Run(version) printed %q, want %q", got, "7.8.9-injected\n")
	}
}

// diag lists one line per monitor and the checklist for other machines.
func TestDiagListsMonitorsAndTheChecklist(t *testing.T) {
	var out bytes.Buffer
	if err := spdev.Run(t.Context(), []string{"diag"}, &out); err != nil {
		t.Fatalf("Run(diag): %v", err)
	}
	text := out.String()
	for _, want := range []string{"Monitors:", "(primary)", "DPI)", "work area", "Per-monitor DPI aware:", "Checklist"} {
		if !strings.Contains(text, want) {
			t.Fatalf("diag output lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "%!") {
		t.Fatalf("diag output has a formatting error:\n%s", text)
	}
}

func TestRunWithoutArgumentsOrWithHelpPrintsUsage(t *testing.T) {
	for _, args := range [][]string{nil, {}, {"help"}, {"-h"}, {"--help"}} {
		var out bytes.Buffer
		if err := spdev.Run(t.Context(), args, &out); err != nil {
			t.Fatalf("Run(%q) returned error: %v", args, err)
		}
		if !strings.Contains(out.String(), "Usage: spdev") || !strings.Contains(out.String(), "Commands:") {
			t.Fatalf("Run(%q) did not print the usage, got %q", args, out.String())
		}
	}
}

func TestRunRejectsUnknownCommands(t *testing.T) {
	for _, command := range []string{"bogus", "", "VERSION", "version2"} {
		var out bytes.Buffer
		err := spdev.Run(t.Context(), []string{command}, &out)
		if !errors.Is(err, spdev.ErrUnknownCommand) {
			t.Fatalf("Run(%q) error = %v, want ErrUnknownCommand", command, err)
		}
		if !strings.Contains(out.String(), "Usage: spdev") {
			t.Fatalf("usage not printed for %q, got %q", command, out.String())
		}
	}
}
