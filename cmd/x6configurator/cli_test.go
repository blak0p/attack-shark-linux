package main

import (
	"context"
	"errors"
	"reflect"
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

func TestRunCLIRoutesProfileWrapper(t *testing.T) {
	var gotRef string
	var gotCommand []string
	deps := cliDependencies{launch: func(ref string, command []string) int {
		gotRef, gotCommand = ref, command
		return 7
	}}

	if got := runCLI([]string{"--profile", "Game", "--", "game", "--fullscreen"}, deps); got != 7 {
		t.Fatalf("exit = %d, want the wrapper's 7", got)
	}
	if gotRef != "Game" || !reflect.DeepEqual(gotCommand, []string{"game", "--fullscreen"}) {
		t.Fatalf("launch got %q %q", gotRef, gotCommand)
	}
}

func TestRunCLIRejectsMalformedProfileWrapper(t *testing.T) {
	for _, args := range [][]string{
		{"--profile"},
		{"--profile", "Game"},
		{"--profile", "Game", "--"},
		{"--profile", "Game", "game"},
		{"--profile", "", "--", "game"},
	} {
		deps := cliDependencies{launch: func(string, []string) int {
			t.Fatalf("launch must not run for %q", args)
			return 0
		}}
		if got := runCLI(args, deps); got != exitUsage {
			t.Errorf("runCLI(%q) = %d, want %d", args, got, exitUsage)
		}
	}
}

func TestRunCLIPriorCommandsIgnoreWrapper(t *testing.T) {
	launches := 0
	deps := cliDependencies{
		startWails: func() error { return nil },
		reset:      func(context.Context) error { return nil },
		launch:     func(string, []string) int { launches++; return 0 },
	}
	if runCLI(nil, deps) != exitSuccess || runCLI([]string{"reset"}, deps) != exitSuccess || launches != 0 {
		t.Fatalf("no-arg and reset must keep working without the wrapper (launches=%d)", launches)
	}
}

func TestCLIProfileWithoutLauncherIsAFailureNotUsage(t *testing.T) {
	if got := runCLI([]string{"--profile", "Game", "--", "true"}, cliDependencies{}); got != exitFailure {
		t.Fatalf("exit = %d, want %d", got, exitFailure)
	}
}
