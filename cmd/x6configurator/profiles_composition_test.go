package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/attackshark/x6"
	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
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

	// 3. A second profile becomes active; the automatic one stays inactive.
	created := service.CreateProfile("Second")
	if created.Error.Code != "" || len(created.Profiles) != 2 {
		t.Fatalf("CreateProfile failed: %+v", created)
	}
	activeID := created.ActiveID
	watcherPath := filepath.Join(dataDir, "watcher-v1.json")
	if err := os.WriteFile(watcherPath, []byte(`{"version":1,"rules":[{"executable":"cs2","profileID":"keep"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deviceDir := filepath.Join(profilesDir, entries[0].Name())
	activePath := filepath.Join(deviceDir, activeID+".json")
	devicePath := filepath.Join(dataDir, "devices-v2.json")
	before := readTree(t, dataDir)

	// 4. Emergency reset changes only the mouse's records: every other file is
	// byte-identical, the active profile and the device record hold the baseline.
	resetRes := service.ResetToFactory(context.Background())
	if resetRes.Cleanup.State != "success" || resetRes.Error.Code != "" {
		t.Fatalf("ResetToFactory failed: %+v", resetRes)
	}
	after := readTree(t, dataDir)
	for path, contents := range before {
		if path == activePath || path == devicePath {
			continue
		}
		if got, ok := after[path]; !ok || got != contents {
			t.Fatalf("reset changed %s: before %q, after %q (present=%t)", path, contents, got, ok)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Fatalf("reset created unexpected file %s", path)
		}
	}
	if listed := service.GetProfiles(); listed.ActiveID != activeID || len(listed.Profiles) != 2 || listed.Error.Code != "" {
		t.Fatalf("profiles after reset = %+v; want both profiles with %s active", listed, activeID)
	}

	var saved struct {
		Configuration struct {
			Device   device.DeviceConfig       `json:"device"`
			Lighting *device.LightingSelection `json:"lighting"`
			Macros   []json.RawMessage         `json:"macros"`
		} `json:"configuration"`
	}
	if err := json.Unmarshal([]byte(after[activePath]), &saved); err != nil {
		t.Fatalf("decode active profile: %v", err)
	}
	want := resetBaseline()
	if !reflect.DeepEqual(saved.Configuration.Device, want) {
		t.Fatalf("active profile device = %+v, want reset baseline %+v", saved.Configuration.Device, want)
	}
	wantLighting := device.DocumentedResetLightingSettings().LightingSelection
	if saved.Configuration.Lighting == nil || !reflect.DeepEqual(*saved.Configuration.Lighting, wantLighting) {
		t.Fatalf("active profile lighting = %+v, want %+v", saved.Configuration.Lighting, wantLighting)
	}
	if len(saved.Configuration.Macros) != 0 {
		t.Fatalf("active profile macros = %s, want none", saved.Configuration.Macros)
	}
	var record device.DeviceConfig
	if err := deviceStore.Load(id, "attack-shark-x6", &record); err != nil || !reflect.DeepEqual(record, want) {
		t.Fatalf("device record = %+v, %v; want reset baseline %+v", record, err, want)
	}
}

func resetBaseline() device.DeviceConfig {
	settings := device.DocumentedResetLightingSettings()
	remap := device.DefaultRemapConfig()
	return device.DeviceConfig{
		DPIConfig:          device.DocumentedResetDPIConfig(),
		PollingRate:        device.PollingRate1000,
		NormalSleepMinutes: settings.NormalSleepMinutes,
		ResponseTimeMs:     settings.ResponseTimeMs,
		Remap:              &remap,
	}
}

// readTree maps every regular file under root to its contents.
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		contents, err := os.ReadFile(path)
		files[path] = string(contents)
		return err
	})
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	return files
}

func TestEmergencyResetKeepsDamagedProfileStoreAndReportsPersistenceFailure(t *testing.T) {
	dataDir := t.TempDir()
	id := mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "damaged-device"}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{
		VendorID: id.VendorID, ProductID: id.ProductID, Serial: id.Serial, Path: "/dev/hidraw-test", Connection: transport.Dongle,
	}
	command := &targetedCommandFake{}
	service := composeDesktopServiceWithTargeted(dataDir, nil, nil, registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command)
	service.RefreshInventory(context.Background())
	profiles := service.GetProfiles()
	if profiles.Error.Code != "" {
		t.Fatalf("GetProfiles failed: %+v", profiles)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "profiles"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("profiles dir = %v, %v", entries, err)
	}
	activePath := filepath.Join(dataDir, "profiles", entries[0].Name(), profiles.ActiveID+".json")
	if err := os.WriteFile(activePath, []byte("damaged"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := service.ResetToFactory(context.Background())
	if calls, _ := command.observations(); calls != 4 {
		t.Fatalf("reset commands = %d; want all four lanes written", calls)
	}
	if result.Cleanup.State != "failed" || result.Error.Code != "persistence_failed" || !result.RetryAvailable {
		t.Fatalf("ResetToFactory with a damaged profile store = %+v; want retryable persistence_failed", result)
	}
	if contents, err := os.ReadFile(activePath); err != nil || string(contents) != "damaged" {
		t.Fatalf("damaged profile after reset = %q, %v; want it left for the user", contents, err)
	}
}
