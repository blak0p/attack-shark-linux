//go:build linux

package hidlinux

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

func writeFixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRoot builds a fake sysfs/dev tree mirroring the real dongle layout:
// USB device 1-4 with HID interfaces, the X6 HID device exposing the status
// interface, and a hidraw node pointing at it.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dev := filepath.Join(root, "sys/bus/usb/devices/1-4")
	writeFixtureFile(t, filepath.Join(dev, "idVendor"), "1d57\n")
	writeFixtureFile(t, filepath.Join(dev, "idProduct"), "fa60\n")
	for number, endpoint := range map[string]string{"0": "0x81", "1": "0x82", "2": "0x83", "3": "0x84"} {
		iface := filepath.Join(dev, "1-4:1."+number)
		writeFixtureFile(t, filepath.Join(iface, "bInterfaceNumber"), number+"\n")
		writeFixtureFile(t, filepath.Join(iface, "bInterfaceClass"), "03\n")
		writeFixtureFile(t, filepath.Join(iface, "ep_"+endpoint[2:], "bEndpointAddress"), endpoint+"\n")
	}

	hid := filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA60.0001")
	writeFixtureFile(t, filepath.Join(hid, "uevent"), "HID_NAME=Attack Shark X6\nHID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-4/input2\n")
	writeFixtureFile(t, filepath.Join(hid, "report_descriptor"), string([]byte{0x05, 0x01, 0x09, 0x80, 0x85, 0x04}))

	hidraw := filepath.Join(root, "sys/class/hidraw/hidraw3")
	if err := os.MkdirAll(hidraw, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hid, filepath.Join(hidraw, "device")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

type fakeHidrawNode struct {
	reports [][]byte
	block   chan struct{}
	closed  bool
}

func (f *fakeHidrawNode) Read(p []byte) (int, error) {
	if f.block != nil {
		<-f.block
		return 0, io.EOF
	}
	if len(f.reports) == 0 {
		return 0, io.EOF
	}
	report := f.reports[0]
	f.reports = f.reports[1:]
	return copy(p, report), nil
}

func (f *fakeHidrawNode) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	if f.block != nil {
		close(f.block)
	}
	return nil
}

func (f *fakeHidrawNode) SendFeatureReport(report []byte) (int, error) {
	return len(report), nil
}

type fakeHidrawOpener struct{ nodes map[string]hidrawNode }

func (o fakeHidrawOpener) OpenNode(path string) (hidrawNode, error) {
	if node, ok := o.nodes[path]; ok {
		return node, nil
	}
	return nil, os.ErrNotExist
}

func hidrawBackendForFixture(t *testing.T, opener hidrawNodeOpener) (*HidrawBackend, string) {
	t.Helper()
	root := fixtureRoot(t)
	backend := &HidrawBackend{
		sysRoot:     filepath.Join(root, "sys"),
		devRoot:     filepath.Join(root, "dev"),
		readTimeout: statusReadDeadline,
		opener:      opener,
	}
	return backend, root
}

func TestHidrawEnumerateFindsX6DeviceFromSysfs(t *testing.T) {
	backend, _ := hidrawBackendForFixture(t, fakeHidrawOpener{})

	candidates, err := backend.Enumerate(context.Background(), transport.X6Match())
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Enumerate() = %#v; want exactly one candidate", candidates)
	}
	got := candidates[0]
	if got.VendorID != 0x1D57 || got.ProductID != 0xFA60 || got.Connection != transport.Dongle || got.Path != "1:1-4" {
		t.Fatalf("Enumerate() candidate = %#v; want X6 identity on path 1:1-4", got)
	}
}

func TestHidrawEnumerateIgnoresOtherUsbDevicesAndInterfaces(t *testing.T) {
	root := fixtureRoot(t)
	other := filepath.Join(root, "sys/bus/usb/devices/1-3")
	writeFixtureFile(t, filepath.Join(other, "idVendor"), "046d\n")
	writeFixtureFile(t, filepath.Join(other, "idProduct"), "c52b\n")
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-0:1.0/bInterfaceNumber"), "0\n")

	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: statusReadDeadline, opener: fakeHidrawOpener{}}
	candidates, err := backend.Enumerate(context.Background(), transport.X6Match())
	if err != nil {
		t.Fatalf("Enumerate() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Path != "1:1-4" {
		t.Fatalf("Enumerate() = %#v; want only the X6 device", candidates)
	}
}

func TestHidrawEnumerateReturnsNotFoundWhenX6Absent(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-3/idVendor"), "046d\n")
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-3/idProduct"), "c52b\n")

	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: statusReadDeadline, opener: fakeHidrawOpener{}}
	_, err := backend.Enumerate(context.Background(), transport.X6Match())
	if !IsErrorKind(err, NotFound) {
		t.Fatalf("Enumerate() error = %v; want NotFound", err)
	}
}

func TestHidrawValidateDescriptorAcceptsOnlyX6StatusIdentity(t *testing.T) {
	backend, _ := hidrawBackendForFixture(t, fakeHidrawOpener{})
	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{Path: "1:1-4"}
	wanted := transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}

	source, err := backend.ValidateDescriptor(context.Background(), candidate, wanted)
	if err != nil || source.Path != "1:1-4" {
		t.Fatalf("ValidateDescriptor() = %#v, %v; want source 1:1-4", source, err)
	}

	if _, err := backend.ValidateDescriptor(context.Background(), candidate, transport.InputDescriptor{InterfaceNumber: 0}); !IsErrorKind(err, Mismatch) {
		t.Fatalf("ValidateDescriptor(wrong identity) error = %v; want Mismatch", err)
	}
	if _, err := backend.ValidateDescriptor(context.Background(), transport.Candidate{Path: "missing"}, wanted); !IsErrorKind(err, NotFound) {
		t.Fatalf("ValidateDescriptor(unknown source) error = %v; want NotFound", err)
	}
}

func TestHidrawReadInterruptINStreamsReportsUntilConsumed(t *testing.T) {
	root := fixtureRoot(t)
	nodePath := filepath.Join(root, "dev/hidraw3")
	node := &fakeHidrawNode{reports: [][]byte{{0x01}, {0x02}}}
	opener := fakeHidrawOpener{nodes: map[string]hidrawNode{nodePath: node}}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: statusReadDeadline, opener: opener}

	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}

	var seen []int
	err := backend.ReadInterruptIN(context.Background(), transport.InputSource{Path: "1:1-4"}, func(report []byte) bool {
		seen = append(seen, int(report[0]))
		return len(seen) < 2
	})
	if err != nil {
		t.Fatalf("ReadInterruptIN() error = %v", err)
	}
	if !reflect.DeepEqual(seen, []int{1, 2}) {
		t.Fatalf("ReadInterruptIN() delivered %v; want [1 2]", seen)
	}
	if !node.closed {
		t.Fatal("ReadInterruptIN() did not close the hidraw node")
	}
}

func TestHidrawReadInterruptINTimesOutWithoutReports(t *testing.T) {
	root := fixtureRoot(t)
	nodePath := filepath.Join(root, "dev/hidraw3")
	node := &fakeHidrawNode{block: make(chan struct{})}
	opener := fakeHidrawOpener{nodes: map[string]hidrawNode{nodePath: node}}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 50 * time.Millisecond, opener: opener}

	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}

	err := backend.ReadInterruptIN(context.Background(), transport.InputSource{Path: "1:1-4"}, func([]byte) bool { return true })
	if !IsErrorKind(err, Timeout) {
		t.Fatalf("ReadInterruptIN() error = %v; want Timeout", err)
	}
}

func TestHidrawReadInterruptINReturnsNotFoundForUnknownSource(t *testing.T) {
	backend, _ := hidrawBackendForFixture(t, fakeHidrawOpener{})
	err := backend.ReadInterruptIN(context.Background(), transport.InputSource{Path: "missing"}, func([]byte) bool { return true })
	if !IsErrorKind(err, NotFound) {
		t.Fatalf("ReadInterruptIN() error = %v; want NotFound", err)
	}
}
