package desktop

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/hidlinux"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

func TestEmergencyResetPersistsOnlyAfterAllLanesSucceed(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}}
	persister := &persisterFake{}
	result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
	if err != nil || !persister.called || result.Cleanup.State != "success" || len(target.operations) != 4 {
		t.Fatalf("Run() = %#v, %v; writes=%d persist=%t", result, err, len(target.operations), persister.called)
	}
	if persister.binding != target.binding {
		t.Fatalf("persisted binding = %#v, want the reset target %#v", persister.binding, target.binding)
	}
	// The model encodes each documented reset value; the real X6 captured bytes
	// for these values are pinned in the X6 model tests.
	wantReports := [][]byte{
		mustEncodeDPI(t, device.DocumentedResetDPIConfig()),
		mustEncodeLightingSettings(t, device.DocumentedResetLightingSettings()),
		mustEncodePolling(t, device.PollingRate1000),
		mustEncodeRemap(t, device.DefaultRemapConfig()),
	}
	for index, want := range wantReports {
		if !bytes.Equal(target.reports[index], want) {
			t.Fatalf("reset report %d = %x, want %x", index, target.reports[index], want)
		}
	}
}

func TestEmergencyResetStopsOnPhysicalFailureWithoutPersisting(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}, failAt: 2}
	persister := &persisterFake{}
	result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
	if err == nil || persister.called || result.Lanes[0].State != "success" || result.Lanes[1].State != "failed" || result.Lanes[2].State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v; persist=%t", result, err, persister.called)
	}
}

func TestEmergencyResetRequiresSingleSelectedTarget(t *testing.T) {
	result, err := NewEmergencyReset(testModel(), &resetTargetFake{}, nil, &persisterFake{}).Run(context.Background())
	if err == nil || result.Cleanup.State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}

func TestEmergencyResetRejectsAmbiguousDiscoveryWithoutWritesOrPersisting(t *testing.T) {
	target := &resetTargetFake{
		binding: testBinding("reset"),
		devices: []mouse.Device{{Eligible: true}, {Eligible: true}},
	}
	persister := &persisterFake{}

	result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
	if err == nil || len(target.operations) != 0 || persister.called || result.Lanes[0].State != "not_attempted" || result.Cleanup.State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v; writes=%d persist=%t", result, err, len(target.operations), persister.called)
	}
}

func TestEmergencyResetFailsWhenCleanupFailsAfterEveryLane(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}}
	persister := &persisterFake{err: errors.New("disk full")}

	result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
	if err == nil || !persister.called || len(target.operations) != 4 || result.Cleanup.State != "failed" || result.Cleanup.Code != PersistenceFailed {
		t.Fatalf("Run() = %#v, %v; writes=%d persist=%t", result, err, len(target.operations), persister.called)
	}
}

func TestEmergencyResetReportsRetryableCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewEmergencyReset(testModel(), &resetTargetFake{}, nil, &persisterFake{}).Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if result.Error.Code != ApplyFailed || !result.RetryAvailable {
		t.Fatalf("Run() result = %#v, want retryable apply failure", result)
	}
}

func TestEmergencyResetReportsRetryableLaneFailures(t *testing.T) {
	for _, failAt := range []int{1, 2, 3, 4} {
		t.Run("lane failure", func(t *testing.T) {
			persister := &persisterFake{}
			target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}, failAt: failAt}

			result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
			if err == nil {
				t.Fatal("Run() error = nil, want failure")
			}
			if result.Error.Code != ApplyFailed || !result.RetryAvailable {
				t.Fatalf("Run() result = %#v, want retryable apply failure", result)
			}
			if len(target.operations) != failAt || result.Cleanup.State != "not_attempted" {
				t.Fatalf("failure lane %d: writes=%d cleanup=%#v", failAt, len(target.operations), result.Cleanup)
			}
			for index, lane := range result.Lanes {
				want := "not_attempted"
				if index < failAt-1 {
					want = "success"
				} else if index == failAt-1 {
					want = "failed"
				}
				if lane.State != want {
					t.Fatalf("lane %d state=%s want=%s", index, lane.State, want)
				}
			}
			if persister.called {
				t.Fatalf("failure lane %d persisted reset state", failAt)
			}
		})
	}
}

func TestEmergencyResetDisconnectPreservesRecoveryState(t *testing.T) {
	target := &resetTargetFake{
		binding:    testBinding("reset"),
		devices:    []mouse.Device{{Eligible: true}},
		refreshErr: hidlinux.ErrDeviceDisconnected,
	}
	persister := &persisterFake{}

	result, err := NewEmergencyReset(testModel(), target, nil, persister).Run(context.Background())
	if !errors.Is(err, hidlinux.ErrDeviceDisconnected) || len(target.operations) != 0 || persister.called || !result.RetryAvailable {
		t.Fatalf("Run() = %#v, %v; writes=%d persist=%t", result, err, len(target.operations), persister.called)
	}
}

func TestEmergencyResetCanRetryAfterReselection(t *testing.T) {
	target := &resetTargetFake{devices: []mouse.Device{{Eligible: true}}}
	persister := &persisterFake{}
	reset := NewEmergencyReset(testModel(), target, nil, persister)

	if _, err := reset.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil without a selected binding")
	}
	target.binding = testBinding("reselected")
	result, err := reset.Run(context.Background())
	if err != nil || !persister.called || len(target.operations) != 4 || result.Cleanup.State != "success" {
		t.Fatalf("Run() after reselection = %#v, %v; writes=%d persist=%t", result, err, len(target.operations), persister.called)
	}
}

type resetTargetFake struct {
	binding    Binding
	devices    []mouse.Device
	operations []any
	reports    [][]byte
	failAt     int
	refreshErr error
}

func (f *resetTargetFake) Refresh(context.Context) ([]mouse.Device, error) {
	return f.devices, f.refreshErr
}
func (f *resetTargetFake) Selection() (mouse.Binding, bool) { return f.binding, f.binding.Path != "" }
func (f *resetTargetFake) ApplyOperationBound(_ context.Context, _ mouse.Binding, operation mouse.CommandOperation, value any) error {
	f.operations = append(f.operations, value)
	report, err := operation.Encode(value)
	if err != nil {
		return err
	}
	f.reports = append(f.reports, report)
	if f.failAt == len(f.operations) {
		return errors.New("ack failed")
	}
	return nil
}

type persisterFake struct {
	called  bool
	binding Binding
	err     error
}

func (f *persisterFake) PersistReset(binding Binding) error {
	f.called, f.binding = true, binding
	return f.err
}

func mustEncodeDPI(t *testing.T, config device.DPIConfig) []byte {
	t.Helper()
	report, err := stubEncodeDPI(config)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func mustEncodeLightingSettings(t *testing.T, settings device.LightingSettings) []byte {
	t.Helper()
	report, err := testModel().LightingSettingsOperation().Encode(settings)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func mustEncodePolling(t *testing.T, rate device.PollingRate) []byte {
	t.Helper()
	report, err := testModel().PollingOperation().Encode(rate)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func mustEncodeRemap(t *testing.T, config device.RemapConfig) []byte {
	t.Helper()
	report, err := testModel().RemapOperation().Encode(config)
	if err != nil {
		t.Fatal(err)
	}
	return report
}
