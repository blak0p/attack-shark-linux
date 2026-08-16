package desktop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type listenerFake struct {
	mu       sync.RWMutex
	onStatus func(x6.StatusEvent)
	ready    chan struct{}
	once     sync.Once
}

func (f *listenerFake) Listen(ctx context.Context, onStatus func(x6.StatusEvent)) error {
	f.mu.Lock()
	f.onStatus = onStatus
	f.mu.Unlock()
	f.once.Do(func() { close(f.ready) })
	<-ctx.Done()
	return nil
}

func newListenerFake() *listenerFake { return &listenerFake{ready: make(chan struct{})} }

func (f *listenerFake) emit(event x6.StatusEvent) {
	f.mu.RLock()
	callback := f.onStatus
	f.mu.RUnlock()
	if callback != nil {
		callback(event)
	}
}

type eventSinkFake struct {
	mu      sync.Mutex
	events  []StatusEvent
	emitted bool
}

func (f *eventSinkFake) Emit(event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emitted = true
	if status, ok := payload.(StatusEvent); ok {
		f.events = append(f.events, status)
	}
}

func (f *eventSinkFake) snapshot() (bool, []StatusEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.emitted, f.events
}

func TestListenerFoldsStatusAndStageIntoSnapshot(t *testing.T) {
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()})
	listener := newListenerFake()
	sink := &eventSinkFake{}
	service.AttachListener(listener, sink)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartListener(ctx)

	select {
	case <-listener.ready:
	case <-time.After(time.Second):
		t.Fatal("StartListener() never ran the listener")
	}

	listener.emit(x6.StatusEvent{Connection: x6.Dongle, BatteryPercent: 72, BatteryAvailable: true})
	snapshot := service.GetSnapshot()
	if snapshot.Connection != "dongle" || snapshot.Battery == nil || *snapshot.Battery != 72 {
		t.Fatalf("after heartbeat snapshot = %#v; want dongle battery 72", snapshot)
	}

	listener.emit(x6.StatusEvent{Connection: x6.Dongle, ActiveStage: 4, StageAvailable: true})
	snapshot = service.GetSnapshot()
	if snapshot.ObservedStage == nil || *snapshot.ObservedStage != 4 || snapshot.ObservedDPI == nil || *snapshot.ObservedDPI != 3200 || snapshot.Revision != 0 {
		t.Fatalf("after stage event snapshot = %#v; want stage-only observation mapped from acknowledged state", snapshot)
	}

	emitted, events := sink.snapshot()
	if !emitted || len(events) != 2 {
		t.Fatalf("emitted = %v, events = %+v; want two status events", emitted, events)
	}
	if events[0].Battery == nil || *events[0].Battery != 72 || events[0].ActiveStage != nil {
		t.Fatalf("events[0] = %+v; want battery-only event", events[0])
	}
	if events[1].ActiveStage == nil || *events[1].ActiveStage != 4 || events[1].Battery != nil {
		t.Fatalf("events[1] = %+v; want stage-only event", events[1])
	}
}

func TestStageObservationUsesAcknowledgedMappingAndPreemptsUnsentIntent(t *testing.T) {
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()})
	listener := newListenerFake()
	service.AttachListener(listener, &eventSinkFake{})
	service.StageDPI(ToDTO(x6.DPIConfig{StageMask: 0x3f, LiftDistance: 1, ActiveStage: 1, DPI: [8]int{1600, 1200, 3200, 2400, 5600, 26000, 50, 50}}))

	service.handleStatusEvent(x6.StatusEvent{Connection: x6.Dongle, ActiveStage: 3, StageAvailable: true})
	got := service.GetSnapshot()
	if got.ObservedStage == nil || *got.ObservedStage != 3 || got.ObservedDPI == nil || *got.ObservedDPI != 1600 {
		t.Fatalf("stage observation = %#v; want stage 3 mapped to acknowledged 1600 DPI", got)
	}
	if got.Pending != got.Applied {
		t.Fatalf("pending = %#v; want unsent intent discarded in favor of acknowledged mapping %#v", got.Pending, got.Applied)
	}
}

func TestStageObservationKeepsMissingAcknowledgedMappingUnknown(t *testing.T) {
	applied := x6.DefaultDPIConfig()
	applied.StageMask = 0x03
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: applied})
	service.handleStatusEvent(x6.StatusEvent{Connection: x6.Dongle, ActiveStage: 3, StageAvailable: true})

	got := service.GetSnapshot()
	if got.ObservedStage == nil || *got.ObservedStage != 3 || got.ObservedDPI != nil {
		t.Fatalf("missing mapping observation = %#v; want stage 3 with unknown DPI", got)
	}
}

func TestInventoryRefreshStartsNewStageObservationEpoch(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, nil))
	selected := service.RefreshInventory(context.Background()).Selected
	service.handleAttributedStatusEvent(StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, ActiveStage: intPtr(3)})
	if service.GetSnapshot().ObservedStage == nil {
		t.Fatal("stage report did not establish an observation")
	}
	service.RefreshInventory(context.Background())
	if got := service.GetSnapshot(); got.ObservedStage != nil || got.ObservedDPI != nil {
		t.Fatalf("reconnect snapshot = %#v; want prior epoch observation cleared", got)
	}
}
