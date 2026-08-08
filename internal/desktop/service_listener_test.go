package desktop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type listenerFake struct {
	onStatus func(x6.StatusEvent)
}

func (f *listenerFake) Listen(ctx context.Context, onStatus func(x6.StatusEvent)) error {
	f.onStatus = onStatus
	<-ctx.Done()
	return nil
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
	listener := &listenerFake{}
	sink := &eventSinkFake{}
	service.AttachListener(listener, sink)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartListener(ctx)

	deadline := time.Now().Add(time.Second)
	for listener.onStatus == nil {
		if time.Now().After(deadline) {
			t.Fatal("StartListener() never ran the listener")
		}
		time.Sleep(10 * time.Millisecond)
	}

	listener.onStatus(x6.StatusEvent{Connection: x6.Dongle, BatteryPercent: 72, BatteryAvailable: true})
	snapshot := service.GetSnapshot()
	if snapshot.Connection != "dongle" || snapshot.Battery == nil || *snapshot.Battery != 72 {
		t.Fatalf("after heartbeat snapshot = %#v; want dongle battery 72", snapshot)
	}

	listener.onStatus(x6.StatusEvent{Connection: x6.Dongle, ActiveStage: 4, StageAvailable: true})
	snapshot = service.GetSnapshot()
	if snapshot.Pending.ActiveStage != 4 || snapshot.Applied.ActiveStage != 4 || snapshot.Revision != 1 {
		t.Fatalf("after stage event snapshot = %#v; want active stage 4 with bumped revision", snapshot)
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
