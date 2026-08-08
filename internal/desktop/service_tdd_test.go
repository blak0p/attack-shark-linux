package desktop

import (
	"context"
	"errors"
	"testing"

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
