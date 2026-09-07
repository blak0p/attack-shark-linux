package main

import (
	"context"
	"errors"
	"testing"
)

func TestRunCLIRoutesOnlyExactResetBeforeWails(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantExit      int
		wantWailsRuns int
		wantResetRuns int
	}{
		{name: "no arguments starts Wails", wantExit: 0, wantWailsRuns: 1},
		{name: "exact reset is headless", args: []string{"reset"}, wantExit: 0, wantResetRuns: 1},
		{name: "requirements text is rejected", args: []string{"requirements.txt"}, wantExit: 2},
		{name: "CMake file is rejected", args: []string{"CMakeLists.txt"}, wantExit: 2},
		{name: "executable markdown is rejected", args: []string{"notes.mdx"}, wantExit: 2},
		{name: "markdown is rejected", args: []string{"notes.md"}, wantExit: 2},
		{name: "shell readme is rejected", args: []string{"README.sh"}, wantExit: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISPLAY", "")
			wailsRuns := 0
			resetRuns := 0
			got := runCLI(tt.args, cliDependencies{
				startWails: func() error {
					wailsRuns++
					return nil
				},
				reset: func(context.Context) error {
					resetRuns++
					return nil
				},
			})
			if got != tt.wantExit || wailsRuns != tt.wantWailsRuns || resetRuns != tt.wantResetRuns {
				t.Fatalf("runCLI(%q) = exit %d, Wails %d, reset %d; want exit %d, Wails %d, reset %d", tt.args, got, wailsRuns, resetRuns, tt.wantExit, tt.wantWailsRuns, tt.wantResetRuns)
			}
		})
	}
}

func TestRunCLIMapsHeadlessAndWailsFailuresToExitOne(t *testing.T) {
	tests := []struct {
		name string
		args []string
		deps cliDependencies
	}{
		{
			name: "reset failure",
			args: []string{"reset"},
			deps: cliDependencies{reset: func(context.Context) error { return errors.New("reset failed") }},
		},
		{
			name: "Wails failure",
			deps: cliDependencies{startWails: func() error { return errors.New("Wails failed") }},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runCLI(tt.args, tt.deps); got != 1 {
				t.Fatalf("runCLI(%q) = %d, want 1", tt.args, got)
			}
		})
	}
}
