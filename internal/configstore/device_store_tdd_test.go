package configstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestSeriallessStoreAtomicCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices-v2.json")
	store := NewDeviceStore(path)
	claimID, _, err := store.CreateClaim("desk receiver", "x6", transport.TopologyEvidence{Bus: 1, Ports: []uint8{4}})
	if err != nil {
		t.Fatalf("CreateClaim() error = %v", err)
	}
	if err := store.SaveClaimSettings(claimID, 1, map[string]int{"dpi": 1600}); err != nil {
		t.Fatalf("SaveClaimSettings() error = %v", err)
	}
	lookups, err := store.ListClaimLookups()
	if err != nil || len(lookups) != 1 {
		t.Fatalf("ListClaimLookups() = %#v, %v", lookups, err)
	}
	if lookups[0].Alias != "desk receiver" || lookups[0].ValidatedProfile != "x6" || lookups[0].Topology.String() != "1-4" {
		t.Fatalf("lookup = %#v, want only claim metadata", lookups[0])
	}
	encoded, err := json.Marshal(lookups[0])
	if err != nil || string(encoded) != `{"alias":"desk receiver","validatedProfile":"x6","topology":{"bus":1,"ports":[4]}}` {
		t.Fatalf("lookup JSON = %s, %v; want no settings fields", encoded, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".claims", string(claimID)+".manifest.json")); err != nil {
		t.Fatalf("private manifest missing: %v", err)
	}
}

func TestSeriallessStorePreservesMalformedPayloadAndReadsV2BeforeV3Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices-v2.json")
	legacy := []byte(`{"version":2,"devices":{},"migrations":{}}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewDeviceStore(path)
	if _, err := store.ListClaimLookups(); err != nil {
		t.Fatalf("ListClaimLookups() v2 read error = %v", err)
	}
	claimID, _, err := store.CreateClaim("moved receiver", "x6", transport.TopologyEvidence{Bus: 2, Ports: []uint8{3, 7}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveClaimSettings(claimID, 1, map[string]int{"dpi": 800}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, ".claims", string(claimID)+".manifest.json")
	if err := os.WriteFile(manifest, []byte(`{malformed`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var destination map[string]int
	if err := store.LoadClaimSettings(claimID, &destination); err == nil {
		t.Fatal("LoadClaimSettings() error = nil, want malformed manifest rejection")
	}
	after, err := os.ReadFile(manifest)
	if err != nil || string(after) != string(before) {
		t.Fatalf("malformed manifest changed = %q, %v", after, err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(contents, []byte(`"version":3`)) {
		t.Fatalf("v3 first write = %q, %v", contents, err)
	}
}

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
