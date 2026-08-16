package desktop

import (
	"sync"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func TestSyncCoordinatorCoalescesEditsAfterOneSecond(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	writer := &recordingBoundWriter{}
	coordinator := NewSyncCoordinator(scheduler, func(Binding) bool { return true }, writer.Apply)
	binding := testBinding("A")

	first, err := coordinator.Schedule(binding, dpiConfig(800))
	if err != nil {
		t.Fatalf("Schedule(first) error = %v", err)
	}
	second, err := coordinator.Schedule(binding, dpiConfig(1600))
	if err != nil {
		t.Fatalf("Schedule(second) error = %v", err)
	}
	if second <= first {
		t.Fatalf("revisions = %d, %d; want monotonic increase", first, second)
	}
	if got := scheduler.Delay(1); got != syncDebounceDelay {
		t.Fatalf("second delay = %v, want %v", got, syncDebounceDelay)
	}

	scheduler.Advance(syncDebounceDelay - time.Millisecond)
	if got := len(writer.calls); got != 0 {
		t.Fatalf("writes before one second = %d, want 0", got)
	}
	scheduler.Advance(time.Millisecond)
	if got := len(writer.calls); got != 1 {
		t.Fatalf("writes after one second = %d, want 1", got)
	}
	if got := writer.calls[0]; got.revision != second || got.config.DPI[0] != 1600 {
		t.Fatalf("write = %#v, want newest revision %d and DPI 1600", got, second)
	}
}

func TestSyncCoordinatorKeepsBindingsAndRevisionsIsolated(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	writer := &recordingBoundWriter{}
	coordinator := NewSyncCoordinator(scheduler, func(Binding) bool { return true }, writer.Apply)
	bindingA, bindingB := testBinding("A"), testBinding("B")

	revisionA, err := coordinator.Schedule(bindingA, dpiConfig(800))
	if err != nil {
		t.Fatalf("Schedule(A) error = %v", err)
	}
	revisionB, err := coordinator.Schedule(bindingB, dpiConfig(2400))
	if err != nil {
		t.Fatalf("Schedule(B) error = %v", err)
	}
	if revisionA != 1 || revisionB != 1 {
		t.Fatalf("initial revisions = A:%d B:%d, want A:1 B:1", revisionA, revisionB)
	}

	scheduler.Advance(syncDebounceDelay)
	if got := len(writer.calls); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
	if writer.calls[0].binding != bindingA || writer.calls[0].config.DPI[0] != 800 {
		t.Fatalf("first write = %#v, want binding A at DPI 800", writer.calls[0])
	}
	if writer.calls[1].binding != bindingB || writer.calls[1].config.DPI[0] != 2400 {
		t.Fatalf("second write = %#v, want binding B at DPI 2400", writer.calls[1])
	}
}

func TestSyncCoordinatorKeepsSeriallessWorkInMemoryAndRejectsStaleCapturedBindings(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	writer := &recordingBoundWriter{}
	valid := true
	coordinator := NewSyncCoordinator(scheduler, func(Binding) bool { return valid }, writer.Apply)

	serialless := testBinding("")
	if _, err := coordinator.Schedule(serialless, dpiConfig(800)); err != nil {
		t.Fatalf("Schedule(serialless) error = %v", err)
	}
	scheduler.Advance(syncDebounceDelay)
	if got := len(writer.calls); got != 1 || writer.calls[0].binding != serialless {
		t.Fatalf("serialless calls = %#v; want one memory-only write", writer.calls)
	}

	binding := testBinding("A")
	if _, err := coordinator.Schedule(binding, dpiConfig(1600)); err != nil {
		t.Fatalf("Schedule(binding) error = %v", err)
	}
	valid = false
	scheduler.Advance(syncDebounceDelay)
	if got := len(writer.calls); got != 1 {
		t.Fatalf("writes for rejected captured binding = %d, want only the serialless write", got)
	}
}

func TestSyncCoordinatorCancelsTimerBeforeExpiry(t *testing.T) {
	scheduler := &fakeSyncScheduler{}
	writer := &recordingBoundWriter{}
	coordinator := NewSyncCoordinator(scheduler, func(Binding) bool { return true }, writer.Apply)
	binding := testBinding("A")

	if _, err := coordinator.Schedule(binding, dpiConfig(1600)); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	coordinator.Cancel(binding)
	scheduler.Advance(syncDebounceDelay)
	if got := len(writer.calls); got != 0 {
		t.Fatalf("writes after cancellation = %d, want 0", got)
	}
}

type fakeSyncScheduler struct {
	mu     sync.Mutex
	now    time.Duration
	timers []*fakeSyncTimer
}

type fakeSyncTimer struct {
	due      time.Duration
	canceled bool
	f        func()
}

func (s *fakeSyncScheduler) After(delay time.Duration, f func()) SyncCancel {
	s.mu.Lock()
	defer s.mu.Unlock()
	timer := &fakeSyncTimer{due: s.now + delay, f: f}
	s.timers = append(s.timers, timer)
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		timer.canceled = true
	}
}

func (s *fakeSyncScheduler) Advance(delay time.Duration) {
	s.mu.Lock()
	s.now += delay
	var expired []func()
	for _, timer := range s.timers {
		if !timer.canceled && timer.due <= s.now {
			timer.canceled = true
			expired = append(expired, timer.f)
		}
	}
	s.mu.Unlock()
	for _, callback := range expired {
		callback()
	}
}

func (s *fakeSyncScheduler) Delay(index int) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.timers[index].due - s.now
}
func (s *fakeSyncScheduler) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.timers)
}

type recordedBoundWrite struct {
	binding  Binding
	revision uint64
	config   x6.DPIConfig
}

type recordingBoundWriter struct{ calls []recordedBoundWrite }

func (w *recordingBoundWriter) Apply(binding Binding, revision uint64, config x6.DPIConfig) error {
	w.calls = append(w.calls, recordedBoundWrite{binding, revision, config})
	return nil
}

func testBinding(serial string) Binding {
	return Binding{ID: DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: serial}, ProfileID: "x6", Path: "/dev/hidraw0", InventoryRevision: 1}
}

func dpiConfig(dpi int) x6.DPIConfig {
	config := x6.DefaultDPIConfig()
	config.DPI[0] = dpi
	return config
}
