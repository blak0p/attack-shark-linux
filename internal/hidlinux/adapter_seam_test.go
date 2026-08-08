package hidlinux

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestAdapterCleansUpClaimConfigurationAndDeviceInReverseOrder(t *testing.T) {
	cleanup := make([]string, 0, 3)
	claim := &fakeClaim{cleanup: &cleanup}
	adapter := NewAdapter(
		&fakeUSB{device: &fakeDevice{configuration: &fakeConfiguration{claim: claim, cleanup: &cleanup}, cleanup: &cleanup}},
		fakeSysfs{descriptor: x6ReportDescriptor},
	)

	err := adapter.WithValidatedCandidate(context.Background(), validCandidate(), func(Claim) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithValidatedCandidate() error = %v", err)
	}

	if got, want := cleanup, []string{"claim", "configuration", "device"}; !sameStrings(got, want) {
		t.Fatalf("cleanup order = %v, want %v", got, want)
	}
}

func TestAdapterCleansUpInReverseOrderWhenClaimUseFails(t *testing.T) {
	cleanup := make([]string, 0, 3)
	claim := &fakeClaim{cleanup: &cleanup}
	adapter := NewAdapter(
		&fakeUSB{device: &fakeDevice{configuration: &fakeConfiguration{claim: claim, cleanup: &cleanup}, cleanup: &cleanup}},
		fakeSysfs{descriptor: x6ReportDescriptor},
	)
	wantErr := errors.New("claim use failed")

	err := adapter.WithValidatedCandidate(context.Background(), validCandidate(), func(Claim) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithValidatedCandidate() error = %v, want %v", err, wantErr)
	}
	if got, want := cleanup, []string{"claim", "configuration", "device"}; !sameStrings(got, want) {
		t.Fatalf("cleanup order = %v, want %v", got, want)
	}
}

func TestAdapterDoesNotOverlapOperations(t *testing.T) {
	usb := &blockingUSB{
		device:  &fakeDevice{configuration: &fakeConfiguration{claim: &fakeClaim{}}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	adapter := NewAdapter(usb, fakeSysfs{descriptor: x6ReportDescriptor})

	first := make(chan error, 1)
	go func() {
		first <- adapter.WithValidatedCandidate(context.Background(), validCandidate(), func(Claim) error { return nil })
	}()
	<-usb.entered

	second := make(chan error, 1)
	go func() {
		second <- adapter.WithValidatedCandidate(context.Background(), validCandidate(), func(Claim) error { return nil })
	}()
	close(usb.release)

	if err := <-first; err != nil {
		t.Fatalf("first operation error = %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("second operation error = %v", err)
	}
	if got, want := usb.maxActive, 1; got != want {
		t.Fatalf("maximum concurrent USB operations = %d, want %d", got, want)
	}
}

func TestAdapterDoesNotOpenUSBForCandidateMismatch(t *testing.T) {
	tests := []struct {
		name       string
		candidate  Candidate
		descriptor []byte
	}{
		{name: "identity", candidate: Candidate{VendorID: 0xFFFF, ProductID: x6ProductID, Bus: 1, PortPath: "1-2"}, descriptor: x6ReportDescriptor},
		{name: "physical path", candidate: Candidate{VendorID: x6VendorID, ProductID: x6ProductID, Bus: 0, PortPath: ""}, descriptor: x6ReportDescriptor},
		{name: "report descriptor", candidate: validCandidate(), descriptor: []byte{0x05, 0x01, 0x09, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usb := &fakeUSB{device: &fakeDevice{configuration: &fakeConfiguration{claim: &fakeClaim{}}}}
			adapter := NewAdapter(usb, fakeSysfs{descriptor: tt.descriptor})

			err := adapter.WithValidatedCandidate(context.Background(), tt.candidate, func(Claim) error { return nil })
			if !errors.Is(err, ErrCandidateMismatch) {
				t.Fatalf("WithValidatedCandidate() error = %v, want ErrCandidateMismatch", err)
			}
			if got := usb.opens; got != 0 {
				t.Fatalf("USB Open() calls = %d, want 0", got)
			}
		})
	}
}

type fakeUSB struct {
	device *fakeDevice
	opens  int
}

func (f *fakeUSB) Open(context.Context, Candidate) (Device, error) {
	f.opens++
	return f.device, nil
}

type blockingUSB struct {
	device            *fakeDevice
	entered, release  chan struct{}
	mu                sync.Mutex
	active, maxActive int
}

func (f *blockingUSB) Open(context.Context, Candidate) (Device, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	select {
	case f.entered <- struct{}{}:
	default:
	}
	<-f.release

	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	return f.device, nil
}

type fakeDevice struct {
	configuration *fakeConfiguration
	cleanup       *[]string
}

func (f *fakeDevice) OpenConfiguration(context.Context) (Configuration, error) {
	return f.configuration, nil
}
func (f *fakeDevice) Close() error {
	if f.cleanup != nil {
		*f.cleanup = append(*f.cleanup, "device")
	}
	return nil
}

type fakeConfiguration struct {
	claim   Claim
	cleanup *[]string
}

func (f *fakeConfiguration) Claim(context.Context, int, int) (Claim, error) { return f.claim, nil }
func (f *fakeConfiguration) Close() error {
	if f.cleanup != nil {
		*f.cleanup = append(*f.cleanup, "configuration")
	}
	return nil
}

type fakeClaim struct{ cleanup *[]string }

func (f *fakeClaim) Close() error {
	if f.cleanup != nil {
		*f.cleanup = append(*f.cleanup, "claim")
	}
	return nil
}

func (*fakeClaim) ReadInterruptIN(context.Context, uint8, func([]byte) bool) error { return nil }
func (*fakeClaim) ControlTransfer(context.Context, uint8, uint8, uint16, uint16, []byte) (int, error) {
	return 0, nil
}

type fakeSysfs struct{ descriptor []byte }

func (f fakeSysfs) ReportDescriptor(context.Context, Candidate) ([]byte, error) {
	return f.descriptor, nil
}

func validCandidate() Candidate {
	return Candidate{VendorID: x6VendorID, ProductID: x6ProductID, Bus: 1, PortPath: "1-2", Interfaces: validInterfaces()}
}

var x6ReportDescriptor = []byte{0x05, 0x01, 0x09, 0x80}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
