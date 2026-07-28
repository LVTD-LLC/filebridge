package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LVTD-LLC/rowset/cli/internal/rowsetcli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := run(ctx, rowsetcli.IO{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}, os.Args[1:])
	if exitCode != rowsetcli.ExitSuccess {
		os.Exit(exitCode)
	}
}

func run(ctx context.Context, streams rowsetcli.IO, args []string) int {
	err := rowsetcli.Run(ctx, streams, args)
	exitCode := rowsetcli.ExitCode(err)
	if err != nil && exitCode != rowsetcli.ExitSuccess {
		_, _ = fmt.Fprintln(streams.Stderr, err)
	}
	return exitCode
}
