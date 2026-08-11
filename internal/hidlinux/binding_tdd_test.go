//go:build linux

package hidlinux

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/mouse"
	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
)

func TestHidrawSendAndAwaitBindingUsesOnlyExactValidatedPath(t *testing.T) {
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	node := newCommandHidrawNode([][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}})
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: time.Second, opener: opener}
	payload := make([]byte, protocol.DPIReportLength)
	payload[0] = 0x04
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}, ProfileID: "x6", Path: "1:1-4"}

	if err := backend.SendAndAwaitBound(context.Background(), binding, payload, func(report []byte) bool { return !protocol.MatchesDPIACK(report) }); err != nil {
		t.Fatalf("SendAndAwaitBound() error = %v", err)
	}
	if got := opener.opens(); got != 1 {
		t.Fatalf("hidraw opens = %d, want exact node opened once", got)
	}
}

func TestHidrawSendAndAwaitBindingRejectsStaleIdentityOrPathBeforeNodeOpen(t *testing.T) {
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	node := newCommandHidrawNode(nil)
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: time.Second, opener: opener}
	payload := make([]byte, protocol.DPIReportLength)
	payload[0] = 0x04

	for _, binding := range []mouse.Binding{
		{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "B"}, ProfileID: "x6", Path: "1:1-4"},
		{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}, ProfileID: "x6", Path: "1:1-9"},
	} {
		err := backend.SendAndAwaitBound(context.Background(), binding, payload, func([]byte) bool { return false })
		if !errors.Is(err, mouse.ErrStaleBinding) {
			t.Fatalf("SendAndAwaitBound(%#v) error = %v, want ErrStaleBinding", binding, err)
		}
	}
	if got := opener.opens(); got != 0 {
		t.Fatalf("hidraw opens = %d, want 0 for stale bindings", got)
	}
}
