// Package procwatch detects running games from /proc and drives profile
// switches through an injected Switcher. It never touches hardware itself.
package procwatch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Scan returns the set of executable names running under procRoot: each
// pid's comm, the basename of its exe link target and the basename of every
// cmdline argument (split on / and \ so Wine/Proton paths count).
// Unreadable or vanished pids and non-numeric entries are skipped.
func Scan(procRoot string) (map[string]bool, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		dir := filepath.Join(procRoot, e.Name())
		if comm, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
			add(names, strings.TrimRight(string(comm), "\n"))
		}
		if exe, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
			add(names, baseName(exe))
		}
		if cmd, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			for _, arg := range bytes.Split(cmd, []byte{0}) {
				add(names, baseName(string(arg)))
			}
		}
	}
	return names, nil
}

func add(names map[string]bool, name string) {
	if name != "" {
		names[name] = true
	}
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Rule maps an executable name (exact, case-sensitive) to a profile ID.
type Rule struct {
	Executable string
	ProfileID  string
}

// Switcher reads and changes the active profile.
type Switcher interface {
	ActiveID() string
	Switch(id string) error
}

// Watcher is a tick-driven state machine; it starts no goroutines.
type Watcher struct {
	sw    Switcher
	rules []Rule

	active   bool
	exe      string // executable that triggered the current profile
	setID    string // profile the watcher last switched to
	previous string // profile to restore when no game is left
}

func NewWatcher(sw Switcher, rules []Rule) *Watcher {
	return &Watcher{sw: sw, rules: append([]Rule(nil), rules...)}
}

func (w *Watcher) match(running map[string]bool) (Rule, bool) {
	for _, r := range w.rules {
		if running[r.Executable] {
			return r, true
		}
	}
	return Rule{}, false
}

// Tick advances the state machine with the currently running names. A Switch
// error leaves the state unchanged, so the next tick retries.
func (w *Watcher) Tick(running map[string]bool) error {
	if w.active {
		if running[w.exe] {
			return nil
		}
		if w.sw.ActiveID() != w.setID {
			// Someone else changed the profile: forget our state, no restore.
			w.active = false
		} else if r, ok := w.match(running); ok {
			if err := w.sw.Switch(r.ProfileID); err != nil {
				return err
			}
			w.exe, w.setID = r.Executable, r.ProfileID
			return nil
		} else {
			// An unknown previous profile (it could not be read) cannot be restored.
			if w.previous != "" {
				if err := w.sw.Switch(w.previous); err != nil {
					return err
				}
			}
			w.active = false
			return nil
		}
	}
	r, ok := w.match(running)
	if !ok {
		return nil
	}
	prev := w.sw.ActiveID()
	if err := w.sw.Switch(r.ProfileID); err != nil {
		return err
	}
	w.active, w.exe, w.setID, w.previous = true, r.Executable, r.ProfileID, prev
	return nil
}

// Run scans procRoot every interval and ticks the watcher until ctx is done.
// Scan and Switch errors are passed to onErr (may be nil) and retried.
func Run(ctx context.Context, w *Watcher, procRoot string, interval time.Duration, onErr func(error)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		running, err := Scan(procRoot)
		if err == nil {
			err = w.Tick(running)
		}
		if err != nil && onErr != nil {
			onErr(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
