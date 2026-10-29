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

// Running is what Scan observed: executable names and absolute paths.
type Running struct {
	Names map[string]bool
	Paths []string
}

// Scan returns the names and paths running under procRoot. Names: each pid's
// comm, the basename of its exe link target and the basename of every cmdline
// argument (split on / and \ so Wine/Proton paths count). Paths: the exe and
// cwd link targets, plus absolute cmdline arguments that are argument 0 or end
// in .exe; Wine drive-Z arguments are converted to Unix paths, other drives
// and relative arguments are ignored. Unreadable or vanished pids and
// non-numeric entries are skipped.
func Scan(procRoot string) (Running, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return Running{}, err
	}
	names := map[string]bool{}
	var paths []string
	addPath := func(p string) {
		if p != "" {
			paths = append(paths, filepath.Clean(p))
		}
	}
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
			addPath(exe)
		}
		if cwd, err := os.Readlink(filepath.Join(dir, "cwd")); err == nil {
			addPath(cwd)
		}
		if cmd, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			for i, arg := range bytes.Split(cmd, []byte{0}) {
				add(names, baseName(string(arg)))
				if p, ok := argPath(string(arg)); ok &&
					(i == 0 || strings.HasSuffix(strings.ToLower(p), ".exe")) {
					addPath(p)
				}
			}
		}
	}
	return Running{Names: names, Paths: paths}, nil
}

// argPath returns the absolute Unix path an argument denotes, if any.
func argPath(arg string) (string, bool) {
	if len(arg) >= 3 && (arg[0] == 'z' || arg[0] == 'Z') && arg[1] == ':' && (arg[2] == '\\' || arg[2] == '/') {
		return strings.ReplaceAll(arg[2:], `\`, "/"), true
	}
	if strings.HasPrefix(arg, "/") {
		return arg, true
	}
	return "", false
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

// Rule maps a game folder, or an executable name when Folder is empty (exact,
// case-sensitive), to a profile ID. A rule with neither never matches.
type Rule struct {
	Executable string
	Folder     string
	ProfileID  string
}

// compiledRule is a Rule with its folder precomputed (cleaned, plus the
// symlink-resolved form when it differs).
type compiledRule struct {
	Rule
	dirs []string
}

func compile(r Rule) compiledRule {
	c := compiledRule{Rule: r}
	if r.Folder == "" {
		return c
	}
	clean := filepath.Clean(r.Folder)
	c.dirs = []string{clean}
	if real, err := filepath.EvalSymlinks(clean); err == nil && real != clean {
		c.dirs = append(c.dirs, real)
	}
	return c
}

func (c compiledRule) matches(run Running) bool {
	if c.Folder != "" {
		for _, d := range c.dirs {
			prefix := strings.TrimSuffix(d, "/") + "/"
			for _, p := range run.Paths {
				if p == d || strings.HasPrefix(p, prefix) {
					return true
				}
			}
		}
		return false
	}
	return c.Executable != "" && run.Names[c.Executable]
}

// Switcher reads and changes the active profile.
type Switcher interface {
	ActiveID() string
	Switch(id string) error
}

// Watcher is a tick-driven state machine; it starts no goroutines.
type Watcher struct {
	sw    Switcher
	rules []compiledRule

	active   bool
	current  compiledRule // rule that triggered the current profile
	setID    string       // profile the watcher last switched to
	previous string       // profile to restore when no game is left
}

func NewWatcher(sw Switcher, rules []Rule) *Watcher {
	w := &Watcher{sw: sw}
	for _, r := range rules {
		w.rules = append(w.rules, compile(r))
	}
	return w
}

func (w *Watcher) match(running Running) (compiledRule, bool) {
	for _, r := range w.rules {
		if r.matches(running) {
			return r, true
		}
	}
	return compiledRule{}, false
}

// Tick advances the state machine with the currently running names. A Switch
// error leaves the state unchanged, so the next tick retries.
func (w *Watcher) Tick(running Running) error {
	if w.active {
		if w.current.matches(running) {
			return nil
		}
		if w.sw.ActiveID() != w.setID {
			// Someone else changed the profile: forget our state, no restore.
			w.active = false
		} else if r, ok := w.match(running); ok {
			if err := w.sw.Switch(r.ProfileID); err != nil {
				return err
			}
			w.current, w.setID = r, r.ProfileID
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
	w.active, w.current, w.setID, w.previous = true, r, r.ProfileID, prev
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
