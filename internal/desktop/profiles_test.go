package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func profileService(t *testing.T, root string, load func(Binding) (x6.DeviceConfig, error)) *Service {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	// No command is attached: bootstrap and management must never need hardware.
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "profile-a", Path: "/dev/fake-a"}, {VendorID: 0x1d57, ProductID: 0xfa60, Serial: "profile-b", Path: "/dev/fake-b"}}}, nil))
	s.AttachProfiles(configstore.NewProfileStore(root), load)
	return s
}
func selectProfileDevice(t *testing.T, s *Service, index int) Binding {
	t.Helper()
	inventory := s.RefreshInventory(context.Background())
	selected := s.SelectDevice(inventory.Devices[index].ID)
	if selected.Selected == nil {
		t.Fatal(selected)
	}
	return *selected.Selected
}
func TestProfilesBootstrapManagementAndRestart(t *testing.T) {
	root := t.TempDir()
	loads := 0
	saved := x6.DefaultDeviceConfig()
	saved.DPI[0] = 2400
	load := func(Binding) (x6.DeviceConfig, error) { loads++; return saved, nil }
	s := profileService(t, root, load)
	if got := s.GetProfiles(); got.Error.Code != SelectionRequired || got.Applied {
		t.Fatalf("before selection: %+v", got)
	}
	binding := selectProfileDevice(t, s, 0)
	first := s.GetProfiles()
	if first.Error.Code != "" || len(first.Profiles) != 1 || first.ActiveID == "" || first.Applied || !strings.Contains(first.Notice, "local settings") || !strings.Contains(first.Notice, "No mouse settings were changed") {
		t.Fatalf("bootstrap: %+v", first)
	}
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"Profiles"`) || strings.Contains(string(raw), `"profiles"`) || !strings.Contains(string(raw), `"CreatedAt"`) {
		t.Fatalf("UI casing: %s", raw)
	}
	reentry := s.GetProfiles()
	if reentry.ActiveID != first.ActiveID || len(reentry.Profiles) != 1 || loads != 1 {
		t.Fatalf("reentry: %+v loads=%d", reentry, loads)
	}
	p, err := configstore.NewProfileStore(root).Read(binding.ID, binding.ProfileID, first.ActiveID)
	if err != nil {
		t.Fatal(err)
	}
	var payload profileConfiguration
	if err = json.Unmarshal(p.Configuration, &payload); err != nil || payload.Device.DPI[0] != 2400 {
		t.Fatalf("payload: %+v %v", payload, err)
	}
	if got := s.DeleteProfile(first.ActiveID); got.Error.Code != ProtectedProfile || got.ActiveID != first.ActiveID || len(got.Profiles) != 1 {
		t.Fatalf("last deletion: %+v", got)
	}
	if got := s.CreateProfile("../invalid"); got.Error.Code != InvalidProfile || got.ActiveID != first.ActiveID || len(got.Profiles) != 1 {
		t.Fatalf("rejected create: %+v", got)
	}
	created := s.CreateProfile("Gaming")
	if created.Error.Code != "" || created.ActiveID == first.ActiveID || len(created.Profiles) != 2 || created.Applied {
		t.Fatalf("create: %+v", created)
	}
	if got := s.RenameProfile(created.ActiveID, "Work"); got.Error.Code != "" || got.Profiles[1].Name != "Work" {
		t.Fatalf("rename: %+v", got)
	}
	duplicate := s.DuplicateProfile(first.ActiveID, "Copy")
	if duplicate.Error.Code != "" || duplicate.ActiveID != created.ActiveID || len(duplicate.Profiles) != 3 {
		t.Fatalf("duplicate: %+v", duplicate)
	}
	if got := s.DeleteProfile(created.ActiveID); got.Error.Code != ProtectedProfile {
		t.Fatalf("active deletion: %+v", got)
	}
	if got := s.DeleteProfile(first.ActiveID); got.Error.Code != "" || len(got.Profiles) != 2 {
		t.Fatalf("delete: %+v", got)
	}
	if got := s.DismissProfileNotice(); got.Error.Code != "" || got.Notice != "" {
		t.Fatalf("dismiss: %+v", got)
	}
	restarted := profileService(t, root, func(Binding) (x6.DeviceConfig, error) {
		t.Fatal("existing profiles must not reload seed")
		return saved, nil
	})
	selectProfileDevice(t, restarted, 0)
	if got := restarted.GetProfiles(); got.Error.Code != "" || got.ActiveID != created.ActiveID || got.Notice != "" || len(got.Profiles) != 2 {
		t.Fatalf("restart: %+v", got)
	}
	selectProfileDevice(t, s, 1)
	isolated := s.GetProfiles()
	if isolated.Error.Code != "" || len(isolated.Profiles) != 1 || isolated.ActiveID == created.ActiveID {
		t.Fatalf("isolation: %+v", isolated)
	}
}
func TestProfilesAbsentDefaultsAndMalformedSeed(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		s := profileService(t, t.TempDir(), func(Binding) (x6.DeviceConfig, error) { return x6.DeviceConfig{}, os.ErrNotExist })
		selectProfileDevice(t, s, 0)
		got := s.GetProfiles()
		if got.Error.Code != "" || !strings.Contains(got.Notice, "app defaults") || got.Applied {
			t.Fatalf("defaults: %+v", got)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		root := t.TempDir()
		s := profileService(t, root, func(Binding) (x6.DeviceConfig, error) { return x6.DeviceConfig{}, errors.New("damaged source") })
		selectProfileDevice(t, s, 0)
		if got := s.GetProfiles(); got.Error.Code != ProfileUnavailable || got.Applied {
			t.Fatalf("malformed: %+v", got)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("overwrote source: %v %v", entries, err)
		}
	})
	t.Run("invalid config", func(t *testing.T) {
		s := profileService(t, t.TempDir(), func(Binding) (x6.DeviceConfig, error) { c := x6.DefaultDeviceConfig(); c.DPI[0] = -1; return c, nil })
		selectProfileDevice(t, s, 0)
		if got := s.GetProfiles(); got.Error.Code != InvalidProfile {
			t.Fatalf("invalid: %+v", got)
		}
	})
}
