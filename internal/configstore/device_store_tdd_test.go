package configstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/mouse"
)

func TestDeviceStoreRoundTripPreservesUnknownAndRejectsPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices-v2.json")
	store := NewDeviceStore(path)
	id := mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "one"}
	config := map[string]any{"dpi": float64(1600)}
	if err := store.Save(id, "x6", "Attack Shark X6", 1, config); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := store.Load(id, "x6", &got); err != nil || got["dpi"] != float64(1600) {
		t.Fatalf("Load() = %#v, %v", got, err)
	}
	if err := store.Save(id, "other", "Other", 1, config); !errors.Is(err, ErrIncompatibleRecord) {
		t.Fatalf("Save() incompatible error = %v", err)
	}
	if err := store.Save(mouse.DeviceID{}, "x6", "Attack Shark X6", 1, config); !errors.Is(err, mouse.ErrAmbiguousIdentity) {
		t.Fatalf("Save() ambiguous identity error = %v", err)
	}
	if err := store.Save(id, "x6", "Attack Shark X6", 1, map[string]any{"path": "/dev/hidraw0"}); !errors.Is(err, ErrTransientPath) {
		t.Fatalf("Save() path error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("rejected records changed state = %q, %v", after, err)
	}
}

func TestDeviceStoreMigratesV1BackupFirstAndDeterministically(t *testing.T) {
	dir := t.TempDir()
	applied, factory := legacyFiles(t, dir)
	store := NewDeviceStore(filepath.Join(dir, "devices-v2.json"))
	target := MigrationTarget{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "one"}, Profile: "x6", Model: "Attack Shark X6"}

	result, err := store.MigrateV1(applied, factory, []MigrationTarget{target}, false)
	if err != nil || result.Status != MigrationImported || result.TargetKey != target.ID.Key() {
		t.Fatalf("MigrateV1() = %#v, %v", result, err)
	}
	for _, source := range []string{applied, factory} {
		backup, backupErr := os.ReadFile(source + ".v1.bak")
		original, originalErr := os.ReadFile(source)
		if backupErr != nil || originalErr != nil || string(backup) != string(original) {
			t.Fatalf("backup for %s = %q, %v; original = %q, %v", source, backup, backupErr, original, originalErr)
		}
	}
	result, err = store.MigrateV1(applied, factory, []MigrationTarget{target}, false)
	if err != nil || result.Status != MigrationAlreadyImported {
		t.Fatalf("repeat MigrateV1() = %#v, %v", result, err)
	}
}

func TestDeviceStoreMigrationRejectsUnsafeTargetsWithoutOverwrite(t *testing.T) {
	for _, tt := range []struct {
		name      string
		targets   []MigrationTarget
		cancelled bool
		want      error
	}{
		{"zero target", nil, false, ErrMigrationNoTarget},
		{"ambiguous target", []MigrationTarget{testTarget("one"), testTarget("two")}, false, ErrMigrationSelectionRequired},
		{"cancelled", []MigrationTarget{testTarget("one")}, true, ErrMigrationCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			applied, factory := legacyFiles(t, dir)
			store := NewDeviceStore(filepath.Join(dir, "devices-v2.json"))
			_, err := store.MigrateV1(applied, factory, tt.targets, tt.cancelled)
			if !errors.Is(err, tt.want) {
				t.Fatalf("MigrateV1() error = %v, want %v", err, tt.want)
			}
			if _, err := os.Stat(filepath.Join(dir, "devices-v2.json")); !os.IsNotExist(err) {
				t.Fatalf("v2 state = %v, want absent", err)
			}
		})
	}
}

func TestDeviceStoreMigrationPreservesExistingAndFailedDestinations(t *testing.T) {
	for _, tt := range []struct {
		name      string
		legacy    []byte
		prepare   func(*DeviceStore, MigrationTarget) error
		storePath func(string) string
		want      error
	}{
		{"incompatible legacy", []byte(`{"version":9,"dpi":{}}`), nil, func(dir string) string { return filepath.Join(dir, "devices-v2.json") }, nil},
		{"existing target", nil, func(store *DeviceStore, target MigrationTarget) error {
			return store.Save(target.ID, target.Profile, target.Model, 1, map[string]int{"dpi": 800})
		}, func(dir string) string { return filepath.Join(dir, "devices-v2.json") }, ErrMigrationTargetExists},
		{"write failure", nil, nil, func(dir string) string {
			blocked := filepath.Join(dir, "blocked")
			if err := os.WriteFile(blocked, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(blocked, "devices-v2.json")
		}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			applied, factory := legacyFiles(t, dir)
			if tt.legacy != nil {
				if err := os.WriteFile(applied, tt.legacy, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			target := testTarget("one")
			store := NewDeviceStore(tt.storePath(dir))
			if tt.prepare != nil {
				if err := tt.prepare(store, target); err != nil {
					t.Fatal(err)
				}
			}
			_, err := store.MigrateV1(applied, factory, []MigrationTarget{target}, false)
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("MigrateV1() error = %v, want %v", err, tt.want)
			}
			if tt.want == nil && err == nil {
				t.Fatal("MigrateV1() error = nil, want rejection")
			}
			if _, statErr := os.Stat(applied); statErr != nil {
				t.Fatalf("legacy applied state missing: %v", statErr)
			}
		})
	}
}

func legacyFiles(t *testing.T, dir string) (string, string) {
	t.Helper()
	applied, factory := filepath.Join(dir, "applied.json"), filepath.Join(dir, "factory.json")
	for _, path := range []string{applied, factory} {
		if err := os.WriteFile(path, []byte(`{"version":1,"dpi":{"active":3}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return applied, factory
}

func testTarget(serial string) MigrationTarget {
	return MigrationTarget{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: serial}, Profile: "x6", Model: "Attack Shark X6"}
}
