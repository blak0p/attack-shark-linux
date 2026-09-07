package main

import (
	"context"
	"errors"
)

const (
	exitSuccess = iota
	exitFailure
	exitUsage
)

var errEmergencyResetUnavailable = errors.New("emergency reset is not available")

type cliDependencies struct {
	startWails func() error
	reset      func(context.Context) error
}

func runCLI(args []string, dependencies cliDependencies) int {
	switch {
	case len(args) == 0:
		if dependencies.startWails == nil || dependencies.startWails() != nil {
			return exitFailure
		}
		return exitSuccess
	case len(args) == 1 && args[0] == "reset":
		if dependencies.reset == nil || dependencies.reset(context.Background()) != nil {
			return exitFailure
		}
		return exitSuccess
	default:
		return exitUsage
	}
}
