package desktop

import (
	"bytes"
	"context"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
	"testing"
)

// The runner pauses inside the reset critical section; retries submitted there
// must observe reconciliation, not resurrect pre-purge recovery values.
func TestResetSerializesSettingsPersistenceRetries(t *testing.T) {
	for _, normal := range []bool{true, false} {
		t.Run(map[bool]string{true: "sleep", false: "debounce"}[normal], func(t *testing.T) {
			s, _ := newResetRuntime(t, t.TempDir(), &resetRecordingCommand{})
			st := s.currentSettingsState()
			minutes, ms := 10.0, 20
			st.retryNormal, st.retryResponse = &minutes, &ms
			entered, release := make(chan struct{}), make(chan struct{})
			s.AttachResetRunner(&pausedResetRunner{entered: entered, release: release})
			resetDone := make(chan struct{})
			go func() { defer close(resetDone); s.ResetToFactory(context.Background()) }()
			<-entered
			if s.operationMu.TryLock() {
				s.operationMu.Unlock()
				close(release)
				<-resetDone
				t.Fatal("reset did not hold operation lock")
			}
			retryStarted, retryDone := make(chan struct{}), make(chan struct{})
			go func() {
				close(retryStarted)
				defer close(retryDone)
				if normal {
					s.RetryNormalSleepPersistence()
				} else {
					s.RetryDebouncePersistence()
				}
			}()
			<-retryStarted
			close(release)
			<-resetDone
			<-retryDone
			if sleep, debounce := s.GetNormalSleepSnapshot(), s.GetDebounceSnapshot(); sleep.RetryAvailable || debounce.RetryAvailable || sleep.Persisted != nil || debounce.Persisted != nil {
				t.Fatalf("retry resurrected reset state: %#v %#v", sleep, debounce)
			}
		})
	}
}

func TestResetSerializesSettingsAndLightingApplies(t *testing.T) {
	for _, lane := range []string{"sleep", "debounce", "lighting"} {
		t.Run(lane, func(t *testing.T) {
			command := &capturedApplyCommand{}
			s := newCapturedApplyRuntime(t, command)
			s.StageNormalSleep(10)
			s.StageDebounce(20)
			entered, release := make(chan struct{}), make(chan struct{})
			s.AttachResetRunner(&pausedResetRunner{entered: entered, release: release})
			resetDone, applyDone := make(chan struct{}), make(chan struct{})
			go func() { defer close(resetDone); s.ResetToFactory(context.Background()) }()
			<-entered
			started := make(chan struct{})
			go func() {
				close(started)
				defer close(applyDone)
				switch lane {
				case "sleep":
					s.ApplyNormalSleep()
				case "debounce":
					s.ApplyDebounce()
				case "lighting":
					s.ApplyLighting()
				}
			}()
			<-started
			close(release)
			<-resetDone
			<-applyDone
			if len(command.reports) != 1 || !bytes.Equal(command.reports[0], capturedResetBytes(t, "050f015003a800ff00010401ff")) {
				t.Fatalf("post-reset %s reports: %x", lane, command.reports)
			}
		})
	}
}

func newCapturedApplyRuntime(t *testing.T, command *capturedApplyCommand) *Service {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "reset", Path: "/dev/hidraw-reset"}
	inventory := mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command)
	s := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(inventory)
	s.RefreshInventory(context.Background())
	return s
}

type capturedApplyCommand struct{ reports [][]byte }

func (c *capturedApplyCommand) SendAndAwaitBound(_ context.Context, _ mouse.Binding, report []byte, ack func([]byte) bool) error {
	c.reports = append(c.reports, append([]byte(nil), report...))
	ack([]byte{3, 0x10, 0x50, 0, report[0]})
	return nil
}

type pausedResetRunner struct{ entered, release chan struct{} }

func (r *pausedResetRunner) Run(context.Context) (ResetResult, error) {
	close(r.entered)
	<-r.release
	return ResetResult{Cleanup: ResetLaneResult{State: "success"}}, nil
}
