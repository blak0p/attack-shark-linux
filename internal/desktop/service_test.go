package desktop

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type fakeStatus struct {
	status x6.Status
	err    error
}

func (f fakeStatus) Status(context.Context) (x6.Status, error) { return f.status, f.err }

type fakeApply struct {
	calls  int
	config x6.DPIConfig
	err    error
}

func (f *fakeApply) ApplyAndPersist(_ context.Context, config x6.DPIConfig, _ x6.AppliedDPIStore) error {
	f.calls++
	f.config = config
	return f.err
}

type fakeStore struct {
	applied x6.DPIConfig
	loadErr error
}

func (f fakeStore) LoadApplied() (x6.DPIConfig, error) { return f.applied, f.loadErr }
func (fakeStore) SaveApplied(x6.DPIConfig) error       { return nil }

func TestRefreshStatusMapsConnectedBatteryAndEquivalentErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		reader  fakeStatus
		want    ErrorCode
		battery *int
	}{
		{"connected battery", fakeStatus{status: x6.Status{Connection: x6.Dongle, BatteryAvailable: true, BatteryPercent: 84}}, "", intPointer(84)},
		{"no device", fakeStatus{err: &x6.ServiceError{Kind: x6.NoUsableDevice, Err: errors.New("absent")}}, DeviceUnavailable, nil},
		{"permission", fakeStatus{err: os.ErrPermission}, PermissionDenied, nil},
		{"read", fakeStatus{err: &x6.ServiceError{Kind: x6.ReadFailure, Err: errors.New("read")}}, StatusReadFailed, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := New(tt.reader, &fakeApply{}, fakeStore{applied: x6.DefaultDPIConfig()})
			snapshot := service.RefreshStatus(context.Background())
			if snapshot.Error.Code != tt.want || !sameInt(snapshot.Battery, tt.battery) {
				t.Fatalf("snapshot = %#v, want error %q and battery %v", snapshot, tt.want, tt.battery)
			}
		})
	}
}

func TestStageAndApplyKeepsPendingUntilSuccessfulApply(t *testing.T) {
	applied := x6.DefaultDPIConfig()
	pending := applied
	pending.DPI[0] = 1600
	for _, tt := range []struct {
		name      string
		applyErr  error
		wantError ErrorCode
		wantSaved bool
	}{
		{"success", nil, "", true},
		{"ack failure", &x6.ServiceError{Kind: x6.AckFailure, Err: errors.New("no ack")}, ApplyFailed, false},
		{"persist failure", &x6.ServiceError{Kind: x6.PersistFailure, Err: errors.New("disk")}, PersistenceFailed, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := &fakeApply{err: tt.applyErr}
			service := New(fakeStatus{}, writer, fakeStore{applied: applied})
			if got := service.GetSnapshot(); got.Applied.DPI[0] != applied.DPI[0] {
				t.Fatalf("restored DPI = %d, want %d", got.Applied.DPI[0], applied.DPI[0])
			}
			if got := service.StageDPI(ToDTO(pending)); got.Error.Code != "" || writer.calls != 0 {
				t.Fatalf("stage = %#v, writes = %d; want staged without write", got, writer.calls)
			}
			got := service.ApplyDPI(context.Background())
			if got.Error.Code != tt.wantError || got.Applied.DPI[0] == pending.DPI[0] != tt.wantSaved || got.Pending.DPI[0] != pending.DPI[0] {
				t.Fatalf("apply = %#v, want error %q and saved %t", got, tt.wantError, tt.wantSaved)
			}
		})
	}
}

func TestApplySerializesWritesWithoutBlockingNewStagedRevision(t *testing.T) {
	applied := x6.DefaultDPIConfig()
	first := applied
	first.DPI[0] = 1600
	second := applied
	second.DPI[0] = 2400
	writer := newBlockingApply()
	service := New(fakeStatus{}, writer, fakeStore{applied: applied})

	if got := service.StageDPI(ToDTO(first)); got.Revision != 1 {
		t.Fatalf("first stage revision = %d, want 1", got.Revision)
	}
	firstApply := make(chan Snapshot, 1)
	go func() { firstApply <- service.ApplyDPI(context.Background()) }()
	<-writer.started

	staged := make(chan Snapshot, 1)
	go func() { staged <- service.StageDPI(ToDTO(second)) }()
	select {
	case got := <-staged:
		if got.Revision != 2 || got.Pending.DPI[0] != second.DPI[0] {
			t.Fatalf("second stage = %#v, want revision 2 with pending DPI %d", got, second.DPI[0])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("StageDPI blocked behind an in-flight ApplyDPI")
	}

	secondApply := make(chan Snapshot, 1)
	go func() { secondApply <- service.ApplyDPI(context.Background()) }()
	if got := writer.callCount(); got != 1 {
		t.Fatalf("concurrent applies = %d writer calls before release, want 1", got)
	}
	close(writer.release)

	if got := <-firstApply; got.Applied.DPI[0] != first.DPI[0] || got.Pending.DPI[0] != second.DPI[0] {
		t.Fatalf("first apply = %#v, want applied %d and pending %d", got, first.DPI[0], second.DPI[0])
	}
	if got := <-secondApply; got.Applied.DPI[0] != second.DPI[0] || got.Pending.DPI[0] != second.DPI[0] {
		t.Fatalf("second apply = %#v, want applied and pending DPI %d", got, second.DPI[0])
	}
	if got := writer.callCount(); got != 2 {
		t.Fatalf("serialized applies = %d writer calls, want 2", got)
	}
}

type blockingApply struct {
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func newBlockingApply() *blockingApply {
	return &blockingApply{started: make(chan struct{}), release: make(chan struct{})}
}

func (f *blockingApply) ApplyAndPersist(_ context.Context, _ x6.DPIConfig, _ x6.AppliedDPIStore) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call == 1 {
		close(f.started)
		<-f.release
	}
	return nil
}

func (f *blockingApply) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func intPointer(value int) *int { return &value }

func sameInt(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
