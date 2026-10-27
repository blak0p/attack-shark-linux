package configstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const hexA = "0123456789abcdef0123456789abcdef"
const hexB = "fedcba9876543210fedcba9876543210"

func TestWatcherStoreMissingFileIsDefaults(t *testing.T) {
	store := NewWatcherStore(filepath.Join(t.TempDir(), "watcher-v1.json"))
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || len(got.Rules) != 0 {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestWatcherStoreSaveLoadRoundTripAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "watcher-v1.json")
	store := NewWatcherStore(path)
	want := WatcherSettings{Enabled: true, Rules: []WatcherRule{{Executable: "game.exe", ProfileID: hexA}, {Executable: "Other", ProfileID: hexB}}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("load = %#v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, %v", info, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestWatcherStoreSaveTrimsExecutable(t *testing.T) {
	store := NewWatcherStore(filepath.Join(t.TempDir(), "w.json"))
	if err := store.Save(WatcherSettings{Rules: []WatcherRule{{Executable: "  game  ", ProfileID: hexA}}}); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Load()
	if got.Rules[0].Executable != "game" {
		t.Fatalf("executable = %q", got.Rules[0].Executable)
	}
}

func TestWatcherStoreCorruptOrInvalidFile(t *testing.T) {
	for name, contents := range map[string]string{
		"garbage":  "{not json",
		"bad hex":  `{"enabled":true,"rules":[{"executable":"a","profile_id":"XYZ"}]}`,
		"path":     `{"enabled":true,"rules":[{"executable":"a/b","profile_id":"` + hexA + `"}]}`,
		"unknown":  `{"enabled":true,"extra":1}`,
		"trailing": `{"enabled":true} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "w.json")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := NewWatcherStore(path).Load()
			if !errors.Is(err, ErrInvalidWatcherSettings) {
				t.Fatalf("err = %v", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != contents {
				t.Fatal("load modified file")
			}
		})
	}
}

func TestWatcherStoreRejectsInvalidAndLeavesFileUntouched(t *testing.T) {
	many := make([]WatcherRule, 65)
	for i := range many {
		many[i] = WatcherRule{Executable: strings.Repeat("x", i+1), ProfileID: hexA}
	}
	cases := map[string][]WatcherRule{
		"empty":       {{Executable: "", ProfileID: hexA}},
		"blank":       {{Executable: "   ", ProfileID: hexA}},
		"slash":       {{Executable: "a/b", ProfileID: hexA}},
		"backslash":   {{Executable: `a\b`, ProfileID: hexA}},
		"nul":         {{Executable: "a\x00b", ProfileID: hexA}},
		"too long":    {{Executable: strings.Repeat("a", 256), ProfileID: hexA}},
		"duplicate":   {{Executable: "a", ProfileID: hexA}, {Executable: "a", ProfileID: hexB}},
		"dup trimmed": {{Executable: "a", ProfileID: hexA}, {Executable: " a", ProfileID: hexB}},
		"short id":    {{Executable: "a", ProfileID: "abc"}},
		"upper id":    {{Executable: "a", ProfileID: strings.ToUpper(hexA)}},
		"non-hex id":  {{Executable: "a", ProfileID: strings.Repeat("g", 32)}},
		"65 rules":    many,
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "w.json")
			store := NewWatcherStore(path)
			if err := store.Save(WatcherSettings{Enabled: true, Rules: []WatcherRule{{Executable: "keep", ProfileID: hexA}}}); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			err := store.Save(WatcherSettings{Enabled: false, Rules: rules})
			if !errors.Is(err, ErrInvalidWatcherSettings) {
				t.Fatalf("err = %v", err)
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("file changed on rejection")
			}
		})
	}
}

func TestWatcherStoreAcceptsBoundaries(t *testing.T) {
	rules := make([]WatcherRule, 64)
	for i := range rules {
		rules[i] = WatcherRule{Executable: strings.Repeat("x", i+1), ProfileID: hexA}
	}
	rules[63].Executable = strings.Repeat("y", 255)
	if err := NewWatcherStore(filepath.Join(t.TempDir(), "w.json")).Save(WatcherSettings{Rules: rules}); err != nil {
		t.Fatal(err)
	}
}

func TestWatcherStoreSaveIOFailureLeavesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.json")
	store := NewWatcherStore(path)
	if err := store.Save(WatcherSettings{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)
	err := store.Save(WatcherSettings{Enabled: false})
	if err == nil || errors.Is(err, ErrInvalidWatcherSettings) {
		t.Skipf("directory permissions not enforced: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed on I/O failure")
	}
}
