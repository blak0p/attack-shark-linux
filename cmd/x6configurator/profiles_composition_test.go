package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestDesktopCompositionProfilesBootstrapAndEmergencyReset(t *testing.T) {
	dataDir := t.TempDir()
	id := mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "comp-device"}
	config := x6.DefaultDeviceConfig()
	config.DPI[0] = 3200
	config.PollingRate = x6.PollingRate500

	// Seed devices-v2.json
	deviceStore := configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json"))
	if err := deviceStore.Save(id, "attack-shark-x6", "Attack Shark X6", 3, config); err != nil {
		t.Fatalf("save initial device config: %v", err)
	}

	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{
		VendorID: id.VendorID, ProductID: id.ProductID, Serial: id.Serial, Path: "/dev/hidraw-test", Connection: transport.Dongle,
	}
	cmd := &targetedCommandFake{}
	service := composeDesktopServiceWithTargeted(dataDir, nil, nil, registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, cmd)

	service.RefreshInventory(context.Background())
	if sel := service.SelectDevice(id); sel.Error.Code != "" {
		t.Fatalf("SelectDevice failed: %+v", sel)
	}

	// 1. GetProfiles should bootstrap automatic profile seeded from devices-v2.json
	profiles := service.GetProfiles()
	if profiles.Error.Code != "" || len(profiles.Profiles) != 1 || !strings.Contains(profiles.Notice, "local settings") {
		t.Fatalf("GetProfiles bootstrap mismatch: %+v", profiles)
	}

	// Verify profile files exist under dataDir/profiles
	profilesDir := filepath.Join(dataDir, "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected profiles directory with 1 device folder, got: %v, err: %v", entries, err)
	}

	// 2. Ordinary edit (DPI apply) should update active profile JSON
	nextDPI := service.GetSnapshot().Pending
	nextDPI.DPI[0] = 1600
	service.StageDPI(nextDPI)
	if res := service.ApplyDPI(context.Background()); res.Error.Code != "" {
		t.Fatalf("ApplyDPI failed: %+v", res)
	}

	// 3. Emergency Reset purges dataDir including dataDir/profiles
	resetRes := service.ResetToFactory(context.Background())
	if resetRes.Cleanup.State != "success" || resetRes.Error.Code != "" {
		t.Fatalf("ResetToFactory failed: %+v", resetRes)
	}

	// After emergency reset, dataDir is cleaned/recreated; profiles directory must be gone or empty
	if entries, err := os.ReadDir(profilesDir); err == nil && len(entries) > 0 {
		t.Fatalf("profiles directory was not purged during emergency reset: %v", entries)
	}
}
