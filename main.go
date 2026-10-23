package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/guness/bitrise-step-firebase-app-distribution/internal/step"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config, err := step.ReadConfig(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}
	if err := step.NewRunner(os.Stdout).Run(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return step.ExitCode(err)
	}
	return 0
}
