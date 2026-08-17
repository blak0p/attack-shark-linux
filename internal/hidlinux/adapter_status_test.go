package hidlinux

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestAdapterDiscoveryRejectsEveryMismatchBeforeUSBAccess(t *testing.T) {
	tests := []struct {
		name       string
		candidate  Candidate
		descriptor []byte
	}{
		{name: "wrong vendor", candidate: Candidate{VendorID: 0xffff, ProductID: x6ProductID, Bus: 1, PortPath: "1-2", Interfaces: validInterfaces()}, descriptor: x6ReportDescriptor},
		{name: "unstable physical path", candidate: Candidate{VendorID: x6VendorID, ProductID: x6ProductID, Bus: 1, PortPath: "", Interfaces: validInterfaces()}, descriptor: x6ReportDescriptor},
		{name: "missing interface", candidate: candidateWithInterface(InterfaceDescriptor{Number: 1, AlternateSetting: 0, Class: hidClass, Endpoints: []uint8{statusEndpoint}}), descriptor: x6ReportDescriptor},
		{name: "wrong alternate setting", candidate: candidateWithInterface(InterfaceDescriptor{Number: 2, AlternateSetting: 1, Class: hidClass, Endpoints: []uint8{statusEndpoint}}), descriptor: x6ReportDescriptor},
		{name: "non HID interface", candidate: candidateWithInterface(InterfaceDescriptor{Number: 2, AlternateSetting: 0, Class: 0xff, Endpoints: []uint8{statusEndpoint}}), descriptor: x6ReportDescriptor},
		{name: "missing status endpoint", candidate: candidateWithInterface(InterfaceDescriptor{Number: 2, AlternateSetting: 0, Class: hidClass, Endpoints: []uint8{0x81}}), descriptor: x6ReportDescriptor},
		{name: "wrong HID usage", candidate: candidateWithInterface(), descriptor: []byte{0x05, 0x01, 0x09, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usb := &statusUSB{}
			adapter := NewStatusAdapter(&statusDiscovery{candidates: []Candidate{tt.candidate}}, usb, fakeSysfs{descriptor: tt.descriptor})
			candidates, err := adapter.Enumerate(context.Background(), transport.X6Match())
			if tt.name == "wrong vendor" {
				if !IsErrorKind(err, NotFound) || len(candidates) != 0 {
					t.Fatalf("Enumerate() = %v, %v; want typed not-found and no recognized candidate", candidates, err)
				}
				if usb.opens != 0 || usb.claims != 0 || usb.reads != 0 {
					t.Fatalf("mismatch performed USB calls: opens=%d claims=%d reads=%d", usb.opens, usb.claims, usb.reads)
				}
				return
			}
			if err != nil || len(candidates) != 1 {
				t.Fatalf("Enumerate() = %v, %v; want one candidate", candidates, err)
			}
			_, err = adapter.ValidateDescriptor(context.Background(), candidates[0], transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83})
			if !IsErrorKind(err, Mismatch) {
				t.Fatalf("ValidateDescriptor() error = %v, want typed mismatch", err)
			}
			if usb.opens != 0 || usb.claims != 0 || usb.reads != 0 {
				t.Fatalf("mismatch performed USB calls: opens=%d claims=%d reads=%d", usb.opens, usb.claims, usb.reads)
			}
		})
	}
}

func TestAdapterMapsDiscoveryAndOperationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind ErrorKind
	}{
		{name: "permission", err: os.ErrPermission, kind: Permission},
		{name: "timeout", err: context.DeadlineExceeded, kind: Timeout},
		{name: "cancelled", err: context.Canceled, kind: Cancelled},
		{name: "disconnect", err: ErrDeviceDisconnected, kind: Disconnected},
		{name: "io", err: errors.New("transport failed"), kind: IO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewStatusAdapter(&statusDiscovery{err: tt.err}, &statusUSB{}, fakeSysfs{})
			_, err := adapter.Enumerate(context.Background(), transport.X6Match())
			if !IsErrorKind(err, tt.kind) || !errors.Is(err, tt.err) {
				t.Fatalf("Enumerate() error = %v, want %s wrapping %v", err, tt.kind, tt.err)
			}
		})
	}
}

func TestAdapterStatusRunsOnlyOnExplicitRequestAndKeepsAbsentBatteryUnavailable(t *testing.T) {
	tests := []struct {
		name              string
		reports           [][]byte
		wantBattery       int
		wantBatteryExists bool
	}{
		{name: "battery report", reports: [][]byte{{0x03, 0x10, 0x40, 0x00, 8}}, wantBattery: 80, wantBatteryExists: true},
		{name: "no battery report", reports: [][]byte{{0x03, 0x10, 0x50, 0x00, 4}}, wantBatteryExists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usb := &statusUSB{reports: tt.reports}
			discovery := &statusDiscovery{candidates: []Candidate{candidateWithInterface()}}
			service := x6.NewService(NewStatusAdapter(discovery, usb, fakeSysfs{descriptor: x6ReportDescriptor}))
			if discovery.calls != 0 || usb.opens != 0 || usb.reads != 0 {
				t.Fatalf("construction performed status I/O: discovery=%d opens=%d reads=%d", discovery.calls, usb.opens, usb.reads)
			}

			status, err := service.Status(context.Background())
			if err != nil {
				t.Fatalf("Status() error = %v", err)
			}
			if status.Connection != x6.Dongle || status.BatteryAvailable != tt.wantBatteryExists || status.BatteryPercent != tt.wantBattery {
				t.Fatalf("Status() = %#v", status)
			}
			if discovery.calls != 1 || usb.opens != 1 || usb.reads != 1 {
				t.Fatalf("explicit status I/O calls: discovery=%d opens=%d reads=%d, want one each", discovery.calls, usb.opens, usb.reads)
			}
		})
	}
}

type statusDiscovery struct {
	candidates []Candidate
	err        error
	calls      int
}

func (f *statusDiscovery) Enumerate(context.Context) ([]Candidate, error) {
	f.calls++
	return f.candidates, f.err
}

type statusUSB struct {
	reports              [][]byte
	opens, claims, reads int
	readErr              error
}

func (f *statusUSB) Open(context.Context, Candidate) (Device, error) {
	f.opens++
	return &statusDevice{usb: f}, nil
}

type statusDevice struct{ usb *statusUSB }

func (f *statusDevice) OpenConfiguration(context.Context) (Configuration, error) {
	return &statusConfiguration{usb: f.usb}, nil
}
func (*statusDevice) Close() error { return nil }

type statusConfiguration struct{ usb *statusUSB }

func (f *statusConfiguration) Claim(context.Context, int, int) (Claim, error) {
	f.usb.claims++
	return &statusClaim{usb: f.usb}, nil
}
func (*statusConfiguration) Close() error { return nil }

type statusClaim struct{ usb *statusUSB }

func (*statusClaim) ControlTransfer(context.Context, uint8, uint8, uint16, uint16, []byte) (int, error) {
	return 0, errors.New("unexpected control transfer")
}

func (f *statusClaim) ReadInterruptIN(_ context.Context, endpoint uint8, use func([]byte) bool) error {
	if endpoint != statusEndpoint {
		return errors.New("unexpected endpoint")
	}
	f.usb.reads++
	for _, report := range f.usb.reports {
		if !use(report) {
			break
		}
	}
	return f.usb.readErr
}
func (*statusClaim) Close() error { return nil }

func candidateWithInterface(interfaces ...InterfaceDescriptor) Candidate {
	if len(interfaces) == 0 {
		interfaces = validInterfaces()
	}
	return Candidate{VendorID: x6VendorID, ProductID: x6ProductID, Bus: 1, PortPath: "1-2", Interfaces: interfaces}
}

func validInterfaces() []InterfaceDescriptor {
	return []InterfaceDescriptor{{Number: 2, AlternateSetting: 0, Class: hidClass, Endpoints: []uint8{statusEndpoint}}}
}
