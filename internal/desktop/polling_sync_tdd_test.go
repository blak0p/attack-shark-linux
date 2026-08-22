package desktop

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestPollingSyncCoordinatorCoalescesLatestTargetedRevision(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	var calls []pollingCall
	coordinator := NewPollingSyncCoordinator(scheduler, func(Binding) bool { return true }, func(binding Binding, revision uint64, rate x6.PollingRate) error {
		calls = append(calls, pollingCall{binding: binding, revision: revision, rate: rate})
		return nil
	})
	binding := testBinding("alpha")

	if err := coordinator.ScheduleAt(binding, 1, x6.PollingRate125); err != nil {
		t.Fatalf("ScheduleAt(first) error = %v", err)
	}
	if err := coordinator.ScheduleAt(binding, 2, x6.PollingRate1000); err != nil {
		t.Fatalf("ScheduleAt(latest) error = %v", err)
	}
	scheduler.Advance(syncDebounceDelay)

	if len(calls) != 1 || calls[0].binding != binding || calls[0].revision != 2 || calls[0].rate != x6.PollingRate1000 {
		t.Fatalf("calls = %#v; want one latest targeted polling apply", calls)
	}
}

func TestPollingServiceAcknowledgesBeforeSaveRetriesAndExcludesSessionOnly(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	serial := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	command := &pollingCommandFake{}
	scheduler := &fakeSyncScheduler{}
	saves := 0
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{serial}}, command)).
		AttachPollingPersistence(
			func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), os.ErrNotExist },
			func(Binding, x6.DeviceConfig) error {
				saves++
				if saves == 1 {
					return errors.New("disk full")
				}
				return nil
			},
		).
		attachPollingAutomaticSave(scheduler)
	service.RefreshInventory(context.Background())
	service.StagePollingRate(x6.PollingRate500)
	scheduler.Advance(syncDebounceDelay)

	failed := service.GetPollingSnapshot()
	if command.calls != 1 || failed.Applied != x6.PollingRate500 || failed.Firmware != "success" || failed.Persistence != "failed" || !failed.RetryAvailable {
		t.Fatalf("acknowledged polling result = %#v, calls=%d; want applied before retryable save failure", failed, command.calls)
	}
	if got := service.RetryPollingPersistence(); got.Persistence != "success" || got.RetryAvailable || saves != 2 {
		t.Fatalf("RetryPollingPersistence() = %#v, saves=%d; want persistence-only success", got, saves)
	}
}

func TestResetToFactoryStagesPollingDefault(t *testing.T) {
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()})
	service.StagePollingRate(x6.PollingRate125)
	service.ResetToFactory()
	if got := service.GetPollingSnapshot().Desired; got != x6.PollingRate1000 {
		t.Fatalf("polling desired after ResetToFactory() = %d, want factory 1000", got)
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

type pollingCall struct {
	binding  Binding
	revision uint64
	rate     x6.PollingRate
}

type pollingCommandFake struct{ calls int }

func (f *pollingCommandFake) SendAndAwaitBound(_ context.Context, _ mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.calls++
	continueReading([]byte{0x03, 0x10, 0x50, 0, 0x06})
	return nil
}
