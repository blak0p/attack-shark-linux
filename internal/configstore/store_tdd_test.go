package configstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func TestStoreRestoresAppliedStateWithoutChangingFactoryDefaults(t *testing.T) {
	dir := t.TempDir()
	factory := filepath.Join(dir, "factory.json")
	if err := os.WriteFile(factory, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(filepath.Join(dir, "applied.json"), factory)
	config := x6.DefaultDPIConfig()
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
	if seeded != x6.DefaultDPIConfig() {
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
	custom := x6.DefaultDPIConfig()
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
	if err != nil || got != x6.DefaultDPIConfig() {
		t.Fatalf("LoadFactory() = %#v, %v; want reseeded factory defaults", got, err)
	}
}

func TestAppliedStateIsSavedOnlyAfterAcknowledgementAndRestoredOnRestart(t *testing.T) {
	dir := t.TempDir()
	appliedPath := filepath.Join(dir, "applied.json")
	store := New(appliedPath, filepath.Join(dir, "factory.json"))
	config := x6.DefaultDPIConfig()
	config.DPI[0] = 1600
	command := &ackCommand{beforeAcknowledgement: func() {
		if _, err := os.Stat(appliedPath); !os.IsNotExist(err) {
			t.Fatalf("applied state exists before acknowledgement: %v", err)
		}
	}}

	if err := x6.NewCommandService(command).ApplyAndPersist(context.Background(), config, store); err != nil {
		t.Fatalf("ApplyAndPersist() error = %v", err)
	}
	restarted, err := store.LoadApplied()
	if err != nil || restarted.DPI[0] != 1600 || len(command.report) != 52 {
		t.Fatalf("restarted state = %#v, %v; report length = %d", restarted, err, len(command.report))
	}
}

type ackCommand struct {
	report                []byte
	beforeAcknowledgement func()
}

func (f *ackCommand) SendAndAwait(_ context.Context, report []byte, keepReading func([]byte) bool) error {
	f.report = append([]byte(nil), report...)
	f.beforeAcknowledgement()
	if keepReading([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		return os.ErrInvalid
	}
	return nil
}
