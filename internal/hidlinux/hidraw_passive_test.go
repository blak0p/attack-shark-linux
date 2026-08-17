//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
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
	writeFixtureFile(t, filepath.Join(dev, "serial"), "fixture-x6-serial\n")
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
	if got.VendorID != 0x1D57 || got.ProductID != 0xFA60 || got.Serial != "fixture-x6-serial" || got.Connection != transport.Dongle || got.Path != "1:1-4" {
		t.Fatalf("Enumerate() candidate = %#v; want X6 identity on path 1:1-4", got)
	}
}

func TestHidrawProfileValidRejectsInterfaceMismatchWithoutHidrawOpen(t *testing.T) {
	backend, root := hidrawBackendForFixture(t, fakeHidrawOpener{})
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/1-4:1.2/bInterfaceNumber"), "1\n")

	candidates, err := backend.Enumerate(context.Background(), transport.X6Match())
	if err != nil || len(candidates) != 1 || candidates[0].Serial != "fixture-x6-serial" {
		t.Fatalf("Enumerate() = %#v, %v; want visible stable-serial candidate", candidates, err)
	}
	facts := mouse.HIDFacts{StatusInput: transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}}
	if backend.ProfileValid(context.Background(), candidates[0], facts) {
		t.Fatal("ProfileValid() accepted an interface-mismatched candidate")
	}
}

func TestHidrawInventoryDiagnosticsIdentifyValidAndRejectedInterfaces(t *testing.T) {
	for _, tt := range []struct {
		name       string
		mutate     func(string)
		validation string
		interfaceN string
		eligible   string
		serial     string
	}{
		{
			name:       "valid vendor interface",
			validation: "profile_validation=true",
			interfaceN: "interface_number=2",
			eligible:   "eligibility=true",
			serial:     "serial_present=true",
		},
		{
			name: "interface one rejection",
			mutate: func(root string) {
				writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/1-4:1.2/bInterfaceNumber"), "1\n")
			},
			validation: "profile_validation=rejected",
			interfaceN: "interface_number=1",
			eligible:   "eligibility=false",
			serial:     "serial_present=true",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			backend, root := hidrawBackendForFixture(t, fakeHidrawOpener{})
			if tt.mutate != nil {
				tt.mutate(root)
			}
			var logs bytes.Buffer
			oldWriter := inventoryDiagnosticWriter
			inventoryDiagnosticWriter = &logs
			defer func() { inventoryDiagnosticWriter = oldWriter }()

			candidates, err := backend.Enumerate(context.Background(), transport.X6Match())
			if err != nil {
				t.Fatal(err)
			}
			facts := mouse.HIDFacts{StatusInput: transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}}
			if got := backend.ProfileValid(context.Background(), candidates[0], facts); got != (tt.mutate == nil) {
				t.Fatalf("ProfileValid() = %t; want %t", got, tt.mutate == nil)
			}
			output := logs.String()
			for _, want := range []string{"event=enumeration", "event=profile_validation", "vid_pid=1d57:fa60", "endpoint=0x83", tt.validation, tt.interfaceN, tt.eligible, tt.serial, "hidraw_basename=hidraw3"} {
				if !strings.Contains(output, want) {
					t.Fatalf("diagnostic output missing %q:\n%s", want, output)
				}
			}
			for _, forbidden := range []string{"fixture-x6-serial", root, "/dev/", "report"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("diagnostic output contains forbidden %q:\n%s", forbidden, output)
				}
			}
		})
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

func TestHidrawReadInterruptINWaitsForReadWorkerShutdown(t *testing.T) {
	root := fixtureRoot(t)
	nodePath := filepath.Join(root, "dev/hidraw3")
	node := &delayedCloseHidrawNode{
		closed:      make(chan struct{}),
		closeCalled: make(chan struct{}),
		readRelease: make(chan struct{}),
	}
	opener := fakeHidrawOpener{nodes: map[string]hidrawNode{nodePath: node}}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 20 * time.Millisecond, opener: opener}

	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- backend.ReadInterruptIN(context.Background(), transport.InputSource{Path: "1:1-4"}, func([]byte) bool { return true })
	}()
	<-node.closeCalled
	select {
	case err := <-done:
		t.Fatalf("ReadInterruptIN() returned before read worker shutdown: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	close(node.readRelease)
	if err := <-done; !IsErrorKind(err, Timeout) {
		t.Fatalf("ReadInterruptIN() error = %v, want Timeout", err)
	}
}

func TestHidrawReadInterruptINReturnsNotFoundForUnknownSource(t *testing.T) {
	backend, _ := hidrawBackendForFixture(t, fakeHidrawOpener{})
	err := backend.ReadInterruptIN(context.Background(), transport.InputSource{Path: "missing"}, func([]byte) bool { return true })
	if !IsErrorKind(err, NotFound) {
		t.Fatalf("ReadInterruptIN() error = %v; want NotFound", err)
	}
}

type delayedCloseHidrawNode struct {
	closed      chan struct{}
	closeCalled chan struct{}
	readRelease chan struct{}
	closeOnce   sync.Once
}

func (n *delayedCloseHidrawNode) Read([]byte) (int, error) {
	<-n.closed
	<-n.readRelease
	return 0, io.EOF
}

func (n *delayedCloseHidrawNode) Close() error {
	n.closeOnce.Do(func() {
		close(n.closeCalled)
		close(n.closed)
	})
	return nil
}

func (n *delayedCloseHidrawNode) SendFeatureReport([]byte) (int, error) {
	return 0, errors.New("unexpected feature report")
}
