package desktop

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestPollingComponentOwnsSelectedStateAndPreservesSleepPersistence(t *testing.T) {
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()})
	if service.pollingComponent == nil || service.pollingComponent.states == nil {
		t.Fatal("polling component must own initialized per-device state")
	}
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	service.AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, nil))
	service.RefreshInventory(context.Background())
	service.StageNormalSleep(12)
	service.StagePollingRate(device.PollingRate500)
	if got := service.GetNormalSleepSnapshot().Pending; got != 12 {
		t.Fatalf("sleep pending = %v; want 12 after polling stage", got)
	}
}

func TestPollingSyncCoordinatorCoalescesLatestTargetedRevision(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	var calls []pollingCall
	coordinator := NewPollingSyncCoordinator(scheduler, func(Binding) bool { return true }, func(binding Binding, revision uint64, rate device.PollingRate) error {
		calls = append(calls, pollingCall{binding: binding, revision: revision, rate: rate})
		return nil
	})
	binding := testBinding("alpha")

	if err := coordinator.ScheduleAt(binding, 1, device.PollingRate125); err != nil {
		t.Fatalf("ScheduleAt(first) error = %v", err)
	}
	if err := coordinator.ScheduleAt(binding, 2, device.PollingRate1000); err != nil {
		t.Fatalf("ScheduleAt(latest) error = %v", err)
	}
	scheduler.Advance(syncDebounceDelay)

	if len(calls) != 1 || calls[0].binding != binding || calls[0].revision != 2 || calls[0].rate != device.PollingRate1000 {
		t.Fatalf("calls = %#v; want one latest targeted polling apply", calls)
	}
}

func TestPollingServiceAcknowledgesBeforeSaveRetriesAndExcludesSessionOnly(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	serial := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	command := &pollingCommandFake{}
	scheduler := &fakeSyncScheduler{}
	saves := 0
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{serial}}, command)).
		AttachPollingPersistence(
			func(Binding) (device.DeviceConfig, error) { return device.DefaultDeviceConfig(), os.ErrNotExist },
			func(Binding, device.DeviceConfig) error {
				saves++
				if saves == 1 {
					return errors.New("disk full")
				}
				return nil
			},
		).
		attachPollingAutomaticSave(scheduler)
	service.RefreshInventory(context.Background())
	service.StagePollingRate(device.PollingRate500)
	scheduler.Advance(syncDebounceDelay)
	if command.calls != 0 {
		t.Fatalf("scheduler writes = %d, want 0 before explicit apply", command.calls)
	}
	service.ApplyPollingRate(context.Background())

	failed := service.GetPollingSnapshot()
	if command.calls != 1 || failed.Applied != device.PollingRate500 || failed.Firmware != "success" || failed.Persistence != "failed" || !failed.RetryAvailable {
		t.Fatalf("acknowledged polling result = %#v, calls=%d; want applied before retryable save failure", failed, command.calls)
	}
	if got := service.RetryPollingPersistence(); got.Persistence != "success" || got.RetryAvailable || saves != 2 {
		t.Fatalf("RetryPollingPersistence() = %#v, saves=%d; want persistence-only success", got, saves)
	}
}

func TestResetToFactoryWithoutRunnerDoesNotStagePolling(t *testing.T) {
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()})
	service.StagePollingRate(device.PollingRate125)
	result := service.ResetToFactory(context.Background())
	if result.Error.Code != SelectionRequired || !result.RetryAvailable {
		t.Fatalf("ResetToFactory() = %#v, want retryable selection failure", result)
	}
}

func TestPollingPersistenceExcludesSessionOnlyBindings(t *testing.T) {
	if pollingPersistenceAllowed(Binding{SessionOnly: true}) {
		t.Fatal("session-only polling binding must not be persisted")
	}
	if !pollingPersistenceAllowed(Binding{}) {
		t.Fatal("serial-bearing polling binding must remain persistable")
	}
}

func TestPollingApplyAndPersistenceCompletionEmitSnapshots(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	serial := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	scheduler := &fakeSyncScheduler{}
	events := &pollingEventSink{}
	saves := 0
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{serial}}, &pollingCommandFake{})).
		AttachListener(nil, events).
		AttachPollingPersistence(
			func(Binding) (device.DeviceConfig, error) { return device.DefaultDeviceConfig(), os.ErrNotExist },
			func(Binding, device.DeviceConfig) error {
				saves++
				if saves == 1 {
					return errors.New("disk full")
				}
				return nil
			},
		).
		attachPollingAutomaticSave(scheduler)
	service.RefreshInventory(context.Background())
	service.StagePollingRate(device.PollingRate500)
	scheduler.Advance(syncDebounceDelay)
	service.ApplyPollingRate(context.Background())

	if len(events.events) != 1 {
		t.Fatalf("polling completion events = %#v; want one apply completion", events.events)
	}
	failed, ok := events.events[0].payload.(PollingConfigurationEvent)
	if !ok || failed.Snapshot.Firmware != "success" || failed.Snapshot.Persistence != "failed" || !failed.Snapshot.RetryAvailable {
		t.Fatalf("first completion = %#v; want applied polling with retryable persistence failure", events.events[0].payload)
	}

	service.RetryPollingPersistence()
	if len(events.events) != 2 {
		t.Fatalf("polling completion events after retry = %#v; want persistence retry completion", events.events)
	}
	retried, ok := events.events[1].payload.(PollingConfigurationEvent)
	if !ok || retried.Snapshot.Persistence != "success" || retried.Snapshot.Persisted == nil || *retried.Snapshot.Persisted != device.PollingRate500 {
		t.Fatalf("retry completion = %#v; want persisted polling result", events.events[1].payload)
	}
}

func TestPollingApplyFailureEmitsFailureSnapshot(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	serial := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	scheduler := &fakeSyncScheduler{}
	events := &pollingEventSink{}
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{serial}}, &pollingCommandFake{err: errors.New("ack timeout")})).
		AttachListener(nil, events).
		attachPollingAutomaticSave(scheduler)
	service.RefreshInventory(context.Background())
	service.StagePollingRate(device.PollingRate250)
	scheduler.Advance(syncDebounceDelay)
	service.ApplyPollingRate(context.Background())

	if len(events.events) != 1 || events.events[0].event != "mouse:polling-configuration" {
		t.Fatalf("polling completion events = %#v; want one polling failure event", events.events)
	}
	failed, ok := events.events[0].payload.(PollingConfigurationEvent)
	if !ok || failed.Snapshot.Desired != device.PollingRate250 || failed.Snapshot.Applied != device.PollingRate1000 || failed.Snapshot.Firmware != "failed" {
		t.Fatalf("failure completion = %#v; want failed polling snapshot", events.events[0].payload)
	}
}

type pollingCall struct {
	binding  Binding
	revision uint64
	rate     device.PollingRate
}

type pollingCommandFake struct {
	calls int
	err   error
}

func (f *pollingCommandFake) SendAndAwaitBound(_ context.Context, _ mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0, 0x06})
	return nil
}

type pollingEvent struct {
	event   string
	payload any
}

type pollingEventSink struct{ events []pollingEvent }

func (s *pollingEventSink) Emit(event string, payload any) {
	s.events = append(s.events, pollingEvent{event: event, payload: payload})
}
