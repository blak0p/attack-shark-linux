package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/applock"
	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
	"github.com/blak0p/attack-shark-linux/internal/hidlinux"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

const (
	// Shell conventions: 127 for a command that does not exist, 126 for one
	// that exists but cannot be started.
	exitCommandNotFound  = 127
	exitCommandCannotRun = 126

	// wrapperSwitchTimeout is the total time the wrapper may spend on mouse work
	// before launching the command (detection, reading profiles and the switch
	// together), and separately the time the restore after the command may take.
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

// quietDiagnostics silences the device diagnostics and structured logs so the
// wrapper does not flood the launched command's terminal; the wrapper reports
// its own warnings. The returned function restores the previous state.
func quietDiagnostics() (restore func()) {
	previous := slog.Default()
	restoreMouse := mouse.SetInventoryDiagnosticWriter(io.Discard)
	restoreHID := hidlinux.SetInventoryDiagnosticWriter(io.Discard)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.Level(math.MaxInt32)})))
	return func() {
		slog.SetDefault(previous)
		restoreHID()
		restoreMouse()
	}
}

// launchWithProfile is the production wiring of runWrapper.
func launchWithProfile(ref string, command []string) int {
	defer quietDiagnostics()()
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
			// The second result is the status reader, which the wrapper does not need.
			service, _ := composeDesktopService(dataDir, nil)
			return service, nil
		},
		timeout: wrapperSwitchTimeout,
		stderr:  os.Stderr,
	})
}

// runWrapper applies the named profile, runs the command, restores the previous
// profile and returns the command's exit code. Profile problems only warn: the
// command always runs. SIGTERM or SIGHUP received before the command starts
// abort the launch; the profile is put back and the exit code is 128+n.
func runWrapper(ref string, command []string, deps wrapperDependencies) int {
	warn := func(format string, args ...any) {
		fmt.Fprintf(deps.stderr, "attack-shark-linux: warning: "+format+"\n", args...)
	}

	signals := deps.signals
	if signals == nil {
		notified := make(chan os.Signal, 4)
		signal.Notify(notified, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(notified)
		signals = notified
	}

	// The check below and the app opening right after it can race: the app does
	// not refuse to start when the lock is taken, so a very narrow window remains
	// in which both write to the mouse. That is accepted; the lock is advisory.
	prepared := make(chan func(), 1)
	go func() {
		held, err := deps.lockHeld()
		switch {
		case err != nil:
			warn("cannot check whether the app is running, leaving the mouse alone: %v", err)
		case held:
			// The app owns the mouse while it is open.
		default:
			prepared <- applyProfile(ref, deps, warn)
			return
		}
		prepared <- nil
	}()

	var restore func()
	select {
	case restore = <-prepared:
	case sig := <-signals:
		// Mouse work is bounded by the timeout, so waiting for it is short.
		if restore = <-prepared; restore != nil {
			restore()
		}
		return exitForSignal(sig)
	}

	code := runCommand(command, deps, signals)
	if restore != nil {
		restore()
	}
	return code
}

func exitForSignal(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return exitFailure
}

// applyProfile switches to the profile and returns the restore step, or nil
// when nothing was (or may have been) changed. All the work before the switch
// shares one deadline of deps.timeout. A switch that failed with apply_failed or
// timed out may have written part of the profile, so those still return a
// restore step; other failures wrote nothing.
func applyProfile(ref string, deps wrapperDependencies, warn func(string, ...any)) func() {
	deadline := time.Now().Add(deps.timeout)
	left := func() time.Duration { return max(time.Until(deadline), 0) }

	switcher, err := deps.newSwitcher()
	if err != nil {
		warn("profile %q not applied: %v", ref, err)
		return nil
	}
	if _, ok, _ := bounded(left(), func() desktop.Inventory {
		ctx, cancel := context.WithTimeout(context.Background(), deps.timeout)
		defer cancel()
		return switcher.RefreshInventory(ctx)
	}); !ok {
		warn("profile %q not applied: mouse detection timed out", ref)
		return nil
	}
	snapshot, ok, _ := bounded(left(), switcher.GetProfiles)
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
	result, ok, switchFinished := bounded(left(), func() desktop.ProfileSnapshot { return switcher.SwitchProfile(id) })
	switch {
	case !ok:
		warn("profile %q: switch timed out", ref)
	case result.Error.Code != "":
		warn("profile %q: switch failed (%s)", ref, result.Error.Code)
		if result.Error.Code != desktop.ApplyFailed {
			return nil
		}
	}
	if previous == "" {
		return nil
	}
	return func() {
		// An abandoned switch is still writing to the mouse; wait for it rather
		// than overlap a second write.
		select {
		case <-switchFinished:
		case <-time.After(deps.timeout):
			warn("restoring the previous profile skipped: the earlier switch is still writing to the mouse")
			return
		}
		// The app may have been opened during the game and owns the mouse now.
		if held, err := deps.lockHeld(); err != nil {
			warn("restoring the previous profile skipped: cannot check whether the app is running: %v", err)
			return
		} else if held {
			return
		}
		result, ok, _ := bounded(deps.timeout, func() desktop.ProfileSnapshot { return switcher.SwitchProfile(previous) })
		switch {
		case !ok:
			warn("restoring the previous profile timed out")
		case result.Error.Code != "":
			warn("restoring the previous profile failed (%s)", result.Error.Code)
		}
	}
}

// bounded runs f in a goroutine and gives up after d. An abandoned call keeps
// running; finished is closed when f returns so callers can wait for it before
// touching the same device again.
func bounded[T any](d time.Duration, f func() T) (value T, ok bool, finished <-chan struct{}) {
	done := make(chan T, 1)
	over := make(chan struct{})
	go func() {
		v := f()
		done <- v
		close(over)
	}()
	select {
	case v := <-done:
		return v, true, over
	case <-time.After(d):
		return value, false, over
	}
}

// runCommand starts the command with inherited stdio, forwards SIGTERM and SIGHUP
// to it and returns its exit code (128+n when a signal killed it). SIGINT is not
// forwarded: Ctrl-C reaches the command through the terminal's process group, so
// forwarding it as well would interrupt the command twice.
func runCommand(command []string, deps wrapperDependencies, signals <-chan os.Signal) int {
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
				if sig != syscall.SIGINT {
					_ = cmd.Process.Signal(sig)
				}
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
