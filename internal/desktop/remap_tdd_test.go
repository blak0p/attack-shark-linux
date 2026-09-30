package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type remapSelectionPersistence struct {
	load func(Binding) (x6.DeviceConfig, error)
	save func(Binding, x6.DeviceConfig) error
}

func (p remapSelectionPersistence) Load(b Binding) (x6.DeviceConfig, error) { return p.load(b) }
func (p remapSelectionPersistence) Save(b Binding, config x6.DeviceConfig) error {
	if p.save != nil {
		return p.save(b, config)
	}
	return nil
}

func TestRemapOwnerTracksSelectedState(t *testing.T) {
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()})
	owner := service.remapComponent
	if owner == nil || owner.service != service || owner.currentState() != owner.legacy {
		t.Fatal("remap owner is not wired to facade")
	}
	aID := DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"}
	bID := DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta"}
	a := owner.stateForBinding(Binding{ID: aID, SessionOnly: true})
	b := owner.stateForBinding(Binding{ID: bID, SessionOnly: true})
	if a == b || owner.stateForBinding(Binding{ID: aID, SessionOnly: true}) != a {
		t.Fatal("remap state is not isolated by device")
	}
	snapshot := remapSnapshotOf(a)
	snapshot.Pending.Buttons[0].Action = x6.RemapFire
	if remapSnapshotOf(a).Pending.Buttons[0].Action == x6.RemapFire {
		t.Fatal("snapshot aliases owner state")
	}
}

func TestRemapApplyAndRetryPreserveSharedConfig(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	command := &remapCommandFake{ack: true}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command))
	selected := service.RefreshInventory(context.Background()).Selected
	if selected == nil {
		t.Fatal("no selection")
	}
	stored := x6.DefaultDeviceConfig()
	stored.DPI[0], stored.PollingRate, stored.NormalSleepMinutes, stored.ResponseTimeMs = 1600, x6.PollingRate500, 12, 4
	saves := 0
	service.remapPersistence = remapSelectionPersistence{
		load: func(Binding) (x6.DeviceConfig, error) { return stored, nil },
		save: func(binding Binding, config x6.DeviceConfig) error {
			saves++
			if binding != *selected {
				t.Fatalf("save binding = %#v, want %#v", binding, *selected)
			}
			if config.DPIConfig != stored.DPIConfig || config.PollingRate != stored.PollingRate || config.NormalSleepMinutes != stored.NormalSleepMinutes || config.ResponseTimeMs != stored.ResponseTimeMs {
				t.Fatal("remap save replaced unrelated settings")
			}
			if saves == 1 {
				return errors.New("disk full")
			}
			stored = config
			return nil
		},
	}
	draft := x6.DefaultRemapConfig()
	draft.Buttons[0].Action = x6.RemapFire
	failed := service.ApplyRemap(draft)
	if failed.Error.Code != PersistenceFailed || !failed.RetryAvailable {
		t.Fatal("missing persistence-only retry")
	}
	draft.Buttons[0].Action = x6.RemapMute
	got := service.RetryRemapPersistence()
	if got.Error.Code != "" || got.RetryAvailable || got.Persistence != "success" || command.calls != 1 || saves != 2 {
		t.Fatalf("retry = %#v, writes=%d saves=%d", got, command.calls, saves)
	}
	if stored.Remap == nil || stored.Remap.Buttons[0].Action != x6.RemapFire {
		t.Fatal("retry did not persist captured acknowledged config")
	}
}

// Loading A synchronously switches selection before the bound apply accesses
// state again. No goroutine scheduling or elapsed-time assumption is involved.
func TestRemapApplyKeepsCapturedStateWhenSelectionChangesDuringLoad(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	command := &remapCommandFake{ack: true}
	inventory := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta", Path: "/dev/hidraw1"},
	}}, command)
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(inventory)
	devices := service.RefreshInventory(context.Background())
	if len(devices.Devices) != 2 {
		t.Fatalf("inventory = %#v", devices)
	}
	a, b := devices.Devices[0].ID, devices.Devices[1].ID
	if err := inventory.Select(a); err != nil {
		t.Fatal(err)
	}
	switched := false
	service.remapPersistence = remapSelectionPersistence{load: func(binding Binding) (x6.DeviceConfig, error) {
		if binding.ID == a && !switched {
			switched = true
			if err := inventory.Select(b); err != nil {
				t.Fatal(err)
			}
		}
		return x6.DefaultDeviceConfig(), nil
	}}
	config := x6.DefaultRemapConfig()
	config.Buttons[0].Action = x6.RemapFire
	got := service.ApplyRemap(config)
	if !switched {
		t.Fatal("selection hook was not exercised")
	}
	other := service.GetRemapSnapshot()
	if other.Error.Code != "" || !remapConfigsEqual(other.Pending, x6.DefaultRemapConfig()) {
		t.Fatalf("B was mutated by A apply: error = %q, firmware = %q", other.Error.Code, other.Firmware)
	}
	if got.Error.Code != StaleBinding {
		t.Fatalf("captured A error = %q, firmware = %q; want stale binding", got.Error.Code, got.Firmware)
	}
	if command.calls != 0 {
		t.Fatalf("stale apply wrote %d commands", command.calls)
	}
}
