package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/mouse"
	"github.com/alejandro/attack-shark-linux/internal/transport"
	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type statusFake struct {
	status x6.Status
	err    error
}

func (f statusFake) Status(context.Context) (x6.Status, error) { return f.status, f.err }

type writerFake struct {
	calls int
	err   error
}

func (f *writerFake) ApplyAndPersist(context.Context, x6.DPIConfig, x6.AppliedDPIStore) error {
	f.calls++
	return f.err
}

type appliedStoreFake struct {
	applied x6.DPIConfig
	factory x6.DPIConfig
	err     error
}

func (f appliedStoreFake) LoadApplied() (x6.DPIConfig, error) { return f.applied, f.err }
func (f appliedStoreFake) LoadFactory() (x6.DPIConfig, error) {
	if f.factory == (x6.DPIConfig{}) {
		return x6.DefaultDPIConfig(), f.err
	}
	return f.factory, nil
}
func (appliedStoreFake) SaveApplied(x6.DPIConfig) error { return nil }

type inventorySourceFake struct{ candidates []transport.Candidate }

func (f inventorySourceFake) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type fakeHidrawCommand struct {
	calls   int
	binding mouse.Binding
}

func (f *fakeHidrawCommand) SendAndAwaitBound(_ context.Context, binding mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.calls++
	f.binding = binding
	continueReading([]byte{0x04, 0x04})
	return nil
}

func TestSnapshotExposesFactoryDefaultsForReset(t *testing.T) {
	factory := x6.DefaultDPIConfig()
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{})
	got := service.GetSnapshot()
	if got.Factory != ToDTO(factory) {
		t.Fatalf("GetSnapshot().Factory = %#v; want %#v", got.Factory, ToDTO(factory))
	}
}

func TestServiceStagesWithoutWritingAndAppliesOnlyOnAcknowledgedSuccess(t *testing.T) {
	applied := x6.DefaultDPIConfig()
	pending := applied
	pending.DPI[0] = 1600
	for _, tt := range []struct {
		name        string
		applyErr    error
		wantApplied bool
	}{
		{name: "acknowledged success", wantApplied: true},
		{name: "ack failure keeps pending", applyErr: &x6.ServiceError{Kind: x6.AckFailure, Err: errors.New("timeout")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := &writerFake{err: tt.applyErr}
			service := New(statusFake{}, writer, appliedStoreFake{applied: applied})
			staged := service.StageDPI(ToDTO(pending))
			if writer.calls != 0 || staged.Pending.DPI[0] != 1600 {
				t.Fatalf("StageDPI() = %#v, calls = %d; want local pending state and no write", staged, writer.calls)
			}
			got := service.ApplyDPI(context.Background())
			if writer.calls != 1 || (got.Applied.DPI[0] == 1600) != tt.wantApplied || got.Pending.DPI[0] != 1600 {
				t.Fatalf("ApplyDPI() = %#v, calls = %d", got, writer.calls)
			}
		})
	}
}

func TestServiceShowsBatteryOnlyWhenStatusSuppliesIt(t *testing.T) {
	available := New(statusFake{status: x6.Status{Connection: x6.Dongle, BatteryAvailable: true, BatteryPercent: 84}}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).RefreshStatus(context.Background())
	if available.Connection != "dongle" || available.Battery == nil || *available.Battery != 84 {
		t.Fatalf("available status = %#v", available)
	}
	unavailable := New(statusFake{err: &x6.ServiceError{Kind: x6.NoUsableDevice, Err: errors.New("absent")}}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).RefreshStatus(context.Background())
	if unavailable.Battery != nil || unavailable.Error.Code != DeviceUnavailable {
		t.Fatalf("unavailable status = %#v; want unavailable battery and error", unavailable)
	}
}

func TestServiceTracksStagedRevisionAndRetainsFailedPendingConfiguration(t *testing.T) {
	applied := x6.DefaultDPIConfig()
	pending := applied
	pending.DPI[0] = 1600
	service := New(statusFake{}, &writerFake{err: &x6.ServiceError{Kind: x6.AckFailure, Err: errors.New("timeout")}}, appliedStoreFake{applied: applied})

	staged := service.StageDPI(ToDTO(pending))
	if staged.Revision != 1 || staged.Applied.DPI[0] != 800 || staged.Pending.DPI[0] != 1600 {
		t.Fatalf("StageDPI() = %#v; want revision 1 with separate applied and pending values", staged)
	}
	failed := service.ApplyDPI(context.Background())
	if failed.Revision != 1 || failed.Error.Code != ApplyFailed || failed.Applied.DPI[0] != 800 || failed.Pending.DPI[0] != 1600 {
		t.Fatalf("failed ApplyDPI() = %#v; want retained revision and pending configuration", failed)
	}
}

func TestServiceExposesInventoryAndRequiresExplicitDesktopSelection(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	targeted := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"},
	}}, nil)
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(targeted)

	inventory := service.RefreshInventory(context.Background())
	if len(inventory.Devices) != 2 || inventory.Selected != nil {
		t.Fatalf("RefreshInventory() = %#v; want two devices and no implicit selection", inventory)
	}
	if inventory.Devices[1].ID != (DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}) || inventory.Devices[1].Path != "/dev/hidraw1" || inventory.Devices[1].Profile != "attack-shark-x6" || !inventory.Devices[1].Eligible {
		t.Fatalf("RefreshInventory().Devices[1] = %#v; want the selectable bravo device DTO", inventory.Devices[1])
	}

	selected := service.SelectDevice(inventory.Devices[1].ID)
	if selected.Error.Code != "" || selected.Selected == nil || selected.Selected.ID != inventory.Devices[1].ID || selected.Selected.Path != "/dev/hidraw1" {
		t.Fatalf("SelectDevice() = %#v; want the selected bravo binding DTO", selected)
	}
}

func TestApplyDPIRequiresSelectedBindingAndRejectsStaleSelectionBeforeDeviceIO(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	bravo := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"}

	for _, tt := range []struct {
		name       string
		selectID   DeviceID
		candidates []transport.Candidate
		wantError  ErrorCode
	}{
		{name: "two eligible devices without selection", wantError: SelectionRequired},
		{name: "stale selected binding", selectID: DeviceID{VendorID: alpha.VendorID, ProductID: alpha.ProductID, Serial: alpha.Serial}, candidates: []transport.Candidate{{VendorID: alpha.VendorID, ProductID: alpha.ProductID, Serial: alpha.Serial, Path: "/dev/hidraw9"}, bravo}, wantError: StaleBinding},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := &mutableInventorySource{candidates: []transport.Candidate{alpha, bravo}}
			command := &fakeHidrawCommand{}
			legacyWriter := &writerFake{}
			targeted := mouse.NewTargetedService(registry, source, command)
			service := New(statusFake{}, legacyWriter, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(targeted)
			if inventory := service.RefreshInventory(context.Background()); len(inventory.Devices) != 2 {
				t.Fatalf("RefreshInventory() = %#v; want two eligible devices", inventory)
			}
			if tt.selectID != (DeviceID{}) {
				if selected := service.SelectDevice(tt.selectID); selected.Error.Code != "" {
					t.Fatalf("SelectDevice() = %#v", selected)
				}
				source.candidates = tt.candidates
			}

			got := service.ApplyDPI(context.Background())
			if got.Error.Code != tt.wantError {
				t.Fatalf("ApplyDPI().Error = %#v; want %q", got.Error, tt.wantError)
			}
			if command.calls != 0 || legacyWriter.calls != 0 {
				t.Fatalf("device I/O calls = targeted:%d legacy:%d; want zero", command.calls, legacyWriter.calls)
			}
		})
	}
}

func TestApplyDPITargetsTheSelectedBindingWithoutUsingLegacyWriter(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	bravo := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"}
	source := &mutableInventorySource{candidates: []transport.Candidate{alpha, bravo}}
	command := &fakeHidrawCommand{}
	legacyWriter := &writerFake{}
	targeted := mouse.NewTargetedService(registry, source, command)
	service := New(statusFake{}, legacyWriter, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(targeted)
	if inventory := service.RefreshInventory(context.Background()); len(inventory.Devices) != 2 {
		t.Fatalf("RefreshInventory() = %#v; want two eligible devices", inventory)
	}
	if selected := service.SelectDevice(DeviceID{VendorID: bravo.VendorID, ProductID: bravo.ProductID, Serial: bravo.Serial}); selected.Error.Code != "" {
		t.Fatalf("SelectDevice() = %#v", selected)
	}
	pending := x6.DefaultDPIConfig()
	pending.DPI[0] = 1600
	service.StageDPI(ToDTO(pending))

	got := service.ApplyDPI(context.Background())
	if got.Error.Code != "" || got.Applied.DPI[0] != 1600 {
		t.Fatalf("ApplyDPI() = %#v; want selected configuration applied", got)
	}
	if command.calls != 1 || command.binding.ID.Serial != "bravo" || command.binding.Path != "/dev/hidraw1" {
		t.Fatalf("targeted command = calls:%d binding:%#v; want one bravo-path operation", command.calls, command.binding)
	}
	if legacyWriter.calls != 0 {
		t.Fatalf("legacy writer calls = %d; want 0", legacyWriter.calls)
	}
}

func TestSelectedApplyPersistsAndRestoresPerDeviceState(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	bravo := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"}
	command := &fakeHidrawCommand{}
	inventory := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{alpha, bravo}}, command)
	persisted := map[mouse.DeviceID]x6.DPIConfig{}
	defaults := x6.DefaultDPIConfig()
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: defaults, factory: defaults}).AttachInventory(inventory).AttachDevicePersistence(
		func(binding Binding) (x6.DPIConfig, error) {
			value, ok := persisted[binding.ID]
			if !ok {
				return x6.DPIConfig{}, os.ErrNotExist
			}
			return value, nil
		},
		func(binding Binding, value x6.DPIConfig) error { persisted[binding.ID] = value; return nil },
	)
	service.RefreshInventory(context.Background())
	for _, tc := range []struct {
		id  mouse.DeviceID
		dpi int
	}{
		{id: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"}, dpi: 1600},
		{id: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}, dpi: 2400},
	} {
		if result := service.SelectDevice(tc.id); result.Error.Code != "" {
			t.Fatalf("SelectDevice() = %#v", result)
		}
		next := service.GetSnapshot().Pending
		next.DPI[0] = tc.dpi
		service.StageDPI(next)
		if result := service.ApplyDPI(context.Background()); result.Error.Code != "" || result.Applied.DPI[0] != tc.dpi {
			t.Fatalf("ApplyDPI() = %#v", result)
		}
	}
	if command.calls != 2 {
		t.Fatalf("targeted command calls = %d, want 2 exact selected applies", command.calls)
	}
	if persisted[mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"}].DPI[0] != 1600 || persisted[mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}].DPI[0] != 2400 {
		t.Fatalf("persisted = %#v; want distinct device values", persisted)
	}
	reloaded := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: defaults, factory: defaults}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{alpha, bravo}}, command)).AttachDevicePersistence(
		func(binding Binding) (x6.DPIConfig, error) {
			value, ok := persisted[binding.ID]
			if !ok {
				return x6.DPIConfig{}, os.ErrNotExist
			}
			return value, nil
		},
		nil,
	)
	reloaded.RefreshInventory(context.Background())
	if result := reloaded.SelectDevice(mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo"}); result.Error.Code != "" {
		t.Fatalf("SelectDevice(bravo) = %#v", result)
	}
	if got := reloaded.GetSnapshot().Applied.DPI[0]; got != 2400 {
		t.Fatalf("reloaded bravo applied DPI = %d, want 2400", got)
	}
}

func TestSessionOnlyBindingSkipsPersistenceAndMigration(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Path: "/dev/hidraw0"}
	command := &fakeHidrawCommand{}
	persistenceCalls, migrationCalls := 0, 0
	legacyApplied := x6.DefaultDPIConfig()
	legacyApplied.DPI[0] = 2400
	legacyFactory := x6.DefaultDPIConfig()
	legacyFactory.DPI[0] = 3200
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: legacyApplied, factory: legacyFactory}).
		AttachInventory(mouse.NewTargetedService(registry, validatedInventorySource{candidates: []transport.Candidate{candidate}, valid: true}, command)).
		AttachMigrator(func(Binding) error { migrationCalls++; return nil }).
		AttachDevicePersistence(
			func(Binding) (x6.DPIConfig, error) { persistenceCalls++; return x6.DPIConfig{}, os.ErrNotExist },
			func(Binding, x6.DPIConfig) error { persistenceCalls++; return nil },
		)

	inventory := service.RefreshInventory(context.Background())
	if inventory.Selected == nil || !inventory.Selected.SessionOnly || inventory.Devices[0].Warning != "session-only (no serial)" {
		t.Fatalf("RefreshInventory() = %#v; want selected session-only device", inventory)
	}
	if migrationCalls != 0 || persistenceCalls != 0 {
		t.Fatalf("migration=%d persistence=%d; want no durable operations", migrationCalls, persistenceCalls)
	}
	defaults := x6.DefaultDPIConfig()
	if snapshot := service.GetSnapshot(); snapshot.Applied.DPI[0] != defaults.DPI[0] || snapshot.Pending.DPI[0] != defaults.DPI[0] || snapshot.Factory.DPI[0] != defaults.DPI[0] {
		t.Fatalf("session-only snapshot = %#v; want safe defaults %#v rather than legacy applied=%d factory=%d", snapshot, defaults, legacyApplied.DPI[0], legacyFactory.DPI[0])
	}
	if result := service.ApplyDPI(context.Background()); result.Error.Code != "" {
		t.Fatalf("ApplyDPI() = %#v", result)
	}
	if command.calls != 1 || migrationCalls != 0 || persistenceCalls != 0 {
		t.Fatalf("command=%d migration=%d persistence=%d; want hidraw-only apply without durable operations", command.calls, migrationCalls, persistenceCalls)
	}
}

type mutableInventorySource struct{ candidates []transport.Candidate }

func (f *mutableInventorySource) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type validatedInventorySource struct {
	candidates []transport.Candidate
	valid      bool
}

func (f validatedInventorySource) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

func (f validatedInventorySource) ProfileValid(context.Context, transport.Candidate, mouse.HIDFacts) bool {
	return f.valid
}

func TestRefreshInventoryExposesAmbiguousIdentityAsErrorDTO(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "shared", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "shared", Path: "/dev/hidraw1"},
	}}, nil))

	inventory := service.RefreshInventory(context.Background())
	if len(inventory.Devices) != 2 {
		t.Fatalf("RefreshInventory().Devices = %#v; want both ambiguous devices", inventory.Devices)
	}
	if inventory.Error.Code != ErrorCode("ambiguous_identity") {
		t.Fatalf("RefreshInventory().Error = %#v; want ambiguous_identity DTO", inventory.Error)
	}
}

func TestStatusEventsCarryTheSelectedBindingContract(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	targeted := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{
		VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0",
	}}}, nil)
	sink := &contractEventSink{}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(targeted).AttachListener(nil, sink)
	if inventory := service.RefreshInventory(context.Background()); inventory.Selected == nil {
		t.Fatalf("RefreshInventory() = %#v; want the sole selected binding", inventory)
	}

	service.handleStatusEvent(x6.StatusEvent{Connection: x6.Dongle, BatteryPercent: 77, BatteryAvailable: true})
	if sink.name != "mouse:status" {
		t.Fatalf("event name = %q; want mouse:status", sink.name)
	}
	payload, err := json.Marshal(sink.payload)
	if err != nil {
		t.Fatalf("marshal event payload: %v", err)
	}
	var contract struct {
		ID                DeviceID
		Path              string
		InventoryRevision uint64
	}
	if err := json.Unmarshal(payload, &contract); err != nil {
		t.Fatalf("decode event contract: %v", err)
	}
	if contract.ID.Serial != "alpha" || contract.Path != "/dev/hidraw0" || contract.InventoryRevision == 0 {
		t.Fatalf("event contract = %#v; want selected identity, path, and inventory revision", contract)
	}
}

type contractEventSink struct {
	name    string
	payload any
}

func (s *contractEventSink) Emit(name string, payload any) {
	s.name, s.payload = name, payload
}
