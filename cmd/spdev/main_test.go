package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"lingoreel/internal/buildinfo"
)

func TestRunVersionPrintsBuildVersion(t *testing.T) {
	var out bytes.Buffer

	if err := run([]string{"version"}, &out); err != nil {
		t.Fatalf("run(version) returned error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != buildinfo.Version {
		t.Fatalf("run(version) printed %q, want %q", got, buildinfo.Version)
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
