package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type gatedSettingsCommand struct {
	entered chan struct{}
	release chan struct{}
	reports chan []byte
}

func (c *gatedSettingsCommand) SendAndAwaitBound(_ context.Context, _ mouse.Binding, report []byte, continueReading func([]byte) bool) error {
	if c.reports != nil {
		c.reports <- append([]byte(nil), report...)
	}
	close(c.entered)
	<-c.release
	continueReading([]byte{0x03, 0x10, 0x50, 0, 0x04})
	return nil
}

func TestSleepApplyUsesLightingForCapturedBinding(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidates := []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta", Path: "/dev/hidraw1"},
	}
	command := &gatedSettingsCommand{entered: make(chan struct{}), release: make(chan struct{}), reports: make(chan []byte, 2)}
	close(command.release)
	target := mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command)
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(target)
	devices := s.RefreshInventory(context.Background()).Devices
	a, b := devices[0].ID, devices[1].ID
	s.SelectDevice(a)
	s.StageLighting(x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonOne})
	s.SelectDevice(b)
	s.StageLighting(x6.LightingSelection{Mode: x6.LightingColorBreathing, TemplateID: x6.LightingTemplateColorBreathingOne})
	s.SelectDevice(a)
	// The first write establishes A's expected wire vector without relying on packet layout.
	readReport := func() []byte {
		t.Helper()
		select {
		case report := <-command.reports:
			return report
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for lighting settings report")
			return nil
		}
	}
	s.ApplyNormalSleep()
	want := readReport()
	command.entered = make(chan struct{})
	s.lightingComponent.beforeSettingsLightingRead = func() {
		if err := target.Select(b); err != nil {
			t.Fatal(err)
		}
	}
	s.lightingComponent.afterSettingsLightingRead = func() {
		if err := target.Select(a); err != nil {
			t.Fatal(err)
		}
	}
	s.ApplyNormalSleep()
	got := readReport()
	if string(got) != string(want) {
		t.Fatalf("A binding paired with B lighting: got report %v, want A report %v", got, want)
	}
}

func TestSettingsApplyKeepsOriginalBindingAcrossSelection(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidates := []transport.Candidate{
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"},
		{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta", Path: "/dev/hidraw1"},
	}
	command := &gatedSettingsCommand{entered: make(chan struct{}), release: make(chan struct{})}
	records := map[DeviceID]x6.DeviceConfig{}
	target := mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command)
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(target).AttachNormalSleepPersistence(
		func(b Binding) (x6.DeviceConfig, error) { return records[b.ID], nil },
		func(b Binding, c x6.DeviceConfig) error { records[b.ID] = c; return nil },
	)
	inventory := s.RefreshInventory(context.Background())
	first := inventory.Devices[0].ID
	s.SelectDevice(first)
	var second DeviceID
	for _, device := range inventory.Devices {
		if device.ID != first {
			second = device.ID
		}
	}
	if second == (DeviceID{}) {
		t.Fatal("second device missing")
	}
	s.StageNormalSleep(12)
	applied := make(chan NormalSleepSnapshot, 1)
	go func() { applied <- s.ApplyNormalSleep() }()
	<-command.entered
	if err := target.Select(second); err != nil {
		t.Fatal(err)
	}
	close(command.release)
	result := <-applied
	if result.Error.Code != "" || records[first].NormalSleepMinutes != 12 || records[second].NormalSleepMinutes == 12 {
		t.Fatalf("apply = %#v; persisted first = %#v, second = %#v", result, records[first], records[second])
	}
}

func TestSettingsApplyPersistsCapturedValueDuringConcurrentStage(t *testing.T) {
	for _, normal := range []bool{true, false} {
		name := "debounce"
		if normal {
			name = "normal sleep"
		}
		t.Run(name, func(t *testing.T) {
			registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
			candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
			command := &gatedSettingsCommand{entered: make(chan struct{}), release: make(chan struct{}), reports: make(chan []byte, 1)}
			defer func() {
				select {
				case <-command.release:
				default:
					close(command.release)
				}
			}()
			wait := func(ch <-chan struct{}, label string) {
				t.Helper()
				select {
				case <-ch:
				case <-time.After(2 * time.Second):
					t.Fatalf("timed out waiting for %s", label)
				}
			}
			record := x6.DefaultDeviceConfig()
			s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command)).AttachNormalSleepPersistence(
				func(Binding) (x6.DeviceConfig, error) { return record, nil },
				func(_ Binding, c x6.DeviceConfig) error { record = c; return nil },
			)
			s.RefreshInventory(context.Background())
			done := make(chan struct{})
			if normal {
				s.StageNormalSleep(12)
				go func() { s.ApplyNormalSleep(); close(done) }()
			} else {
				s.StageDebounce(2)
				go func() { s.ApplyDebounce(); close(done) }()
			}
			wait(command.entered, "firmware command entry")
			var report []byte
			select {
			case report = <-command.reports:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for firmware report")
			}
			if normal && report[9] != 24 || !normal && report[10] != 1 {
				t.Fatalf("firmware write = %v; want encoded sleep 12 or response 2", report)
			}
			stageStarted := make(chan struct{})
			staged := make(chan struct{})
			go func() {
				close(stageStarted)
				if normal {
					s.StageNormalSleep(18)
				} else {
					s.StageDebounce(4)
				}
				close(staged)
			}()
			wait(stageStarted, "stage goroutine start")
			// The handshake confirms the goroutine started, but cannot prove it entered Stage without a production hook.
			// Give an unlocked stage a bounded opportunity to complete before releasing the firmware write.
			select {
			case <-staged:
			case <-time.After(20 * time.Millisecond):
			}
			close(command.release)
			wait(done, "apply completion")
			wait(staged, "stage completion")
			if normal {
				snapshot := s.GetNormalSleepSnapshot()
				if record.NormalSleepMinutes != 12 || snapshot.Pending != 18 || snapshot.Firmware != "pending" {
					t.Fatalf("persisted sleep = %v, snapshot = %#v; want applied 12 and pending 18", record.NormalSleepMinutes, snapshot)
				}
			} else {
				snapshot := s.GetDebounceSnapshot()
				if record.ResponseTimeMs != 2 || snapshot.Desired != 4 || snapshot.Firmware != "pending" {
					t.Fatalf("persisted response = %v, snapshot = %#v; want applied 2 and pending 4", record.ResponseTimeMs, snapshot)
				}
			}
		})
	}
}

func TestSettingsPersistencePreservesOtherFieldsAndReloadsSelection(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	record := x6.DefaultDeviceConfig()
	record.PollingRate = x6.PollingRate500
	record.Remap = &x6.RemapConfig{}
	load := func(Binding) (x6.DeviceConfig, error) { return record, nil }
	save := func(_ Binding, c x6.DeviceConfig) error { record = c; return nil }
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, &fakeHidrawCommand{})).AttachNormalSleepPersistence(load, save)
	inventory := s.RefreshInventory(context.Background())
	s.StageNormalSleep(12)
	s.ApplyNormalSleep()
	if record.PollingRate != x6.PollingRate500 || record.Remap == nil {
		t.Fatalf("settings save discarded unrelated fields: %#v", record)
	}
	record.NormalSleepMinutes = 18
	s.SelectDevice(inventory.Devices[0].ID)
	if got := s.GetNormalSleepSnapshot().Pending; got != 18 {
		t.Fatalf("selected settings = %v, want reloaded 18", got)
	}
}

func TestSettingsRetryPreservesOriginalValueAndOtherDomains(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	record := x6.DefaultDeviceConfig()
	record.PollingRate = x6.PollingRate500
	saves := 0
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, &fakeHidrawCommand{})).AttachNormalSleepPersistence(
		func(Binding) (x6.DeviceConfig, error) { return record, nil },
		func(_ Binding, config x6.DeviceConfig) error {
			saves++
			if saves == 1 {
				return errors.New("disk full")
			}
			record = config
			return nil
		},
	)
	s.RefreshInventory(context.Background())
	s.StageNormalSleep(12)
	failed := s.ApplyNormalSleep()
	if !failed.RetryAvailable || failed.Error.Code != PersistenceFailed {
		t.Fatalf("apply = %#v, want retryable persistence failure", failed)
	}
	retried := s.RetryNormalSleepPersistence()
	if record.NormalSleepMinutes != 12 || record.PollingRate != x6.PollingRate500 || retried.RetryAvailable || retried.Error.Code != "" || retried.Persisted == nil || *retried.Persisted != 12 {
		t.Fatalf("retry = %#v, record = %#v; want original sleep and preserved polling", retried, record)
	}
}

func TestSelectingMissingSettingsRecordClearsStaleState(t *testing.T) {
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
	record := x6.DefaultDeviceConfig()
	record.NormalSleepMinutes = 18
	missing := false
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, &fakeHidrawCommand{})).AttachNormalSleepPersistence(func(Binding) (x6.DeviceConfig, error) {
		if missing {
			return x6.DeviceConfig{}, os.ErrNotExist
		}
		return record, nil
	}, func(Binding, x6.DeviceConfig) error { return nil })
	inventory := s.RefreshInventory(context.Background())
	if got := s.GetNormalSleepSnapshot().Pending; got != 18 {
		t.Fatalf("initial sleep = %v", got)
	}
	missing = true
	s.SelectDevice(inventory.Devices[0].ID)
	snapshot := s.GetNormalSleepSnapshot()
	if snapshot.Pending != x6.NormalSleepMinimumMinutes || snapshot.Persisted != nil {
		t.Fatalf("missing record retained stale settings: %#v", snapshot)
	}
}

func TestNormalSleepAndDebounceClassifyDeviceAccessFailures(t *testing.T) {
	for _, tt := range []struct {
		name  string
		err   error
		apply func(*Service) ErrorCode
		want  ErrorCode
	}{
		{name: "normal sleep permission denied", err: os.ErrPermission, apply: func(s *Service) ErrorCode { return s.ApplyNormalSleep().Error.Code }, want: PermissionDenied},
		{name: "normal sleep wrapped disconnected", err: fmt.Errorf("write: %w", os.ErrNotExist), apply: func(s *Service) ErrorCode { return s.ApplyNormalSleep().Error.Code }, want: DeviceDisconnected},
		{name: "debounce permission denied", err: os.ErrPermission, apply: func(s *Service) ErrorCode { return s.ApplyDebounce().Error.Code }, want: PermissionDenied},
		{name: "debounce wrapped disconnected", err: fmt.Errorf("write: %w", os.ErrNotExist), apply: func(s *Service) ErrorCode { return s.ApplyDebounce().Error.Code }, want: DeviceDisconnected},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatal(err)
			}
			candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
			service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, &fakeHidrawCommand{err: tt.err}))
			service.RefreshInventory(context.Background())

			if got := tt.apply(service); got != tt.want {
				t.Fatalf("apply error code = %q, want %q", got, tt.want)
			}
		})
	}
}
