package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestServiceResetToFactoryDelegatesTypedResult(t *testing.T) {
	runner := &resetRunnerFake{result: ResetResult{Cleanup: ResetLaneResult{State: "success"}}}
	service := New(nil, nil, configstore.New(t.TempDir()+"/applied.json", t.TempDir()+"/factory.json")).AttachResetRunner(runner)

	result := service.ResetToFactory(context.Background())

	if runner.calls != 1 || result.Cleanup.State != "success" || result.RetryAvailable {
		t.Fatalf("ResetToFactory() = %#v; calls=%d", result, runner.calls)
	}
}

func TestServiceResetToFactoryPreservesRetryableFailure(t *testing.T) {
	runner := &resetRunnerFake{
		result: ResetResult{Error: Error{Code: ApplyFailed}, RetryAvailable: true},
		err:    errors.New("ack timeout"),
	}
	service := New(nil, nil, configstore.New(t.TempDir()+"/applied.json", t.TempDir()+"/factory.json")).AttachResetRunner(runner)

	result := service.ResetToFactory(context.Background())

	if runner.calls != 1 || result.Error.Code != ApplyFailed || !result.RetryAvailable {
		t.Fatalf("ResetToFactory() = %#v; calls=%d", result, runner.calls)
	}
}

func TestServiceResetToFactoryReconcilesOnlySuccessfulRun(t *testing.T) {
	service := New(nil, nil, configstore.New(t.TempDir()+"/applied.json", t.TempDir()+"/factory.json")).AttachResetRunner(
		&resetRunnerFake{result: ResetResult{Cleanup: ResetLaneResult{State: "success"}}},
	)
	staged := service.GetSnapshot().Pending
	staged.DPI[0] = 1200
	service.StageDPI(staged)

	service.ResetToFactory(context.Background())

	snapshot := service.GetSnapshot()
	want := ToDTO(x6.DocumentedResetDPIConfig())
	if snapshot.Applied != want || snapshot.Pending != want {
		t.Fatalf("snapshot after successful reset = %#v, want documented defaults", snapshot)
	}
}

func TestResetRuntimeQuiescesLateCallbacksBeforeResetWrites(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	command := &resetRecordingCommand{}
	service, scheduler := newResetRuntime(t, stateDir, command)
	binding, ok := service.selectedBinding()
	if !ok {
		t.Fatal("selected binding missing")
	}

	dpi := service.GetSnapshot().Pending
	dpi.DPI[0] = 1600
	dpiSnapshot := service.StageDPI(dpi)
	if err := service.sync.ScheduleAt(binding, dpiSnapshot.Revision, fromDTO(dpi)); err != nil {
		t.Fatalf("ScheduleAt(DPI) error = %v", err)
	}
	polling := service.StagePollingRate(x6.PollingRate500)
	if err := service.pollingSync.ScheduleAt(binding, polling.Revision, x6.PollingRate500); err != nil {
		t.Fatalf("ScheduleAt(polling) error = %v", err)
	}

	result := service.ResetToFactory(context.Background())
	if result.Cleanup.State != "success" || command.calls != 3 {
		t.Fatalf("ResetToFactory() = %#v, writes=%d; want three acknowledged reset lanes", result, command.calls)
	}
	scheduler.FireAll()
	if command.calls != 3 {
		t.Fatalf("late callbacks made %d writes, want only three reset lanes", command.calls)
	}
}

func TestResetRuntimeRestartRequiresFreshConfirmedResetAfterFailure(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateDir, "recovery.json")
	if err := os.WriteFile(statePath, []byte("recovery"), 0o600); err != nil {
		t.Fatal(err)
	}

	failingCommand := &resetRecordingCommand{failAt: 2}
	failed, _ := newResetRuntime(t, stateDir, failingCommand)
	result := failed.ResetToFactory(context.Background())
	if result.Error.Code != ApplyFailed || !result.RetryAvailable || failingCommand.calls != 2 {
		t.Fatalf("failed ResetToFactory() = %#v, writes=%d", result, failingCommand.calls)
	}
	if contents, err := os.ReadFile(statePath); err != nil || string(contents) != "recovery" {
		t.Fatalf("recovery state after failed reset = %q, %v", contents, err)
	}

	restartedCommand := &resetRecordingCommand{}
	restarted, _ := newResetRuntime(t, stateDir, restartedCommand)
	if inventory := restarted.RefreshInventory(context.Background()); inventory.Selected == nil || restartedCommand.calls != 0 {
		t.Fatalf("restart inventory = %#v, writes=%d; want reselection without writes", inventory, restartedCommand.calls)
	}
	if contents, err := os.ReadFile(statePath); err != nil || string(contents) != "recovery" {
		t.Fatalf("recovery state before fresh confirmation = %q, %v", contents, err)
	}

	result = restarted.ResetToFactory(context.Background())
	if result.Cleanup.State != "success" || restartedCommand.calls != 3 {
		t.Fatalf("fresh ResetToFactory() = %#v, writes=%d", result, restartedCommand.calls)
	}
	if entries, err := os.ReadDir(stateDir); err != nil || len(entries) != 0 {
		t.Fatalf("state after fresh confirmation = %#v, %v; want purged state", entries, err)
	}
}

func newResetRuntime(t *testing.T, stateDir string, command *resetRecordingCommand) (*Service, *lateScheduler) {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "reset", Path: "/dev/hidraw-reset"}
	inventory := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command)
	scheduler := &lateScheduler{}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(inventory)
	service.sync = NewSyncCoordinator(scheduler, service.bindingCurrent, service.applyBound)
	service.pollingSync = NewPollingSyncCoordinator(scheduler, service.bindingCurrent, service.applyPollingBound)
	service.RefreshInventory(context.Background())
	service.AttachResetRunner(NewEmergencyReset(inventory, service, configstore.NewStatePurger(stateDir)))
	return service, scheduler
}

type resetRecordingCommand struct {
	calls  int
	failAt int
}

func (c *resetRecordingCommand) SendAndAwaitBound(_ context.Context, _ mouse.Binding, report []byte, continueReading func([]byte) bool) error {
	c.calls++
	if c.failAt == c.calls {
		return errors.New("ack failed")
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0x00, report[0]})
	return nil
}

type lateScheduler struct{ callbacks []func() }

func (s *lateScheduler) After(_ time.Duration, callback func()) SyncCancel {
	s.callbacks = append(s.callbacks, callback)
	return func() {}
}

func (s *lateScheduler) FireAll() {
	for _, callback := range s.callbacks {
		callback()
	}
}

type resetRunnerFake struct {
	result ResetResult
	err    error
	calls  int
}

func (r *resetRunnerFake) Run(context.Context) (ResetResult, error) {
	r.calls++
	return r.result, r.err
}
