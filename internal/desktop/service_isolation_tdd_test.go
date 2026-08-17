package desktop

import (
	"context"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestDeviceStateIsolatedAcrossSelectionAndAttributedListenerUpdates(t *testing.T) {
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	bravo := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "bravo", Path: "/dev/hidraw1"}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(
		mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{alpha, bravo}}, nil),
	).attachAutomaticSave(&fakeSyncScheduler{})
	inventory := service.RefreshInventory(context.Background())
	if len(inventory.Devices) != 2 {
		t.Fatalf("RefreshInventory() = %#v; want two devices", inventory)
	}

	alphaID := inventory.Devices[0].ID
	bravoID := inventory.Devices[1].ID
	if selected := service.SelectDevice(alphaID); selected.Error.Code != "" || selected.Selected == nil {
		t.Fatalf("SelectDevice(alpha) = %#v", selected)
	}
	alphaPending := x6.DefaultDPIConfig()
	alphaPending.DPI[0] = 1200
	service.StageDPI(ToDTO(alphaPending))
	alphaSnapshot := service.ApplyDPI(context.Background())
	if alphaSnapshot.Error.Code != StaleBinding || alphaSnapshot.Applied.DPI[0] != 800 || alphaSnapshot.Pending.DPI[0] != 1200 || alphaSnapshot.Factory.DPI[0] != 800 {
		t.Fatalf("failed alpha apply = %#v; want alpha-only applied, pending, factory, and stale error state", alphaSnapshot)
	}

	if selected := service.SelectDevice(bravoID); selected.Error.Code != "" || selected.Selected == nil {
		t.Fatalf("SelectDevice(bravo) = %#v", selected)
	}
	bravoPending := x6.DefaultDPIConfig()
	bravoPending.DPI[0] = 2400
	service.StageDPI(ToDTO(bravoPending))
	bravoSnapshot := service.GetSnapshot()
	if bravoSnapshot.Pending.DPI[0] != 2400 || bravoSnapshot.Pending.DPI[0] == alphaSnapshot.Pending.DPI[0] {
		t.Fatalf("bravo snapshot = %#v; want independent bravo pending DPI", bravoSnapshot)
	}

	wrongPath := "/dev/hidraw9"
	service.handleAttributedStatusEvent(StatusEvent{ID: alphaID, Path: wrongPath, InventoryRevision: 1, Connection: "wired", Battery: intPtr(10)})
	service.handleAttributedStatusEvent(StatusEvent{ID: alphaID, Path: alpha.Path, InventoryRevision: 1, Connection: "wired", Battery: intPtr(10)})
	if got := service.GetSnapshot(); got.Connection != "" || got.Battery != nil || got.Pending.DPI[0] != 2400 {
		t.Fatalf("bravo after alpha events = %#v; want unchanged bravo state", got)
	}

	selected := service.SelectDevice(alphaID)
	service.handleAttributedStatusEvent(StatusEvent{ID: alphaID, Path: alpha.Path, InventoryRevision: selected.Selected.InventoryRevision, Connection: "wired", Battery: intPtr(10), ActiveStage: intPtr(3)})
	got := service.GetSnapshot()
	if got.Connection != "wired" || got.Battery == nil || *got.Battery != 10 || got.ObservedStage == nil || *got.ObservedStage != 3 || got.ObservedDPI == nil || *got.ObservedDPI != 1600 || got.Pending != got.Applied || got.Revision != alphaSnapshot.Revision {
		t.Fatalf("alpha after matching event = %#v; want only alpha stage observation and preempted intent", got)
	}
}

func intPtr(value int) *int { return &value }

func TestAttributedListenerUpdatesAreRaceSafeForSelectedDevice(t *testing.T) {
	alpha := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(
		mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{alpha}}, nil),
	)
	selected := service.RefreshInventory(context.Background()).Selected
	if selected == nil {
		t.Fatal("RefreshInventory() did not select the only device")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for stage := 1; stage <= 64; stage++ {
			service.handleAttributedStatusEvent(StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, Connection: "dongle", ActiveStage: intPtr(stage % 8)})
		}
	}()
	for range 64 {
		_ = service.GetSnapshot()
	}
	<-done
	if got := service.GetSnapshot(); got.Connection != "dongle" || got.Revision != 0 || got.ObservedStage == nil || *got.ObservedStage != 7 {
		t.Fatalf("GetSnapshot() = %#v; want valid selected-device stage observations only", got)
	}
}
