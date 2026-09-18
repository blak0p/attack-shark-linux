package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
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

func TestComposeEmergencyResetBuildsOneRunnerForTheService(t *testing.T) {
	dataDir := t.TempDir()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	command := &targetedCommandFake{}
	inventory := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{
		VendorID: 0x1D57, ProductID: 0xFA60, Serial: "reset-device", Path: "/dev/hidraw0",
	}}}, command)
	service := desktop.Compose(nil, nil, configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)).AttachInventory(inventory)
	service.RefreshInventory(context.Background())

	result, err := composeEmergencyReset(dataDir, inventory, service).Run(context.Background())
	if err != nil || result.Cleanup.State != "success" {
		t.Fatalf("reset result = %#v, %v; want successful cleanup", result, err)
	}
	if calls, _ := command.observations(); calls != 3 {
		t.Fatalf("reset commands = %d; want exactly one runner's three lanes", calls)
	}
}

func TestDesktopCompositionLoadsVersionTwoPollingWithoutChangingDPI(t *testing.T) {
	dataDir := t.TempDir()
	id := mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"}
	config := x6.DefaultDeviceConfig()
	config.DPI[0], config.PollingRate = 1600, x6.PollingRate250
	if err := configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json")).Save(id, "attack-shark-x6", "Attack Shark X6", 2, config); err != nil {
		t.Fatalf("seed version 2 device config: %v", err)
	}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	service := composeDesktopServiceWithTargeted(dataDir, nil, nil, registry, inventorySourceFake{candidates: []transport.Candidate{{
		VendorID: id.VendorID, ProductID: id.ProductID, Serial: id.Serial, Path: "/dev/hidraw0",
	}}}, nil)
	service.RefreshInventory(context.Background())
	if got := service.GetSnapshot().Applied.DPI[0]; got != 1600 {
		t.Fatalf("loaded DPI = %d, want 1600", got)
	}
	if got := service.GetPollingSnapshot().Applied; got != x6.PollingRate250 {
		t.Fatalf("loaded polling = %d, want 250", got)
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
	if inventory.Devices[0].Warning != "" {
		t.Fatalf("device warning = %q; want durable serialless binding", inventory.Devices[0].Warning)
	}
	if inventory.Selected == nil {
		t.Fatal("RefreshInventory().Selected = nil; want a selected durable binding")
	}
	if inventory.Selected.SessionOnly {
		t.Fatalf("RefreshInventory().Selected.SessionOnly = true; want false")
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

	var migrated x6.DPIConfig
	err = configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json")).Load(
		inventory.Selected.ID, "attack-shark-x6", &migrated,
	)
	if err != nil {
		t.Fatalf("load migrated selected-device state: %v", err)
	}
	if migrated.DPI[0] != 1600 {
		t.Fatalf("migrated applied DPI = %d; want 1600", migrated.DPI[0])
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
	var migrated x6.DPIConfig
	if err := configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json")).Load(selected.Selected.ID, "attack-shark-x6", &migrated); err != nil {
		t.Fatalf("load explicitly selected device migration: %v", err)
	}
	if migrated.DPI[0] != 2400 {
		t.Fatalf("explicitly selected migration = %d; want 2400 DPI", migrated.DPI[0])
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

func TestLinuxAppImagePackagingMetadataAndTaskContract(t *testing.T) {
	desktop, err := os.ReadFile("../../packaging/desktop/attack-shark-x6.desktop")
	if err != nil {
		t.Fatalf("read desktop metadata: %v", err)
	}
	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Attack Shark X6 Configurator",
		"Exec=attack-shark-linux",
		"Icon=attack-shark-x6",
		"Categories=Settings;HardwareSettings;",
	} {
		if !strings.Contains(string(desktop), want) {
			t.Errorf("desktop metadata must contain %q", want)
		}
	}

	icon, err := os.ReadFile("../../packaging/icons/attack-shark-x6.svg")
	if err != nil {
		t.Fatalf("read AppImage icon: %v", err)
	}
	if !strings.Contains(string(icon), "<svg") {
		t.Fatal("AppImage icon must be SVG content")
	}

	taskfile, err := os.ReadFile("Taskfile.yml")
	if err != nil {
		t.Fatalf("read AppImage package task: %v", err)
	}
	for _, want := range []string{
		"package:",
		"build:frontend",
		"go build -o build/attack-shark-linux .",
		"wails3 generate appimage",
		"-binary build/attack-shark-linux",
		"-icon ../../packaging/icons/attack-shark-x6.svg",
		"-desktopfile ../../packaging/desktop/attack-shark-x6.desktop",
		"-outputdir build",
		"-builddir build/appimage",
		"PKG_CONFIG_PATH=\"/usr/lib/pkgconfig${PKG_CONFIG_PATH:+:${PKG_CONFIG_PATH}}\"",
	} {
		if !strings.Contains(string(taskfile), want) {
			t.Errorf("AppImage package task must contain %q", want)
		}
	}
	packageTask := strings.Split(string(taskfile), "\n  package:container:")[0]
	if strings.Contains(packageTask, "wails3 package") {
		t.Fatal("AppImage package task must not recursively invoke wails3 package")
	}
}

func TestPackagedAppImageWebKitProcessesAreExecutable(t *testing.T) {
	appImage, err := filepath.Abs(filepath.Join("build", "attack-shark-linux-x86_64.AppImage"))
	if err != nil {
		t.Fatalf("resolve AppImage path: %v", err)
	}
	if _, err := os.Stat(appImage); os.IsNotExist(err) {
		t.Skip("AppImage artifact is not available; run task package:container before artifact verification")
	} else if err != nil {
		t.Fatalf("stat AppImage artifact: %v", err)
	}
	competingAppImage := filepath.Join("build", "Attack_Shark_X6_Configurator-x86_64.AppImage")
	if _, err := os.Stat(competingAppImage); err == nil {
		t.Fatalf("competing AppImage %q must not remain beside the canonical artifact", competingAppImage)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat competing AppImage: %v", err)
	}

	extractDir := t.TempDir()
	command := exec.Command(appImage, "--appimage-extract")
	command.Dir = extractDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("extract AppImage without launching it: %v\n%s", err, output)
	}

	for _, process := range []string{"WebKitNetworkProcess", "WebKitWebProcess"} {
		path := filepath.Join(extractDir, "squashfs-root", "usr", "lib", "x86_64-linux-gnu", "webkitgtk-6.0", process)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat packaged %s: %v", process, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("packaged %s mode = %04o, want an executable mode", process, info.Mode().Perm())
		}
	}
}

func TestRootlessUbuntuAppImageContainerRoute(t *testing.T) {
	taskfile, err := os.ReadFile("Taskfile.yml")
	if err != nil {
		t.Fatalf("read container packaging task: %v", err)
	}
	for _, want := range []string{
		"package:container:",
		"podman build --tag attack-shark-x6-appimage-builder:ubuntu-24.04",
		"--file ../../packaging/appimage/Containerfile",
		"podman run --rm --userns=keep-id",
		"--volume ../..:/workspace:Z",
		"--env APPIMAGE_EXTRACT_AND_RUN=1",
		"wails3 package GOOS=linux",
		"mv build/Attack_Shark_X6_Configurator-x86_64.AppImage build/attack-shark-linux-x86_64.AppImage",
	} {
		if !strings.Contains(string(taskfile), want) {
			t.Errorf("container packaging task must contain %q", want)
		}
	}

	containerfile, err := os.ReadFile("../../packaging/appimage/Containerfile")
	if err != nil {
		t.Fatalf("read AppImage builder containerfile: %v", err)
	}
	for _, want := range []string{
		"FROM node:22-bookworm-slim AS node",
		"FROM ubuntu:24.04",
		"COPY --from=node /usr/local /usr/local",
		"libgtk-4-dev",
		"libwebkitgtk-6.0-dev",
		"libwebkitgtk-6.0-4",
		"libayatana-appindicator3-dev",
		"go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.23",
	} {
		if !strings.Contains(string(containerfile), want) {
			t.Errorf("AppImage builder containerfile must contain %q", want)
		}
	}
	for _, unsupported := range []string{
		"libwebkit2gtk-4.1-dev",
		"webkit2gtk-4.1.pc",
		"webkitgtk-4.1",
		"/usr/include/webkit",
	} {
		if strings.Contains(string(containerfile), unsupported) {
			t.Errorf("AppImage builder must not include unsupported WebKitGTK 4.1 compatibility layer %q", unsupported)
		}
	}
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

func TestGeneratedWailsBindingsExposePollingAndLightingOperations(t *testing.T) {
	service, err := os.ReadFile("../../frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/service.ts")
	if err != nil {
		t.Fatalf("read generated desktop service binding: %v", err)
	}
	for _, operation := range []string{"GetPollingSnapshot", "StagePollingRate", "ResetToFactory", "GetLightingSnapshot", "StageLighting", "ApplyLighting"} {
		if !strings.Contains(string(service), "export function "+operation) {
			t.Errorf("generated Wails service binding must expose %s", operation)
		}
	}

	for _, path := range []string{
		"../../frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models.ts",
		"frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models.ts",
	} {
		models, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated desktop model binding %q: %v", path, err)
		}
		for _, model := range []string{"class PollingSnapshot", "class LightingSnapshot"} {
			if !strings.Contains(string(models), model) {
				t.Errorf("generated Wails model binding %q must expose %s", path, model)
			}
		}
		if !strings.Contains(string(models), `"Error": Error`) {
			t.Errorf("generated Wails polling binding %q must expose the typed Error field", path)
		}
	}

	lightingModels, err := os.ReadFile("../../frontend/bindings/github.com/blak0p/attack-shark-linux/internal/protocol/x6/models.ts")
	if err != nil {
		t.Fatalf("read generated lighting selection model binding: %v", err)
	}
	if !strings.Contains(string(lightingModels), "class LightingSelection") {
		t.Error("generated Wails model binding must expose LightingSelection")
	}

	effects, err := os.ReadFile("../../frontend/bindings/github.com/blak0p/attack-shark-linux/internal/x6/models.ts")
	if err != nil {
		t.Fatalf("read generated lighting effect model binding: %v", err)
	}
	for _, model := range []string{"class LightingEffect", "class LightingSpeedVariant", "class LightingColorTemplate"} {
		if !strings.Contains(string(effects), model) {
			t.Errorf("generated Wails model binding must expose %s", model)
		}
	}
	for _, field := range []string{
		`"DefaultTemplateID": LightingTemplateID`,
		`"SpeedVariants": LightingSpeedVariant[]`,
		`"ColorTemplates": LightingColorTemplate[]`,
		`$$parsedSource["SpeedVariants"] = $$createField3_0($$parsedSource["SpeedVariants"])`,
		`$$parsedSource["ColorTemplates"] = $$createField4_0($$parsedSource["ColorTemplates"])`,
		`"TemplateID": LightingTemplateID`,
		`"CSSColor": string`,
	} {
		if !strings.Contains(string(effects), field) {
			t.Errorf("generated Wails lighting bindings must preserve %s", field)
		}
	}
}

func TestConfigurationBaselineDocumentsPollingDefaultsAndPersistence(t *testing.T) {
	contents, err := os.ReadFile("../../docs/config-baseline.md")
	if err != nil {
		t.Fatalf("read configuration baseline: %v", err)
	}

	baseline := strings.ToLower(strings.Join(strings.Fields(string(contents)), " "))
	for _, want := range []string{
		"Polling rate | 1000 Hz (factory default)",
		"serial-bearing device profiles persist the acknowledged selection per device",
		"Session-only devices apply the selection for the current session only.",
	} {
		if !strings.Contains(baseline, strings.ToLower(strings.Join(strings.Fields(want), " "))) {
			t.Errorf("configuration baseline must document %q", want)
		}
	}
}
