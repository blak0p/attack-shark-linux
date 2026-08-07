package configstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func TestStoreAtomicallyPersistsAppliedDPIWithoutChangingFactoryDefaults(t *testing.T) {
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
	if got, err := os.ReadFile(factory); err != nil || string(got) != `{"version":1}` {
		t.Fatalf("factory defaults changed: %q, %v", got, err)
	}
}
