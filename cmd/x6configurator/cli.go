package main

import (
	"context"
	"errors"
	"io"
	"os"
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
	launch     func(profileRef string, command []string) int
}

const usageText = `Usage:
  attack-shark-linux                         open the desktop app
  attack-shark-linux reset                   restore the factory configuration
  attack-shark-linux --profile NAME -- CMD   apply profile NAME (name or ID), run CMD,
                                             then restore the previous profile

Steam launch option: attack-shark-linux --profile "NAME" -- %command%
`

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
	case len(args) > 0 && args[0] == "--profile":
		// --profile NAME -- COMMAND [ARGS...]
		if len(args) < 4 || args[1] == "" || args[2] != "--" || dependencies.launch == nil {
			io.WriteString(os.Stderr, usageText)
			return exitUsage
		}
		return dependencies.launch(args[1], args[3:])
	default:
		return exitUsage
	}
}
