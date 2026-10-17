//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func cableFixture(t *testing.T, root string) {
	t.Helper()
	dev := filepath.Join(root, "sys/bus/usb/devices/1-5")
	writeFixtureFile(t, filepath.Join(dev, "idVendor"), "1d57\n")
	writeFixtureFile(t, filepath.Join(dev, "idProduct"), "fa61\n")
	writeFixtureFile(t, filepath.Join(dev, "serial"), "cable\n")
	iface := filepath.Join(dev, "1-5:1.2")
	writeFixtureFile(t, filepath.Join(iface, "bInterfaceNumber"), "2\n")
	writeFixtureFile(t, filepath.Join(iface, "bInterfaceClass"), "03\n")
	writeFixtureFile(t, filepath.Join(iface, "ep_83/bEndpointAddress"), "83\n")
	hid := filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA61.0002")
	writeFixtureFile(t, filepath.Join(hid, "uevent"), "HID_ID=0003:00001D57:0000FA61\nHID_PHYS=usb-0000:0d:00.0-5/input2\n")
	writeFixtureFile(t, filepath.Join(hid, "report_descriptor"), string([]byte{5, 1, 9, 0x80}))
}

func TestWiredDescriptorAdmission(t *testing.T) {
	backend, root := hidrawBackendForFixture(t, fakeHidrawOpener{})
	cableFixture(t, root)
	candidates, err := backend.EnumerateX6(context.Background())
	if err != nil || len(candidates) != 2 {
		t.Fatalf("inventory = %v, %v", candidates, err)
	}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile(), x6.NewWiredProfile())
	if err != nil {
		t.Fatal(err)
	}
	for i, candidate := range candidates {
		pid, connection := uint16(0xfa60), transport.Dongle
		if i == 1 {
			pid, connection = 0xfa61, transport.Wired
		}
		if candidate.ProductID != pid || candidate.Connection != connection {
			t.Fatalf("identity = %+v", candidate)
		}
		profile, ok := registry.Lookup(candidate.VendorID, candidate.ProductID)
		if !ok || profile.Match().ProductID != pid || !backend.ProfileValid(context.Background(), candidate, profile.HIDFacts()) {
			t.Fatalf("profile rejected %+v", candidate)
		}
		if _, err := backend.ValidateDescriptor(context.Background(), candidate, profile.HIDFacts().StatusInput); err != nil {
			t.Fatal(err)
		}
	}
	// Explicit single-match scans must not broaden discovery.
	only, err := backend.Enumerate(context.Background(), transport.X6Match())
	if err != nil || len(only) != 1 || only[0].ProductID != 0xfa60 {
		t.Fatalf("exact scan = %v, %v", only, err)
	}

}

func TestPairedCommandUsesCable(t *testing.T) {
	node := newCommandHidrawNode([][]byte{{3, 0x10, 0x50, 0, 4}})
	backend, root := hidrawBackendForFixture(t, fakeHidrawOpener{})
	cableFixture(t, root)
	hidraw := filepath.Join(root, "sys/class/hidraw/hidraw4")
	if err := os.MkdirAll(hidraw, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA61.0002"), filepath.Join(hidraw, "device")); err != nil {
		t.Fatal(err)
	}
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw4"), node: node}
	backend.opener = opener
	payload := append([]byte{4}, make([]byte, 55)...)
	if err := backend.SendAndAwait(context.Background(), payload, func([]byte) bool { return false }); err != nil || opener.opens() != 1 || !bytes.Equal(node.written(), payload) {
		t.Fatalf("send=%v opens=%d writes=%x", err, opener.opens(), node.written())
	}
}

func TestPairedCommandAdmissionAndBoundCable(t *testing.T) {
	for _, mode := range []string{"bound cable", "invalid cable", "two cables"} {
		t.Run(mode, func(t *testing.T) {
			node := newCommandHidrawNode([][]byte{{3, 0x10, 0x50, 0, 4}})
			backend, root := hidrawBackendForFixture(t, fakeHidrawOpener{})
			cableFixture(t, root)
			hidraw := filepath.Join(root, "sys/class/hidraw/hidraw4")
			if err := os.MkdirAll(hidraw, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA61.0002"), filepath.Join(hidraw, "device")); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "dev/hidraw4")
			if mode == "invalid cable" {
				writeFixtureFile(t, filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA61.0002/report_descriptor"), "invalid")
				path = filepath.Join(root, "dev/hidraw3")
			}
			if mode == "two cables" {
				writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/idProduct"), "fa61\n")
				writeFixtureFile(t, filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA60.0001/uevent"), "HID_ID=0003:00001D57:0000FA61\nHID_PHYS=usb-0000:0d:00.0-4/input2\n")
			}
			opener := &countingHidrawOpener{path: path, node: node}
			backend.opener = opener
			payload := append([]byte{4}, make([]byte, 55)...)
			var err error
			if mode == "bound cable" {
				err = backend.SendAndAwaitBound(context.Background(), mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa61, Serial: "cable"}, Path: "1:1-5", ProfileID: "attack-shark-x6"}, payload, func([]byte) bool { return false })
			} else {
				err = backend.SendAndAwait(context.Background(), payload, func([]byte) bool { return false })
			}
			if mode == "two cables" {
				if !IsErrorKind(err, Mismatch) || opener.opens() != 0 || len(node.written()) != 0 {
					t.Fatalf("ambiguous send=%v opens=%d writes=%x", err, opener.opens(), node.written())
				}
				return
			}
			if err != nil || opener.opens() != 1 || !bytes.Equal(node.written(), payload) {
				t.Fatalf("send=%v opens=%d writes=%x", err, opener.opens(), node.written())
			}
		})
	}
}

func TestUSBModeBoundCommand(t *testing.T) {
	for _, pid := range []uint16{0xfa60, 0xfa61, 0xfa62} {
		t.Run(fmt.Sprintf("%04x", pid), func(t *testing.T) {
			root := fixtureRoot(t)
			writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/idProduct"), fmt.Sprintf("%04x\n", pid))
			writeFixtureFile(t, filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA60.0001/uevent"), fmt.Sprintf("HID_ID=0003:00001D57:0000%04X\nHID_PHYS=usb-0000:0d:00.0-4/input2\n", pid))
			node := newCommandHidrawNode([][]byte{{3, 0x10, 0x50, 0, 4}})
			opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
			backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), opener: opener, readTimeout: time.Second}
			binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: pid, Serial: "fixture-x6-serial"}, Path: "1:1-4", ProfileID: "attack-shark-x6"}
			payload := append([]byte{4}, make([]byte, 55)...)
			ack := false
			err := backend.SendAndAwaitBound(context.Background(), binding, payload, func(report []byte) bool { ack = protocol.MatchesDPIACK(report); return !ack })
			if pid == 0xfa62 {
				if err == nil || opener.opens() != 0 || len(node.written()) != 0 || ack {
					t.Fatalf("rejected PID: err=%v opens=%d writes=%x", err, opener.opens(), node.written())
				}
				return
			}
			if err != nil || !ack || opener.opens() != 1 || !bytes.Equal(node.written(), payload) || binding.ID.ProductID != pid {
				t.Fatalf("bound command: %v ack=%t opens=%d writes=%x", err, ack, opener.opens(), node.written())
			}
		})
	}
}
