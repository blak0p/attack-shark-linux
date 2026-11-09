package configstore

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRestoresAppliedStateWithoutChangingFactoryDefaults(t *testing.T) {
	dir := t.TempDir()
	factory := filepath.Join(dir, "factory.json")
	if err := os.WriteFile(factory, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(dir, "applied.json"), factory)
	config := device.DefaultDPIConfig()
	config.ActiveStage = 3
	if err := store.SaveApplied(config); err != nil {
		t.Fatalf("SaveApplied() error = %v", err)
	}
	got, err := store.LoadApplied()
	if err != nil || got.ActiveStage != 3 {
		t.Fatalf("LoadApplied() = %#v, %v", got, err)
	}
	contents, err := os.ReadFile(factory)
	if err != nil || string(contents) != `{"version":1}` {
		t.Fatalf("factory defaults = %q, %v; want unchanged", contents, err)
	}
}

func TestLoadFactorySeedsDefaultsWhenMissingAndReusesSeed(t *testing.T) {
	dir := t.TempDir()
	factoryPath := filepath.Join(dir, "factory-defaults.json")
	store := New(filepath.Join(dir, "applied.json"), factoryPath)

	seeded, err := store.LoadFactory()
	if err != nil {
		t.Fatalf("LoadFactory() error = %v", err)
	}
	if seeded != device.DefaultDPIConfig() {
		t.Fatalf("seeded factory = %#v; want DefaultDPIConfig", seeded)
	}
	if _, err := os.Stat(factoryPath); err != nil {
		t.Fatalf("factory defaults not seeded: %v", err)
	}
	again, err := store.LoadFactory()
	if err != nil || again != seeded {
		t.Fatalf("repeated LoadFactory() = %#v, %v; want persisted seed", again, err)
	}
}

func TestLoadFactoryReadsExistingSeedWithoutReseeding(t *testing.T) {
	dir := t.TempDir()
	factoryPath := filepath.Join(dir, "factory-defaults.json")
	store := New(filepath.Join(dir, "applied.json"), factoryPath)
	custom := device.DefaultDPIConfig()
	custom.DPI[0] = 1200

	if err := store.SaveApplied(custom); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "applied.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(factoryPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := store.LoadFactory()
	if err != nil || got.DPI[0] != 1200 {
		t.Fatalf("LoadFactory() = %#v, %v; want seeded custom factory", got, err)
	}
}

func TestLoadFactoryRejectsUnusableSeedAndReseeds(t *testing.T) {
	dir := t.TempDir()
	factoryPath := filepath.Join(dir, "factory-defaults.json")
	if err := os.WriteFile(factoryPath, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(dir, "applied.json"), factoryPath)

	got, err := store.LoadFactory()
	if err != nil || got != device.DefaultDPIConfig() {
		t.Fatalf("LoadFactory() = %#v, %v; want reseeded factory defaults", got, err)
	}
}

func TestAppliedStateIsRestoredOnRestart(t *testing.T) {
	dir := t.TempDir()
	appliedPath, factoryPath := filepath.Join(dir, "applied.json"), filepath.Join(dir, "factory.json")
	config := device.DefaultDPIConfig()
	config.DPI[0] = 1600
	if err := New(appliedPath, factoryPath).SaveApplied(config); err != nil {
		t.Fatalf("SaveApplied() error = %v", err)
	}
	restarted, err := New(appliedPath, factoryPath).LoadApplied()
	if err != nil || restarted != config {
		t.Fatalf("restarted state = %#v, %v; want %#v", restarted, err, config)
	}
}
