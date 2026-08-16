//go:build linux

package hidlinux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/mouse"
	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
	"github.com/alejandro/attack-shark-linux/internal/transport"
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

func TestHidrawSendAndAwaitBindingDoesNotReuseTimedOutListenerNode(t *testing.T) {
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	listenerNode := newCommandHidrawNode(nil)
	listenerNode.readWait = make(chan struct{})
	listenerNode.firstReadStarted = make(chan struct{})
	commandNode := newCommandHidrawNode([][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}})
	opener := &countingHidrawOpener{
		path:  filepath.Join(root, "dev/hidraw3"),
		nodes: []hidrawNode{listenerNode, commandNode},
	}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: time.Second, opener: opener}
	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}, ProfileID: "x6", Path: "1:1-4"}
	payload := make([]byte, protocol.DPIReportLength)
	payload[0] = 0x04

	listenerCtx, cancelListener := context.WithCancel(context.Background())
	defer cancelListener()
	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- backend.ReadInterruptIN(listenerCtx, transport.InputSource{Path: "1:1-4"}, func([]byte) bool { return true })
	}()
	<-listenerNode.firstReadStarted

	applyDone := make(chan error, 1)
	go func() {
		applyDone <- backend.SendAndAwaitBound(context.Background(), binding, payload, func(report []byte) bool {
			return !protocol.MatchesDPIACK(report)
		})
	}()
	deadline := time.After(time.Second)
	for {
		backend.ioMu.Lock()
		pending := backend.commandPending
		backend.ioMu.Unlock()
		if pending {
			break
		}
		select {
		case <-deadline:
			t.Fatal("SendAndAwaitBound() did not begin command ownership")
		case <-time.After(time.Millisecond):
		}
	}
	cancelListener()
	if err := <-listenerDone; !IsErrorKind(err, Timeout) {
		t.Fatalf("ReadInterruptIN() error = %v, want Timeout", err)
	}
	if err := <-applyDone; err != nil {
		t.Fatalf("SendAndAwaitBound() error = %v", err)
	}
	if got := opener.opens(); got != 2 {
		t.Fatalf("hidraw opens = %d, want separate listener and command nodes", got)
	}
	if !listenerNode.isClosed() {
		t.Fatal("listener node was not closed after its bounded read")
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

func TestHidrawSendAndAwaitSessionOnlyBindingRequiresSeriallessExactPath(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.Remove(filepath.Join(root, "sys/bus/usb/devices/1-4/serial")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove serial fixture: %v", err)
	}
	node := newCommandHidrawNode([][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}})
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: time.Second, opener: opener}
	payload := make([]byte, protocol.DPIReportLength)
	payload[0] = 0x04
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "session-id"}, ProfileID: "x6", Path: "1:1-4", SessionOnly: true}

	if err := backend.SendAndAwaitBound(context.Background(), binding, payload, func(report []byte) bool { return !protocol.MatchesDPIACK(report) }); err != nil {
		t.Fatalf("SendAndAwaitBound() error = %v", err)
	}
	if got := opener.opens(); got != 1 {
		t.Fatalf("hidraw opens = %d, want exact serial-less node opened once", got)
	}

	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "unexpected\n")
	if err := backend.SendAndAwaitBound(context.Background(), binding, payload, func([]byte) bool { return false }); !errors.Is(err, mouse.ErrStaleBinding) {
		t.Fatalf("SendAndAwaitBound() error = %v, want ErrStaleBinding after serial appears", err)
	}
}
