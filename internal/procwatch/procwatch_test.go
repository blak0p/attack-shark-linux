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
		if !got.Names[want] {
			t.Errorf("missing %q in %v", want, got.Names)
		}
	}
}

func setCwd(t *testing.T, root, pid, target string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(root, pid, "cwd")); err != nil {
		t.Fatal(err)
	}
}

func TestScanCollectsPaths(t *testing.T) {
	root := t.TempDir()
	addProc(t, root, "100", "game", "/games/Native/game.bin", "/games/Native/game.bin", "--data=/ignored/flag")
	addProc(t, root, "200", "wine64-preloader", "/usr/bin/wine64-preloader",
		`Z:\mnt\Juegos\Hades\Hades.exe`, "-windowed")
	setCwd(t, root, "200", "/mnt/Juegos/Hades")
	addProc(t, root, "300", "kate", "/usr/bin/kate", "kate", "/games/Doc/notes.txt")
	addProc(t, root, "400", "proton", "/usr/bin/python3", "python3", "/p/proton", "run", "/lib/Game2/Game2.EXE", `C:\prefix\Other.exe`, "relative/Thing.exe")
	got, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"/games/Native/game.bin":      true,
		"/usr/bin/wine64-preloader":   true,
		"/mnt/Juegos/Hades/Hades.exe": true,
		"/mnt/Juegos/Hades":           true,
		"/usr/bin/kate":               true,
		"/usr/bin/python3":            true,
		"/lib/Game2/Game2.EXE":        true,
	}
	have := map[string]bool{}
	for _, p := range got.Paths {
		have[p] = true
	}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("paths = %v, want %v", have, want)
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
	if !reflect.DeepEqual(got.Names, map[string]bool{"ok": true}) || len(got.Paths) != 0 {
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

func run(names ...string) Running {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return Running{Names: m}
}

func runIn(paths ...string) Running {
	return Running{Names: map[string]bool{}, Paths: paths}
}

var rules = []Rule{{Executable: "a.exe", ProfileID: "A"}, {Executable: "b.exe", ProfileID: "B"}}

func mustTick(t *testing.T, w *Watcher, m Running) {
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

func TestFolderRuleMatchesAnyPathInsideTheFolder(t *testing.T) {
	folder := "/mnt/Juegos/SteamLibrary/steamapps/common/Terraria"
	for name, path := range map[string]string{
		"file inside":   folder + "/Terraria.bin.x86_64",
		"nested file":   folder + "/Content/tool.exe",
		"folder itself": folder,
	} {
		sw := &fakeSwitcher{active: "base"}
		w := NewWatcher(sw, []Rule{{Folder: folder, ProfileID: "T"}})
		mustTick(t, w, runIn(path))
		if !reflect.DeepEqual(sw.calls, []string{"T"}) {
			t.Errorf("%s: calls %v", name, sw.calls)
		}
	}
}

func TestFolderRuleIgnoresSiblingsAndPrefixes(t *testing.T) {
	folder := "/games/Terraria"
	for _, path := range []string{"/games/Terraria2/x", "/games/TerrariaMod", "/games", "/other/games/Terraria/x"} {
		sw := &fakeSwitcher{active: "base"}
		w := NewWatcher(sw, []Rule{{Folder: folder + "/", ProfileID: "T"}})
		mustTick(t, w, runIn(path))
		if len(sw.calls) != 0 {
			t.Errorf("%s matched: calls %v", path, sw.calls)
		}
	}
}

func TestFolderRuleFollowsSymlinkedFolder(t *testing.T) {
	real := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, []Rule{{Folder: link, ProfileID: "T"}})
	mustTick(t, w, runIn(real+"/game"))
	mustTick(t, w, runIn(link+"/game"))
	if !reflect.DeepEqual(sw.calls, []string{"T"}) {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestFolderRuleRestoresWhenNothingRunsInsideAnymore(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, []Rule{{Folder: "/games/T", ProfileID: "T"}})
	mustTick(t, w, runIn("/games/T/x"))
	mustTick(t, w, runIn("/games/T/x"))
	mustTick(t, w, runIn("/elsewhere"))
	if !reflect.DeepEqual(sw.calls, []string{"T", "base"}) {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestFolderAndExecutableRulesShareOrderAndHandOver(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, []Rule{{Folder: "/games/T", ProfileID: "T"}, {Executable: "emu", ProfileID: "E"}})
	both := Running{Names: map[string]bool{"emu": true}, Paths: []string{"/games/T/x"}}
	mustTick(t, w, both)
	if !reflect.DeepEqual(sw.calls, []string{"T"}) {
		t.Fatalf("first rule must win: %v", sw.calls)
	}
	mustTick(t, w, run("emu"))
	mustTick(t, w, run())
	if !reflect.DeepEqual(sw.calls, []string{"T", "E", "base"}) {
		t.Fatalf("calls %v", sw.calls)
	}
}

func TestRuleWithoutExecutableOrFolderNeverMatches(t *testing.T) {
	sw := &fakeSwitcher{active: "base"}
	w := NewWatcher(sw, []Rule{{ProfileID: "X"}})
	mustTick(t, w, Running{Names: map[string]bool{"": true, "a": true}, Paths: []string{"/", "/a"}})
	if len(sw.calls) != 0 {
		t.Fatalf("calls %v", sw.calls)
	}
}
