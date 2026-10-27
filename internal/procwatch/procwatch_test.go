package procwatch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func addProc(t *testing.T, root, pid, comm, exe string, args ...string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if comm != "" {
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var cmd []byte
	for _, a := range args {
		cmd = append(cmd, a...)
		cmd = append(cmd, 0)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), cmd, 0o644); err != nil {
		t.Fatal(err)
	}
	if exe != "" {
		if err := os.Symlink(exe, filepath.Join(dir, "exe")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScanCollectsNames(t *testing.T) {
	root := t.TempDir()
	addProc(t, root, "100", "game", "/opt/games/game.bin", "/opt/games/game.bin", "--flag")
	addProc(t, root, "200", "wine64-preloader", "/usr/bin/wine64-preloader",
		"Z:\\games\\Hades\\Hades.exe", "-windowed")
	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"game", "game.bin", "--flag", "wine64-preloader", "Hades.exe", "-windowed"} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestScanSkipsBadEntries(t *testing.T) {
	root := t.TempDir()
	addProc(t, root, "1", "ok", "")
	// vanished/unreadable: pid dir without readable files
	if err := os.MkdirAll(filepath.Join(root, "2"), 0o755); err != nil {
		t.Fatal(err)
	}
	// non-numeric dir must be ignored
	addProc(t, root, "self", "ignored", "")
	addProc(t, root, "sys", "ignored2", "")
	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]bool{"ok": true}) {
		t.Fatalf("got %v", got)
	}
}

type fakeSwitcher struct {
	active string
	calls  []string
	err    error
}

func (f *fakeSwitcher) ActiveID() string { return f.active }
func (f *fakeSwitcher) Switch(id string) error {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return f.err
	}
	f.active = id
	return nil
}

func run(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

var rules = []Rule{{"a.exe", "A"}, {"b.exe", "B"}}

func mustTick(t *testing.T, w *Watcher, m map[string]bool) {
	t.Helper()
	if err := w.Tick(m); err != nil {
		t.Fatal(err)
	}
}

func TestApplyOnStartRestoreOnExit(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run())
	mustTick(t, w, run("a.exe"))
	mustTick(t, w, run("a.exe"))
	if !reflect.DeepEqual(sw.calls, []string{"A"}) {
		t.Fatalf("calls %v", sw.calls)
	}
	mustTick(t, w, run())
	if !reflect.DeepEqual(sw.calls, []string{"A", "base"}) || sw.active != "base" {
		t.Fatalf("calls %v", sw.calls)
	}
	mustTick(t, w, run())
	if len(sw.calls) != 2 {
		t.Fatalf("extra calls %v", sw.calls)
	}
}

func TestRuleOrderPriority(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run("b.exe", "a.exe"))
	if !reflect.DeepEqual(sw.calls, []string{"A"}) {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestHandOverWithoutRestore(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run("a.exe", "b.exe"))
	mustTick(t, w, run("b.exe"))
	if !reflect.DeepEqual(sw.calls, []string{"A", "B"}) {
		t.Fatalf("calls %v", sw.calls)
	}
	mustTick(t, w, run())
	if !reflect.DeepEqual(sw.calls, []string{"A", "B", "base"}) {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestNoRestoreWhenChangedByOthers(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run("a.exe"))
	sw.active = "user"
	mustTick(t, w, run())
	if !reflect.DeepEqual(sw.calls, []string{"A"}) || sw.active != "user" {
		t.Fatalf("calls %v active %s", sw.calls, sw.active)
	}
}

func TestSwitchErrorRetried(t *testing.T) {
	sw := &fakeSwitcher{active: "base", err: errors.New("boom")}
	w := NewWatcher(sw, rules)
	if err := w.Tick(run("a.exe")); err == nil {
		t.Fatal("expected error")
	}
	sw.err = nil
	mustTick(t, w, run("a.exe"))
	if sw.active != "A" {
		t.Fatalf("active %s", sw.active)
	}
	sw.err = errors.New("boom")
	if err := w.Tick(run()); err == nil {
		t.Fatal("expected restore error")
	}
	sw.err = nil
	mustTick(t, w, run())
	if sw.active != "base" {
		t.Fatalf("active %s", sw.active)
	}
}

func TestNoMatchNoApply(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run("other"))
	if len(sw.calls) != 0 {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestUnknownPreviousProfileIsNotRestored(t *testing.T) {
	sw := &fakeSwitcher{active: ""}
	w := NewWatcher(sw, rules)
	mustTick(t, w, run("a.exe"))
	mustTick(t, w, run())
	mustTick(t, w, run())
	if !reflect.DeepEqual(sw.calls, []string{"A"}) {
		t.Fatalf("calls %v, want only the apply: there is nothing known to restore", sw.calls)
	}
}
