package x6

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestListenDeliversHeartbeatAndStageEvents(t *testing.T) {
	tr := &listenTransport{
		candidates: []transport.Candidate{{Path: "1:1-4", Connection: transport.Dongle}},
		reports: [][]byte{
			{0x03, 0x10, 0x40, 0x01, 0x0a},
			{0x03, 0x10, 0x10, 0x03, 0x00},
		},
	}
	service := NewService(tr)

	var (
		events   []StatusEvent
		eventsMu sync.Mutex
	)
	eventReady := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- service.Listen(ctx, func(event StatusEvent) {
			eventsMu.Lock()
			events = append(events, event)
			eventsMu.Unlock()
			eventReady <- struct{}{}
		})
	}()

	for range 2 {
		select {
		case <-eventReady:
		case <-time.After(2 * time.Second):
			eventsMu.Lock()
			got := append([]StatusEvent(nil), events...)
			eventsMu.Unlock()
			t.Fatalf("events = %+v; want heartbeat and stage event", got)
		}
	}
	eventsMu.Lock()
	gotEvents := append([]StatusEvent(nil), events...)
	eventsMu.Unlock()

	if gotEvents[0].Connection != transport.Dongle || !gotEvents[0].BatteryAvailable || gotEvents[0].BatteryPercent != 100 {
		t.Fatalf("events[0] = %+v; want dongle heartbeat battery 100", gotEvents[0])
	}
	if !gotEvents[1].StageAvailable || gotEvents[1].ActiveStage != 3 {
		t.Fatalf("events[1] = %+v; want DPI stage event stage 3", gotEvents[1])
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Listen() error = %v; want clean stop on cancel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Listen() did not return after context cancellation")
	}
}

func TestListenRecoversWhenDeviceAppears(t *testing.T) {
	tr := newListenTransport(errors.New("no device yet"))
	service := NewService(tr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Listen(ctx, func(StatusEvent) {}) }()

	select {
	case <-tr.enumerated:
	case <-time.After(time.Second):
		t.Fatal("Listen() did not try to enumerate before the device appeared")
	}
	tr.setDevice(
		[]transport.Candidate{{Path: "1:1-4", Connection: transport.Dongle}},
		[][]byte{{0x03, 0x10, 0x40, 0x01, 0x05}},
	)

	select {
	case <-tr.read:
	case <-time.After(2 * time.Second):
		t.Fatal("Listen() did not resume reading after the device appeared")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Listen() error = %v; want clean stop on cancel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Listen() did not return after context cancellation")
	}
}

type listenTransport struct {
	mu         sync.RWMutex
	candidates []transport.Candidate
	reports    [][]byte
	err        error
	enumerated chan struct{}
	read       chan struct{}
}

func newListenTransport(err error) *listenTransport {
	return &listenTransport{
		err:        err,
		enumerated: make(chan struct{}, 1),
		read:       make(chan struct{}, 1),
	}
}

func (f *listenTransport) Enumerate(context.Context, transport.Match) ([]transport.Candidate, error) {
	f.mu.RLock()
	candidates := append([]transport.Candidate(nil), f.candidates...)
	err := f.err
	f.mu.RUnlock()
	select {
	case f.enumerated <- struct{}{}:
	default:
	}
	return candidates, err
}

func (f *listenTransport) setDevice(candidates []transport.Candidate, reports [][]byte) {
	f.mu.Lock()
	f.err = nil
	f.candidates = append([]transport.Candidate(nil), candidates...)
	f.reports = append([][]byte(nil), reports...)
	f.mu.Unlock()
}

func (f *listenTransport) ValidateDescriptor(context.Context, transport.Candidate, transport.InputDescriptor) (transport.InputSource, error) {
	return transport.InputSource{Path: "1:1-4"}, nil
}

func (f *listenTransport) ReadInterruptIN(ctx context.Context, _ transport.InputSource, use func([]byte) bool) error {
	f.mu.RLock()
	reports := append([][]byte(nil), f.reports...)
	f.mu.RUnlock()
	for _, report := range reports {
		use(report)
		select {
		case f.read <- struct{}{}:
		default:
		}
	}
	<-ctx.Done()
	return nil
}
