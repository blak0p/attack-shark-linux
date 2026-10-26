package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestProfilesDamagedExistingFilesAreNotReseeded(t *testing.T) {
	for _, damage := range []string{"index", "payload"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			s := profileService(t, root, func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil })
			selectProfileDevice(t, s, 0)
			initial := s.GetProfiles()
			if initial.Error.Code != "" {
				t.Fatal(initial)
			}
			dirs, err := os.ReadDir(root)
			if err != nil || len(dirs) != 1 {
				t.Fatalf("directories: %v %v", dirs, err)
			}
			name := "index.json"
			if damage == "payload" {
				name = initial.ActiveID + ".json"
			}
			path := filepath.Join(root, dirs[0].Name(), name)
			damaged := []byte("{damaged original data")
			if err = os.WriteFile(path, damaged, 0600); err != nil {
				t.Fatal(err)
			}
			// Restart with a valid seed: damaged existing files must still win over it.
			restarted := profileService(t, root, func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil })
			selectProfileDevice(t, restarted, 0)
			got := restarted.GetProfiles()
			if got.Error.Code != ProfileUnavailable || got.Applied {
				t.Fatalf("damaged existing: %+v", got)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(damaged) {
				t.Fatalf("damaged file overwritten: %q %v", after, err)
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 2 {
				t.Fatalf("unexpected new files: %v %v", entries, err)
			}
		})
	}
}
func TestProfilesUnattachedCompatibility(t *testing.T) {
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()})
	before := s.GetSnapshot()
	if got := s.GetProfiles(); got.Error.Code != SelectionRequired || got.Applied {
		t.Fatalf("profiles: %+v", got)
	}
	after := s.GetSnapshot()
	if after.Applied != before.Applied || after.Pending != before.Pending {
		t.Fatalf("legacy snapshot changed: %+v", after)
	}
}
