package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
	"github.com/alejandro/attack-shark-linux/internal/transport"
)

func TestAdapterSendAndAwaitWritesEncodedDPIAndFiltersACK(t *testing.T) {
	want, err := protocol.EncodeDPIReport(protocol.DefaultDPIConfig())
	if err != nil {
		t.Fatalf("EncodeDPIReport() error = %v", err)
	}
	cleanup := make([]string, 0, 3)
	claim := &applyClaim{reports: [][]byte{{0x03, 0x10, 0x40, 0x00, 8}, {0x03, 0x10, 0x50, 0x00, 0x04}}, cleanup: &cleanup}
	adapter := applyAdapter(claim)

	var seen [][]byte
	err = adapter.SendAndAwait(context.Background(), want, func(report []byte) bool {
		seen = append(seen, append([]byte(nil), report...))
		return !protocol.MatchesDPIACK(report)
	})
	if err != nil {
		t.Fatalf("SendAndAwait() error = %v", err)
	}
	if !bytes.Equal(claim.written, want) {
		t.Fatalf("written report = %x, want %x", claim.written, want)
	}
	if len(claim.written) != protocol.DPIReportLength {
		t.Fatalf("written report length = %d, want %d", len(claim.written), protocol.DPIReportLength)
	}
	if claim.requestType != 0x21 || claim.request != 0x09 || claim.value != 0x0304 || claim.index != 2 {
		t.Fatalf("SET_REPORT setup = (%#x, %#x, %#x, %#x), want (0x21, 0x09, 0x0304, 0x2)", claim.requestType, claim.request, claim.value, claim.index)
	}
	if got, want := seen, claim.reports; !sameReports(got, want) {
		t.Fatalf("ACK reports seen = %x, want %x", got, want)
	}
	if got, want := cleanup, []string{"claim", "configuration", "device"}; !sameStrings(got, want) {
		t.Fatalf("cleanup order = %v, want %v", got, want)
	}
}

func TestAdapterSendAndAwaitRevalidatesBeforeWriting(t *testing.T) {
	claim := &applyClaim{}
	adapter := NewAdapter(&applyUSB{claim: claim}, fakeSysfs{descriptor: []byte{0x05, 0x01, 0x09, 0x02}})
	adapter.sources[candidateKey(validCandidate())] = validCandidate()

	err := adapter.SendAndAwait(context.Background(), make([]byte, protocol.DPIReportLength), func([]byte) bool { return true })
	if !IsErrorKind(err, Mismatch) {
		t.Fatalf("SendAndAwait() error = %v, want typed mismatch", err)
	}
	if len(claim.written) != 0 {
		t.Fatalf("writes = %d, want 0", len(claim.written))
	}
}

func TestAdapterSendAndAwaitMapsWaitErrors(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		kind ErrorKind
	}{
		{name: "timeout", ctx: context.Background(), err: context.DeadlineExceeded, kind: Timeout},
		{name: "cancelled", ctx: cancelledContext(t), err: context.Canceled, kind: Cancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim := &applyClaim{readErr: tt.err}
			adapter := applyAdapter(claim)

			err := adapter.SendAndAwait(tt.ctx, make([]byte, protocol.DPIReportLength), func([]byte) bool { return false })
			if !IsErrorKind(err, tt.kind) || !errors.Is(err, tt.err) {
				t.Fatalf("SendAndAwait() error = %v, want %s wrapping %v", err, tt.kind, tt.err)
			}
		})
	}
}

func TestAdapterSendAndAwaitDoesNotOverlapStatusRead(t *testing.T) {
	claim := &applyClaim{reports: [][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}}, writeEntered: make(chan struct{}), writeRelease: make(chan struct{})}
	adapter := applyAdapter(claim)
	source := transportSource(validCandidate())

	commandDone := make(chan error, 1)
	go func() {
		commandDone <- adapter.SendAndAwait(context.Background(), make([]byte, protocol.DPIReportLength), func(report []byte) bool { return !protocol.MatchesDPIACK(report) })
	}()
	<-claim.writeEntered

	statusDone := make(chan error, 1)
	go func() {
		statusDone <- adapter.ReadInterruptIN(context.Background(), source, func([]byte) bool { return false })
	}()
	time.Sleep(10 * time.Millisecond)
	if got := claim.reads; got != 0 {
		t.Fatalf("status reads while write holds adapter mutex = %d, want 0", got)
	}
	close(claim.writeRelease)
	if err := <-commandDone; err != nil {
		t.Fatalf("SendAndAwait() error = %v", err)
	}
	if err := <-statusDone; err != nil {
		t.Fatalf("ReadInterruptIN() error = %v", err)
	}
}

type applyUSB struct{ claim *applyClaim }

func (f *applyUSB) Open(context.Context, Candidate) (Device, error) {
	return &fakeDevice{configuration: &fakeConfiguration{claim: f.claim, cleanup: f.claim.cleanup}, cleanup: f.claim.cleanup}, nil
}

type applyClaim struct {
	reports                    [][]byte
	written                    []byte
	readErr                    error
	cleanup                    *[]string
	writeEntered, writeRelease chan struct{}
	mu                         sync.Mutex
	reads                      int
	requestType, request       uint8
	value, index               uint16
}

func (f *applyClaim) ControlTransfer(_ context.Context, requestType, request uint8, value, index uint16, report []byte) (int, error) {
	f.requestType, f.request, f.value, f.index = requestType, request, value, index
	f.written = append([]byte(nil), report...)
	if f.writeEntered != nil {
		close(f.writeEntered)
		<-f.writeRelease
	}
	return len(report), nil
}

func (f *applyClaim) ReadInterruptIN(_ context.Context, endpoint uint8, use func([]byte) bool) error {
	if endpoint != statusEndpoint {
		return errors.New("unexpected endpoint")
	}
	f.mu.Lock()
	f.reads++
	f.mu.Unlock()
	for _, report := range f.reports {
		if !use(report) {
			return nil
		}
	}
	return f.readErr
}

func (f *applyClaim) Close() error {
	if f.cleanup != nil {
		*f.cleanup = append(*f.cleanup, "claim")
	}
	return nil
}

func applyAdapter(claim *applyClaim) *Adapter {
	candidate := validCandidate()
	adapter := NewAdapter(&applyUSB{claim: claim}, fakeSysfs{descriptor: x6ReportDescriptor})
	adapter.sources[candidateKey(candidate)] = candidate
	return adapter
}

func transportSource(candidate Candidate) transport.InputSource {
	return transport.InputSource{Path: candidateKey(candidate)}
}

func cancelledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func sameReports(got, want [][]byte) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			return false
		}
	}
	return true
}
