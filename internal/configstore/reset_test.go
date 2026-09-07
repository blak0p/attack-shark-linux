package configstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStatePurgerRemovesCurrentAndLegacyStateAtomically(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "attack-shark-linux")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"devices-v2.json", "applied-dpi.json", "factory-defaults.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewStatePurger(dir).PurgeAll(); err != nil {
		t.Fatalf("PurgeAll() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("state directory = %v, %v; want empty recreated directory", entries, err)
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("state mode = %v, %v; want 0700", info.Mode(), err)
	}
	if _, err := os.Stat(dir + ".reset-quarantine"); !os.IsNotExist(err) {
		t.Fatalf("quarantine remains: %v", err)
	}
}

func TestStatePurgerRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "state")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := NewStatePurger(link).PurgeAll(); err == nil {
		t.Fatal("PurgeAll() error = nil; want symlink refusal")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target was altered: %v", err)
	}
}

func TestStatePurgerRestoresRecoveryStateAfterCleanupFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(statePurgeOps, string) statePurgeOps
	}{
		{
			name: "recreate",
			mutate: func(ops statePurgeOps, _ string) statePurgeOps {
				ops.mkdir = func(string, os.FileMode) error { return errors.New("recreate failed") }
				return ops
			},
		},
		{
			name: "sync",
			mutate: func(ops statePurgeOps, _ string) statePurgeOps {
				ops.open = func(string) (syncCloser, error) { return failingSyncCloser{}, nil }
				return ops
			},
		},
		{
			name: "remove",
			mutate: func(ops statePurgeOps, quarantine string) statePurgeOps {
				removeAll := ops.removeAll
				ops.removeAll = func(path string) error {
					if path == quarantine {
						return errors.New("remove failed")
					}
					return removeAll(path)
				}
				return ops
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			stateFile := filepath.Join(dir, "devices-v2.json")
			if err := os.WriteFile(stateFile, []byte("recovery"), 0o600); err != nil {
				t.Fatal(err)
			}

			quarantine := dir + ".reset-quarantine"
			ops := tc.mutate(defaultStatePurgeOps(), quarantine)
			if err := newStatePurgerWithOps(dir, ops).PurgeAll(); err == nil {
				t.Fatal("PurgeAll() error = nil, want cleanup failure")
			}
			contents, err := os.ReadFile(stateFile)
			if err != nil || string(contents) != "recovery" {
				t.Fatalf("recovery state = %q, %v; want original addressable state", contents, err)
			}
			if _, err := os.Stat(quarantine); !os.IsNotExist(err) {
				t.Fatalf("quarantine = %v; want restoration to the original path", err)
			}
		})
	}
}

func TestStatePurgerQuarantineFailureLeavesStateAddressable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(dir, "devices-v2.json")
	if err := os.WriteFile(stateFile, []byte("recovery"), 0o600); err != nil {
		t.Fatal(err)
	}
	ops := defaultStatePurgeOps()
	ops.rename = func(string, string) error { return errors.New("quarantine failed") }

	if err := newStatePurgerWithOps(dir, ops).PurgeAll(); err == nil {
		t.Fatal("PurgeAll() error = nil, want quarantine failure")
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("recovery state is no longer addressable: %v", err)
	}
}

type failingSyncCloser struct{}

func (failingSyncCloser) Sync() error  { return errors.New("sync failed") }
func (failingSyncCloser) Close() error { return nil }
