package desktop

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/hidlinux"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

func TestEmergencyResetPurgesOnlyAfterAllLanesSucceed(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}}
	purger := &purgerFake{}
	result, err := NewEmergencyReset(testModel(), target, nil, purger).Run(context.Background())
	if err != nil || !purger.called || result.Cleanup.State != "success" || len(target.operations) != 4 {
		t.Fatalf("Run() = %#v, %v; writes=%d purge=%t", result, err, len(target.operations), purger.called)
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

func TestEmergencyResetStopsOnPhysicalFailureWithoutPurge(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}, failAt: 2}
	purger := &purgerFake{}
	result, err := NewEmergencyReset(testModel(), target, nil, purger).Run(context.Background())
	if err == nil || purger.called || result.Lanes[0].State != "success" || result.Lanes[1].State != "failed" || result.Lanes[2].State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v; purge=%t", result, err, purger.called)
	}
}

func TestEmergencyResetRequiresSingleSelectedTarget(t *testing.T) {
	result, err := NewEmergencyReset(testModel(), &resetTargetFake{}, nil, &purgerFake{}).Run(context.Background())
	if err == nil || result.Cleanup.State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}

func TestEmergencyResetRejectsAmbiguousDiscoveryWithoutWritesOrPurge(t *testing.T) {
	target := &resetTargetFake{
		binding: testBinding("reset"),
		devices: []mouse.Device{{Eligible: true}, {Eligible: true}},
	}
	purger := &purgerFake{}

	result, err := NewEmergencyReset(testModel(), target, nil, purger).Run(context.Background())
	if err == nil || len(target.operations) != 0 || purger.called || result.Lanes[0].State != "not_attempted" || result.Cleanup.State != "not_attempted" {
		t.Fatalf("Run() = %#v, %v; writes=%d purge=%t", result, err, len(target.operations), purger.called)
	}
}

func TestEmergencyResetFailsWhenCleanupFailsAfterEveryLane(t *testing.T) {
	target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}}
	purger := &purgerFake{err: errors.New("disk full")}

	result, err := NewEmergencyReset(testModel(), target, nil, purger).Run(context.Background())
	if err == nil || !purger.called || len(target.operations) != 4 || result.Cleanup.State != "failed" || result.Cleanup.Code != PersistenceFailed {
		t.Fatalf("Run() = %#v, %v; writes=%d purge=%t", result, err, len(target.operations), purger.called)
	}
}

func TestEmergencyResetReportsRetryableCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewEmergencyReset(testModel(), &resetTargetFake{}, nil, &purgerFake{}).Run(ctx)
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
			stateDir := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(stateDir, 0o700); err != nil {
				t.Fatal(err)
			}
			seedPath := filepath.Join(stateDir, "recovery.json")
			seed := []byte("recovery state")
			if err := os.WriteFile(seedPath, seed, 0o600); err != nil {
				t.Fatal(err)
			}
			target := &resetTargetFake{binding: testBinding("reset"), devices: []mouse.Device{{Eligible: true}}, failAt: failAt}

			result, err := NewEmergencyReset(testModel(), target, nil, configstore.NewStatePurger(stateDir)).Run(context.Background())
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
			got, readErr := os.ReadFile(seedPath)
			if readErr != nil || !bytes.Equal(got, seed) {
				t.Fatalf("recovery state = %q, %v; want byte-identical seeded state", got, readErr)
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
	purger := &purgerFake{}

	result, err := NewEmergencyReset(testModel(), target, nil, purger).Run(context.Background())
	if !errors.Is(err, hidlinux.ErrDeviceDisconnected) || len(target.operations) != 0 || purger.called || !result.RetryAvailable {
		t.Fatalf("Run() = %#v, %v; writes=%d purge=%t", result, err, len(target.operations), purger.called)
	}
}

func TestEmergencyResetCanRetryAfterReselection(t *testing.T) {
	target := &resetTargetFake{devices: []mouse.Device{{Eligible: true}}}
	purger := &purgerFake{}
	reset := NewEmergencyReset(testModel(), target, nil, purger)

	if _, err := reset.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil without a selected binding")
	}
	target.binding = testBinding("reselected")
	result, err := reset.Run(context.Background())
	if err != nil || !purger.called || len(target.operations) != 4 || result.Cleanup.State != "success" {
		t.Fatalf("Run() after reselection = %#v, %v; writes=%d purge=%t", result, err, len(target.operations), purger.called)
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

type purgerFake struct {
	called bool
	err    error
}

func (f *purgerFake) PurgeAll() error { f.called = true; return f.err }

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
