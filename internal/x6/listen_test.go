package x6

import (
	"context"
	"errors"
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

	var events []StatusEvent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Listen(ctx, func(event StatusEvent) { events = append(events, event) }) }()

	deadline := time.Now().Add(2 * time.Second)
	for len(events) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("events = %+v; want heartbeat and stage event", events)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if events[0].Connection != transport.Dongle || !events[0].BatteryAvailable || events[0].BatteryPercent != 100 {
		t.Fatalf("events[0] = %+v; want dongle heartbeat battery 100", events[0])
	}
	if !events[1].StageAvailable || events[1].ActiveStage != 3 {
		t.Fatalf("events[1] = %+v; want DPI stage event stage 3", events[1])
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
	tr := &listenTransport{err: errors.New("no device yet")}
	service := NewService(tr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Listen(ctx, func(StatusEvent) {}) }()

	time.Sleep(50 * time.Millisecond)
	tr.err = nil
	tr.candidates = []transport.Candidate{{Path: "1:1-4", Connection: transport.Dongle}}
	tr.reports = [][]byte{{0x03, 0x10, 0x40, 0x01, 0x05}}

	deadline := time.Now().Add(2 * time.Second)
	for tr.reads == 0 {
		if time.Now().After(deadline) {
			t.Fatal("Listen() did not resume reading after the device appeared")
		}
		time.Sleep(10 * time.Millisecond)
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
	candidates []transport.Candidate
	reports    [][]byte
	err        error
	reads      int
}

func (f *listenTransport) Enumerate(context.Context, transport.Match) ([]transport.Candidate, error) {
	return f.candidates, f.err
}

func (f *listenTransport) ValidateDescriptor(context.Context, transport.Candidate, transport.InputDescriptor) (transport.InputSource, error) {
	return transport.InputSource{Path: "1:1-4"}, nil
}

func (f *listenTransport) ReadInterruptIN(ctx context.Context, _ transport.InputSource, use func([]byte) bool) error {
	for _, report := range f.reports {
		f.reads++
		use(report)
	}
	<-ctx.Done()
	return nil
}
