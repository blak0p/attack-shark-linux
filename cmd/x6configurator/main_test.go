package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type inventorySourceFake struct{ candidates []transport.Candidate }

func (f inventorySourceFake) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type profileValidBackendFake struct {
	candidate transport.Candidate
	valid     bool
	observed  *mouse.HIDFacts
}

func (f profileValidBackendFake) Enumerate(context.Context, transport.Match) ([]transport.Candidate, error) {
	return []transport.Candidate{f.candidate}, nil
}

func (f profileValidBackendFake) ProfileValid(_ context.Context, _ transport.Candidate, facts mouse.HIDFacts) bool {
	if f.observed != nil {
		*f.observed = facts
	}
	return f.valid
}

type targetedCommandFake struct {
	mu       sync.Mutex
	calls    int
	bindings []mouse.Binding
}

func (f *targetedCommandFake) SendAndAwaitBound(_ context.Context, binding mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.bindings = append(f.bindings, binding)
	continueReading([]byte{0x04, 0x04})
	return nil
}

func (f *targetedCommandFake) observations() (int, []mouse.Binding) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, append([]mouse.Binding(nil), f.bindings...)
}

func TestEmbeddedFrontendServesBuiltRuntimeEntry(t *testing.T) {
	index, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("read embedded frontend index: %v", err)
	}
	if !strings.Contains(string(index), `<div id="root"></div>`) {
		t.Fatal("embedded frontend index must provide the React root")
	}

	match := regexp.MustCompile(`src="/(assets/[^\"]+\.js)"`).FindStringSubmatch(string(index))
	if len(match) != 2 {
		t.Fatal("embedded frontend index must load a built JavaScript entry")
	}
	entry, err := fs.ReadFile(assets, "frontend/dist/"+match[1])
	if err != nil {
		t.Fatalf("read embedded frontend JavaScript entry: %v", err)
	}
	if len(entry) == 0 {
		t.Fatal("embedded frontend JavaScript entry must not be empty")
	}
}

func TestWailsConfigurationBuildsFrontendWithoutInvokingNativeBuild(t *testing.T) {
	contents, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read Wails configuration: %v", err)
	}

	var config struct {
		Frontend struct {
			Build string `json:"build"`
		} `json:"frontend"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		t.Fatalf("parse Wails configuration: %v", err)
	}
	if config.Frontend.Build != "npm run build" {
		t.Fatalf("frontend build = %q, want npm run build", config.Frontend.Build)
	}
	if strings.Contains(config.Frontend.Build, "wails3 build") {
		t.Fatal("frontend build must not invoke wails3 build recursively")
	}
}

func TestNewDesktopServiceUsesDurableAppliedState(t *testing.T) {
	service := newDesktopService(t.TempDir())

	snapshot := service.GetSnapshot()
	if snapshot.Pending.DPI[0] != 800 || snapshot.Applied.DPI[0] != 800 {
		t.Fatalf("initial snapshot = %#v, want the default persisted DPI configuration", snapshot)
	}
}

func TestDesktopCompositionSelectsSoleSeriallessValidatedX6(t *testing.T) {
	dataDir := t.TempDir()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	candidate := transport.Candidate{
		Path:            "/dev/hidraw3",
		VendorID:        0x1D57,
		ProductID:       0xFA60,
		InterfaceNumber: 2,
		Connection:      transport.Dongle,
	}
	var observed mouse.HIDFacts
	service := composeDesktopServiceWithTargeted(
		dataDir,
		nil,
		nil,
		registry,
		x6InventorySource{backend: profileValidBackendFake{candidate: candidate, valid: true, observed: &observed}},
		nil,
	)

	inventory := service.RefreshInventory(context.Background())
	if len(inventory.Devices) != 1 || !inventory.Devices[0].Eligible {
		t.Fatalf("RefreshInventory() = %#v; want one eligible device", inventory)
	}
	if inventory.Devices[0].Warning != "session-only (no serial)" {
		t.Fatalf("device warning = %q; want session-only (no serial)", inventory.Devices[0].Warning)
	}
	if inventory.Selected == nil {
		t.Fatal("RefreshInventory().Selected = nil; want a selected session-only binding")
	}
	if !inventory.Selected.SessionOnly {
		t.Fatalf("RefreshInventory().Selected.SessionOnly = false; want true")
	}
	if observed.StatusInput.InterfaceNumber != 2 {
		t.Fatalf("ProfileValid() facts interface = %d; want interface 2", observed.StatusInput.InterfaceNumber)
	}
}

func TestDesktopCompositionMigratesLegacyStateForTheSoleSelectedDevice(t *testing.T) {
	dataDir := t.TempDir()
	legacy := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	applied := x6.DefaultDPIConfig()
	applied.DPI[0] = 1600
	if err := legacy.SaveApplied(applied); err != nil {
		t.Fatalf("seed legacy applied state: %v", err)
	}
	if _, err := legacy.LoadFactory(); err != nil {
		t.Fatalf("seed legacy factory state: %v", err)
	}

	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := newDesktopService(dataDir).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{
		VendorID: 0x1D57, ProductID: 0xFA60, Serial: "sole-device", Path: "/dev/hidraw0",
	}}}, nil))

	inventory := service.RefreshInventory(context.Background())
	if inventory.Selected == nil || inventory.Selected.ID.Serial != "sole-device" {
		t.Fatalf("RefreshInventory() = %#v; want the sole selected device", inventory)
	}

	var migrated struct {
		Applied struct {
			DPI struct {
				DPI [8]int `json:"dpi"`
			} `json:"dpi"`
		} `json:"applied"`
	}
	err = configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json")).Load(
		inventory.Selected.ID, "attack-shark-x6", &migrated,
	)
	if err != nil {
		t.Fatalf("load migrated selected-device state: %v", err)
	}
	if migrated.Applied.DPI.DPI[0] != 1600 {
		t.Fatalf("migrated applied DPI = %d; want 1600", migrated.Applied.DPI.DPI[0])
	}
}

func TestDesktopCompositionReportsLegacyMigrationFailureForTheSoleSelectedDevice(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "applied-dpi.json"), []byte(`{"version":99,"dpi":{}}`), 0o600); err != nil {
		t.Fatalf("seed incompatible legacy applied state: %v", err)
	}
	legacy := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	if _, err := legacy.LoadFactory(); err != nil {
		t.Fatalf("seed legacy factory state: %v", err)
	}

	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := newDesktopService(dataDir).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{
		VendorID: 0x1D57, ProductID: 0xFA60, Serial: "sole-device", Path: "/dev/hidraw0",
	}}}, nil))

	inventory := service.RefreshInventory(context.Background())
	if inventory.Selected == nil || inventory.Error.Code != "migration_failed" {
		t.Fatalf("RefreshInventory() = %#v; want selected device and migration_failed", inventory)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "applied-dpi.json.v1.bak")); !os.IsNotExist(err) {
		t.Fatalf("incompatible legacy input must not be backed up, stat error = %v", err)
	}
}

func TestDesktopCompositionMigratesLegacyStateAfterExplicitSelection(t *testing.T) {
	dataDir := t.TempDir()
	legacy := configstore.New(filepath.Join(dataDir, "applied-dpi.json"), filepath.Join(dataDir, "factory-defaults.json"))
	applied := x6.DefaultDPIConfig()
	applied.DPI[0] = 2400
	if err := legacy.SaveApplied(applied); err != nil {
		t.Fatalf("seed legacy applied state: %v", err)
	}
	if _, err := legacy.LoadFactory(); err != nil {
		t.Fatalf("seed legacy factory state: %v", err)
	}

	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := newDesktopService(dataDir).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"},
	}}, nil))

	inventory := service.RefreshInventory(context.Background())
	if inventory.Selected != nil {
		t.Fatalf("RefreshInventory() = %#v; want no implicit multi-device selection", inventory)
	}
	selected := service.SelectDevice(inventory.Devices[1].ID)
	if selected.Error.Code != "" || selected.Selected == nil {
		t.Fatalf("SelectDevice() = %#v; want explicit bravo selection", selected)
	}
	var migrated map[string]json.RawMessage
	if err := configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json")).Load(selected.Selected.ID, "attack-shark-x6", &migrated); err != nil {
		t.Fatalf("load explicitly selected device migration: %v", err)
	}
	if !strings.Contains(string(migrated["applied"]), "2400") {
		t.Fatalf("explicitly selected migration = %s; want 2400 DPI", migrated["applied"])
	}
}

func TestDesktopCompositionPersistsSelectedApplyAndRestoresDistinctDeviceValues(t *testing.T) {
	dataDir := t.TempDir()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0", Connection: transport.Dongle}
	bravo := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1", Connection: transport.Dongle}
	command := &targetedCommandFake{}
	service := composeDesktopServiceWithTargeted(dataDir, nil, nil, registry, inventorySourceFake{candidates: []transport.Candidate{alpha, bravo}}, command)
	service.RefreshInventory(context.Background())
	for _, tc := range []struct {
		id  mouse.DeviceID
		dpi int
	}{
		{id: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"}, dpi: 1600},
		{id: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}, dpi: 2400},
	} {
		if got := service.SelectDevice(tc.id); got.Error.Code != "" {
			t.Fatalf("SelectDevice() = %#v", got)
		}
		next := service.GetSnapshot().Pending
		next.DPI[0] = tc.dpi
		service.StageDPI(next)
		if got := service.ApplyDPI(context.Background()); got.Error.Code != "" || got.Applied.DPI[0] != tc.dpi {
			t.Fatalf("ApplyDPI() = %#v", got)
		}
	}
	calls, bindings := command.observations()
	if calls != 2 || bindings[0].Path != "/dev/hidraw0" || bindings[1].Path != "/dev/hidraw1" {
		t.Fatalf("targeted commands = calls:%d bindings:%#v; want alpha then bravo exact paths", calls, bindings)
	}
	reloaded := composeDesktopServiceWithTargeted(dataDir, nil, nil, registry, inventorySourceFake{candidates: []transport.Candidate{alpha, bravo}}, command)
	reloaded.RefreshInventory(context.Background())
	if got := reloaded.SelectDevice(bravoID()); got.Error.Code != "" {
		t.Fatalf("SelectDevice(bravo) = %#v", got)
	}
	if got := reloaded.GetSnapshot().Applied.DPI[0]; got != 2400 {
		t.Fatalf("reloaded bravo DPI = %d, want 2400", got)
	}
}

func bravoID() mouse.DeviceID {
	return mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}
}

func TestWailsBuildTaskDoesNotRecursivelyInvokeWails(t *testing.T) {
	contents, err := os.ReadFile("Taskfile.yml")
	if err != nil {
		t.Fatalf("read Wails build task: %v", err)
	}
	if strings.Contains(string(contents), "wails3 build") {
		t.Fatal("Wails build task must not recursively invoke wails3 build")
	}
	if !strings.Contains(string(contents), "go build") {
		t.Fatal("Wails build task must compile the composition root directly")
	}
}
