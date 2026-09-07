package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestStagingConfigurationDoesNotWriteAfterSchedulerAdvance(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stage func(*Service)
	}{
		{
			name: "DPI",
			stage: func(service *Service) {
				pending := service.GetSnapshot().Pending
				pending.DPI[0] = 1600
				service.StageDPI(pending)
			},
		},
		{
			name: "polling",
			stage: func(service *Service) {
				service.StagePollingRate(x6.PollingRate500)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatalf("NewProfileRegistry() error = %v", err)
			}
			command := &fakeHidrawCommand{}
			scheduler := &fakeSyncScheduler{}
			candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
			service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
				AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command)).
				attachAutomaticSave(scheduler).
				attachPollingAutomaticSave(scheduler)
			service.RefreshInventory(context.Background())

			tt.stage(service)
			scheduler.Advance(syncDebounceDelay)

			if calls := command.callCount(); calls != 0 {
				t.Fatalf("staging then scheduler advance made %d device writes, want 0", calls)
			}
		})
	}
}

func TestApplyPollingRateWritesOnlyAfterExplicitConfirmation(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	command := &pollingCommandFake{}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command))
	service.RefreshInventory(context.Background())

	service.StagePollingRate(x6.PollingRate500)
	if calls := command.calls; calls != 0 {
		t.Fatalf("StagePollingRate() made %d device writes, want 0", calls)
	}

	got := service.ApplyPollingRate(context.Background())
	if command.calls != 1 || got.Applied != x6.PollingRate500 || got.Desired != x6.PollingRate500 || got.Firmware != "success" {
		t.Fatalf("ApplyPollingRate() = %#v, calls=%d; want one acknowledged selected write", got, command.calls)
	}
}

func TestApplyPollingRateRejectsInvalidSelectionWithoutIO(t *testing.T) {
	valid := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	invalid := transport.Candidate{VendorID: 0xFFFF, ProductID: 0x0001, Serial: "unknown", Path: "/dev/hidraw1"}

	for _, tt := range []struct {
		name  string
		setup func(*Service, *mutableInventorySource)
	}{
		{
			name: "absent selection",
			setup: func(service *Service, _ *mutableInventorySource) {
				service.inventory = nil
			},
		},
		{
			name: "stale selection after inventory refresh",
			setup: func(service *Service, source *mutableInventorySource) {
				source.candidates = nil
				service.RefreshInventory(context.Background())
			},
		},
		{
			name: "invalid selection",
			setup: func(service *Service, source *mutableInventorySource) {
				source.candidates = []transport.Candidate{invalid}
				service.RefreshInventory(context.Background())
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatalf("NewProfileRegistry() error = %v", err)
			}
			source := &mutableInventorySource{candidates: []transport.Candidate{valid}}
			command := &pollingCommandFake{}
			saves := 0
			service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
				AttachInventory(mouse.NewTargetedService(registry, source, command)).
				AttachPollingPersistence(
					func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil },
					func(Binding, x6.DeviceConfig) error {
						saves++
						return nil
					},
				)
			service.RefreshInventory(context.Background())
			tt.setup(service, source)

			before := service.GetPollingSnapshot()
			got := service.ApplyPollingRate(context.Background())

			if got.Error.Code != SelectionRequired {
				t.Fatalf("ApplyPollingRate() error = %q, want %q", got.Error.Code, SelectionRequired)
			}
			if got.Desired != before.Desired || got.Applied != before.Applied || got.Persisted != before.Persisted {
				t.Fatalf("ApplyPollingRate() = %#v, before = %#v; want unchanged polling state", got, before)
			}
			if command.calls != 0 || saves != 0 {
				t.Fatalf("ApplyPollingRate() command calls=%d persistence saves=%d; want zero I/O", command.calls, saves)
			}
		})
	}
}

func TestApplyRemapReportsRetryablePersistenceFailureWithoutASecondDeviceWrite(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	command := &remapCommandFake{ack: true}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command))
	persistence := &remapPersistenceFake{failFirst: true}
	service.remapPersistence = persistence
	service.RefreshInventory(context.Background())
	config := x6.DefaultRemapConfig()
	config.Buttons[0].Action = x6.RemapFire

	failed := service.ApplyRemap(config)
	if command.calls != 1 || failed.Applied.Buttons[0].Action != x6.RemapFire || failed.Error.Code != PersistenceFailed || !failed.RetryAvailable {
		t.Fatalf("ApplyRemap() = %#v, writes=%d; want acknowledged state and retryable persistence failure", failed, command.calls)
	}
	retried := service.ApplyRemap(config)
	if command.calls != 1 || persistence.saves != 2 || retried.Error.Code != "" || retried.Persistence != "success" || retried.RetryAvailable {
		t.Fatalf("second ApplyRemap() = %#v, writes=%d saves=%d; want caller-driven persistence-only recovery", retried, command.calls, persistence.saves)
	}
}

func TestApplyRemapValidatesBindingACKAndPersistence(t *testing.T) {
	valid := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	config := x6.DefaultRemapConfig()
	config.Buttons[0].Action = x6.RemapFire

	for _, tt := range []struct {
		name      string
		config    x6.RemapConfig
		setup     func(*Service, *mutableInventorySource)
		ack       bool
		saveErr   error
		wantCode  ErrorCode
		wantCalls int
		wantSaves int
	}{
		{
			name:      "invalid configuration rejects before command",
			config:    x6.RemapConfig{},
			wantCode:  InvalidConfiguration,
			wantCalls: 0,
		},
		{
			name:   "missing selection rejects before command",
			config: config,
			setup: func(service *Service, _ *mutableInventorySource) {
				service.inventory = nil
			},
			wantCode:  SelectionRequired,
			wantCalls: 0,
		},
		{
			name:   "stale binding rejects before command",
			config: config,
			setup: func(_ *Service, source *mutableInventorySource) {
				source.candidates[0].Path = "/dev/hidraw9"
			},
			wantCode:  StaleBinding,
			wantCalls: 0,
		},
		{
			name:      "ack failure does not advance applied state",
			config:    config,
			ack:       false,
			wantCode:  ApplyFailed,
			wantCalls: 1,
		},
		{
			name:      "acknowledged write persists exact selected binding",
			config:    config,
			ack:       true,
			wantCalls: 1,
			wantSaves: 1,
		},
		{
			name:      "persistence failure retains acknowledged remap for caller retry",
			config:    config,
			ack:       true,
			saveErr:   errors.New("disk full"),
			wantCode:  PersistenceFailed,
			wantCalls: 1,
			wantSaves: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatalf("NewProfileRegistry() error = %v", err)
			}
			source := &mutableInventorySource{candidates: []transport.Candidate{valid}}
			command := &remapCommandFake{ack: tt.ack}
			persistence := &remapPersistenceFake{err: tt.saveErr}
			service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
				AttachInventory(mouse.NewTargetedService(registry, source, command))
			service.remapPersistence = persistence
			selected := service.RefreshInventory(context.Background()).Selected
			if tt.setup != nil {
				tt.setup(service, source)
			}

			before := service.GetRemapSnapshot()
			got := service.ApplyRemap(tt.config)

			if got.Error.Code != tt.wantCode {
				t.Fatalf("ApplyRemap() error = %q, want %q", got.Error.Code, tt.wantCode)
			}
			if command.calls != tt.wantCalls || persistence.saves != tt.wantSaves {
				t.Fatalf("ApplyRemap() writes=%d saves=%d; want writes=%d saves=%d", command.calls, persistence.saves, tt.wantCalls, tt.wantSaves)
			}
			if tt.wantCode == ApplyFailed && !remapConfigsEqual(got.Applied, before.Applied) {
				t.Fatalf("ApplyRemap() applied = %#v, want unchanged after failed ACK", got.Applied)
			}
			if tt.wantCalls == 1 && (selected == nil || command.binding != *selected) {
				t.Fatalf("ApplyRemap() binding = %#v, want selected binding %#v", command.binding, selected)
			}
			if tt.wantCode == PersistenceFailed && (!remapConfigsEqual(got.Applied, tt.config) || !got.RetryAvailable) {
				t.Fatalf("ApplyRemap() = %#v, want acknowledged state with retryable persistence failure", got)
			}
		})
	}
}

type remapCommandFake struct {
	calls   int
	ack     bool
	binding mouse.Binding
}

func (f *remapCommandFake) SendAndAwaitBound(_ context.Context, binding mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.calls++
	f.binding = binding
	if !f.ack {
		return errors.New("ack missing")
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0x00, 0x08})
	return nil
}

type remapPersistenceFake struct {
	failFirst bool
	saves     int
	err       error
}

func (*remapPersistenceFake) Load(Binding) (x6.DeviceConfig, error) {
	return x6.DefaultDeviceConfig(), nil
}

func (f *remapPersistenceFake) Save(Binding, x6.DeviceConfig) error {
	f.saves++
	if f.err != nil {
		return f.err
	}
	if f.failFirst && f.saves == 1 {
		return errors.New("disk full")
	}
	return nil
}
