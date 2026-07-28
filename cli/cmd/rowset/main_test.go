package main

import (
	"bytes"
	"context"
	"strings"
	"syscall"
	"testing"

	"github.com/LVTD-LLC/rowset/cli/internal/rowsetcli"
)

type pipeWriter struct{}

func (pipeWriter) Write(_ []byte) (int, error) {
	return 0, syscall.EPIPE
}

func TestRunMapsUsageErrorsAtTheExecutableBoundary(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := run(context.Background(), rowsetcli.IO{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	}, []string{"unknown-command"})

	if exitCode != rowsetcli.ExitUsage {
		t.Fatalf("exit code mismatch: got %d want %d", exitCode, rowsetcli.ExitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr missing diagnostic: %q", stderr.String())
	}
}

func TestRunTreatsBrokenPipeAsNormalTermination(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := run(context.Background(), rowsetcli.IO{
		Stdin:  strings.NewReader(""),
		Stdout: pipeWriter{},
		Stderr: &stderr,
	}, []string{"--version"})

	if exitCode != rowsetcli.ExitSuccess {
		t.Fatalf("exit code mismatch: got %d want %d", exitCode, rowsetcli.ExitSuccess)
	}
	if stderr.Len() != 0 {
		t.Fatalf("broken pipe should be silent, got %q", stderr.String())
	}
}
