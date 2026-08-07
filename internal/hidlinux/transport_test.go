package hidlinux

import (
	"context"
	"errors"
	"github.com/alejandro/attack-shark-linux/internal/x6"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPassiveAdapterReadsWithoutOutgoingCalls(t *testing.T) {
	t.Run("validated input reads one report and stops", func(t *testing.T) {
		testAdapter(t, x6.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}, []string{"enumerate", "open", "read", "close"}, true)
	})
	t.Run("invalid endpoint is rejected before opening", func(t *testing.T) {
		testAdapter(t, x6.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 3}, []string{"enumerate"}, false)
	})
}
func TestPassiveAdapterReceiverReachesDonglePriority(t *testing.T) {
	backend := &backendSpy{report: []byte{3, 0x40, 0, 0, 5}}
	status, err := x6.NewService(newPassiveAdapter(backend)).Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Connection != x6.Dongle {
		t.Fatalf("connection = %q, want %q", status.Connection, x6.Dongle)
	}
	if !reflect.DeepEqual(backend.calls, []string{"enumerate", "open", "read", "close"}) {
		t.Fatalf("calls = %v, want dongle-priority read", backend.calls)
	}
}

func TestPassiveAdapterUsesTwoSecondInterruptReadTimeout(t *testing.T) {
	backend := &backendSpy{report: []byte{3, 0x40, 0, 0, 5}}
	transport := newPassiveAdapter(backend)
	candidates, err := transport.Enumerate(context.Background(), x6.Match{VendorID: 0x1D57, ProductID: 0xFA60})
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Enumerate() candidates = %d, want 1", len(candidates))
	}
	source, err := transport.ValidateDescriptor(context.Background(), candidates[0], x6.InputDescriptor{
		InterfaceNumber: 2,
		UsagePage:       1,
		Usage:           0x80,
		EndpointAddress: 0x83,
	})
	if err != nil {
		t.Fatalf("ValidateDescriptor() error = %v", err)
	}
	if err := transport.ReadInterruptIN(context.Background(), source, func([]byte) bool { return false }); err != nil {
		t.Fatalf("ReadInterruptIN() error = %v", err)
	}
	if backend.readTimeout != 2*time.Second {
		t.Fatalf("ReadWithTimeout() timeout = %v, want %v", backend.readTimeout, 2*time.Second)
	}
}

func TestPassiveAdapterToleratesIntermittentReadTimeouts(t *testing.T) {
	clock := &fakeClock{current: time.Unix(0, 0)}
	device := &readSequenceDevice{
		clock: clock,
		results: []readResult{
			{err: errors.New("timeout")},
			{err: errors.New("timeout")},
			{report: []byte{3, 0x40, 0, 0, 5}},
		},
	}
	adapter := passiveAdapter{
		passiveBackend: &deviceBackend{device: device},
		now:            clock.Now,
	}

	err := adapter.ReadInterruptIN(context.Background(), x6.InputSource{Path: "x6"}, func(report []byte) bool {
		return !reflect.DeepEqual(report, []byte{3, 0x40, 0, 0, 5})
	})
	if err != nil {
		t.Fatalf("ReadInterruptIN() error = %v", err)
	}
	if device.reads != 3 {
		t.Fatalf("reads = %d, want 3", device.reads)
	}
}

func TestPassiveAdapterReturnsAfterAggregateReadDeadline(t *testing.T) {
	clock := &fakeClock{current: time.Unix(0, 0)}
	device := &readSequenceDevice{clock: clock}
	adapter := passiveAdapter{
		passiveBackend: &deviceBackend{device: device},
		now:            clock.Now,
	}

	err := adapter.ReadInterruptIN(context.Background(), x6.InputSource{Path: "x6"}, func([]byte) bool { return false })
	if err == nil {
		t.Fatal("ReadInterruptIN() error = nil, want aggregate deadline error")
	}
	if !strings.Contains(err.Error(), "passive input read deadline exceeded") {
		t.Fatalf("ReadInterruptIN() error = %v, want clear deadline error", err)
	}
	if device.reads != 8 {
		t.Fatalf("reads = %d, want 8 reads across 15 seconds", device.reads)
	}
	if device.timeouts[len(device.timeouts)-1] != time.Second {
		t.Fatalf("final read timeout = %v, want 1s remaining deadline", device.timeouts[len(device.timeouts)-1])
	}
}

func testAdapter(t *testing.T, descriptor x6.InputDescriptor, want []string, read bool) {
	t.Helper()
	backend := &backendSpy{report: []byte{3, 0x40, 0, 0, 5}}
	transport := newPassiveAdapter(backend)
	candidates, err := transport.Enumerate(context.Background(), x6.Match{VendorID: 0x1D57, ProductID: 0xFA60})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("Enumerate() = %v, %v", candidates, err)
	}
	source, err := transport.ValidateDescriptor(context.Background(), candidates[0], descriptor)
	if !read && err == nil {
		t.Fatal("ValidateDescriptor() error = nil, want invalid endpoint error")
	}
	if read && err != nil {
		t.Fatal(err)
	}
	if read && transport.ReadInterruptIN(context.Background(), source, func([]byte) bool { return false }) != nil {
		t.Fatal("ReadInterruptIN failed")
	}
	if !reflect.DeepEqual(backend.calls, want) {
		t.Fatalf("calls = %v, want %v", backend.calls, want)
	}
	if backend.outgoingCalls != 0 {
		t.Fatalf("outgoing HID calls = %d, want 0", backend.outgoingCalls)
	}
}

type backendSpy struct {
	calls         []string
	outgoingCalls int
	report        []byte
	readTimeout   time.Duration
}

func (b *backendSpy) Enumerate(vid, pid uint16, visit func(deviceInfo) error) error {
	b.calls = append(b.calls, "enumerate")
	return visit(deviceInfo{path: "x6", vendorID: vid, productID: pid, usagePage: 1, usage: 0x80, interfaceNumber: 2})
}
func (b *backendSpy) OpenPath(string) (inputDevice, error) {
	b.calls = append(b.calls, "open")
	return &deviceSpy{b}, nil
}

type deviceSpy struct{ backend *backendSpy }

func (d *deviceSpy) ReadWithTimeout(p []byte, timeout time.Duration) (int, error) {
	d.backend.calls = append(d.backend.calls, "read")
	d.backend.readTimeout = timeout
	copy(p, d.backend.report)
	return len(d.backend.report), nil
}
func (d *deviceSpy) Close() error { d.backend.calls = append(d.backend.calls, "close"); return nil }

type fakeClock struct{ current time.Time }

func (c *fakeClock) Now() time.Time { return c.current }

type readResult struct {
	report []byte
	err    error
}

type readSequenceDevice struct {
	clock    *fakeClock
	results  []readResult
	reads    int
	timeouts []time.Duration
}

func (d *readSequenceDevice) ReadWithTimeout(p []byte, timeout time.Duration) (int, error) {
	d.reads++
	d.timeouts = append(d.timeouts, timeout)
	var result readResult
	if len(d.results) > 0 {
		result = d.results[0]
		d.results = d.results[1:]
	} else {
		result.err = errors.New("timeout")
	}
	if result.err != nil {
		d.clock.current = d.clock.current.Add(timeout)
		return 0, result.err
	}
	copy(p, result.report)
	return len(result.report), nil
}

func (d *readSequenceDevice) Close() error { return nil }

type deviceBackend struct{ device inputDevice }

func (b *deviceBackend) Enumerate(_ uint16, _ uint16, visit func(deviceInfo) error) error {
	return visit(deviceInfo{path: "x6", interfaceNumber: 2, usagePage: 1, usage: 0x80})
}

func (b *deviceBackend) OpenPath(string) (inputDevice, error) { return b.device, nil }
