package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"lingoreel/internal/buildinfo"
)

// The version is swapped for a distinctive value so the test fails if run
// prints a constant instead of the value injected at build time.
func TestRunVersionPrintsInjectedBuildVersion(t *testing.T) {
	original := buildinfo.Version
	buildinfo.Version = "7.8.9-injected"
	t.Cleanup(func() { buildinfo.Version = original })
	var out bytes.Buffer

	if err := run([]string{"version"}, &out); err != nil {
		t.Fatalf("run(version) returned error: %v", err)
	}
	if got := out.String(); got != "7.8.9-injected\n" {
		t.Fatalf("run(version) printed %q, want %q", got, "7.8.9-injected\n")
	}
}

func TestRunWithoutArgumentsPrintsUsage(t *testing.T) {
	var out bytes.Buffer

	if err := run(nil, &out); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Uso: spdev") {
		t.Fatalf("usage not printed, got %q", out.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var out bytes.Buffer

	err := run([]string{"bogus"}, &out)
	if !errors.Is(err, errUnknownCommand) {
		t.Fatalf("run(bogus) error = %v, want errUnknownCommand", err)
	}
	if !strings.Contains(out.String(), "Uso: spdev") {
		t.Fatalf("usage not printed for unknown command, got %q", out.String())
	}
}
