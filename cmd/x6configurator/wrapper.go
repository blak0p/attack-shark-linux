package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/applock"
	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
)

const (
	// Shell conventions: 127 for a command that does not exist, 126 for one
	// that exists but cannot be started.
	exitCommandNotFound  = 127
	exitCommandCannotRun = 126

	// wrapperSwitchTimeout bounds every mouse operation so a stuck dongle can
	// never hold the game back.
	wrapperSwitchTimeout = 8 * time.Second
)

// profileSwitcher is the slice of desktop.Service the wrapper needs.
type profileSwitcher interface {
	RefreshInventory(ctx context.Context) desktop.Inventory
	GetProfiles() desktop.ProfileSnapshot
	SwitchProfile(id string) desktop.ProfileSnapshot
}

type wrapperDependencies struct {
	lockHeld    func() (bool, error)
	newSwitcher func() (profileSwitcher, error)
	timeout     time.Duration
	stderr      io.Writer
	// signals receives the signals to forward to the command; nil means
	// SIGINT and SIGTERM delivered to this process.
	signals <-chan os.Signal
}

// launchWithProfile is the production wiring of runWrapper.
func launchWithProfile(ref string, command []string) int {
	configDir, configErr := os.UserConfigDir()
	dataDir := filepath.Join(configDir, "attack-shark-linux")
	return runWrapper(ref, command, wrapperDependencies{
		lockHeld: func() (bool, error) {
			if configErr != nil {
				return false, configErr
			}
			return applock.IsHeld(appLockPath(dataDir))
		},
		newSwitcher: func() (profileSwitcher, error) {
			if configErr != nil {
				return nil, configErr
			}
			service, _ := composeDesktopService(dataDir, nil)
			return service, nil
		},
		timeout: wrapperSwitchTimeout,
		stderr:  os.Stderr,
	})
}

// runWrapper applies the named profile, runs the command, restores the previous
// profile and returns the command's exit code. Profile problems only warn: the
// command always runs.
func runWrapper(ref string, command []string, deps wrapperDependencies) int {
	warn := func(format string, args ...any) {
		fmt.Fprintf(deps.stderr, "attack-shark-linux: warning: "+format+"\n", args...)
	}

	var restore func()
	held, err := deps.lockHeld()
	switch {
	case err != nil:
		warn("cannot check whether the app is running, leaving the mouse alone: %v", err)
	case held:
		// The app owns the mouse while it is open.
	default:
		restore = applyProfile(ref, deps, warn)
	}

	code := runCommand(command, deps)
	if restore != nil {
		restore()
	}
	return code
}

// applyProfile switches to the profile and returns the restore step, or nil
// when nothing was (or may have been) changed.
func applyProfile(ref string, deps wrapperDependencies, warn func(string, ...any)) func() {
	switcher, err := deps.newSwitcher()
	if err != nil {
		warn("profile %q not applied: %v", ref, err)
		return nil
	}
	if _, ok := withTimeout(deps.timeout, func() desktop.Inventory {
		ctx, cancel := context.WithTimeout(context.Background(), deps.timeout)
		defer cancel()
		return switcher.RefreshInventory(ctx)
	}); !ok {
		warn("profile %q not applied: mouse detection timed out", ref)
		return nil
	}
	snapshot, ok := withTimeout(deps.timeout, switcher.GetProfiles)
	if !ok {
		warn("profile %q not applied: reading profiles timed out", ref)
		return nil
	}
	if snapshot.Error.Code != "" {
		warn("profile %q not applied: profiles unavailable (%s)", ref, snapshot.Error.Code)
		return nil
	}
	summaries := make([]configstore.ProfileSummary, len(snapshot.Profiles))
	for i, p := range snapshot.Profiles {
		summaries[i] = configstore.ProfileSummary{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt}
	}
	id, err := configstore.ResolveProfileRef(summaries, ref)
	if err != nil {
		warn("profile %q not applied: %v", ref, err)
		return nil
	}

	previous := snapshot.ActiveID
	result, ok := withTimeout(deps.timeout, func() desktop.ProfileSnapshot { return switcher.SwitchProfile(id) })
	switch {
	case !ok:
		warn("profile %q: switch timed out", ref)
	case result.Error.Code != "":
		warn("profile %q: switch failed (%s)", ref, result.Error.Code)
		return nil
	}
	if previous == "" {
		return nil
	}
	return func() {
		result, ok := withTimeout(deps.timeout, func() desktop.ProfileSnapshot { return switcher.SwitchProfile(previous) })
		switch {
		case !ok:
			warn("restoring the previous profile timed out")
		case result.Error.Code != "":
			warn("restoring the previous profile failed (%s)", result.Error.Code)
		}
	}
}

// withTimeout runs f in a goroutine and gives up after d. The abandoned call
// keeps running; the process exits soon after, so that is acceptable.
func withTimeout[T any](d time.Duration, f func() T) (T, bool) {
	done := make(chan T, 1)
	go func() { done <- f() }()
	select {
	case v := <-done:
		return v, true
	case <-time.After(d):
		var zero T
		return zero, false
	}
}

// runCommand starts the command with inherited stdio, forwards signals to it and
// returns its exit code (128+n when a signal killed it).
func runCommand(command []string, deps wrapperDependencies) int {
	signals := deps.signals
	if signals == nil {
		real := make(chan os.Signal, 4)
		signal.Notify(real, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(real)
		signals = real
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(deps.stderr, "attack-shark-linux: cannot run %q: %v\n", command[0], err)
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return exitCommandNotFound
		}
		return exitCommandCannotRun
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()

	err := cmd.Wait()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	}
	fmt.Fprintf(deps.stderr, "attack-shark-linux: waiting for %q: %v\n", command[0], err)
	return exitFailure
}
