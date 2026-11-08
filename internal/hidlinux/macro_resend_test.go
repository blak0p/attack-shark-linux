//go:build linux

package hidlinux

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// Rejection the receiver sent instead of the report08 ACK on real hardware.
var macroAssignmentRejection = []byte{0x03, 0x10, 0x00, 0x00, 0x02}

var remapACK = []byte{0x03, 0x10, 0x50, 0x00, 0x08}

func macroResendFixture(t *testing.T, node *scriptedNode, silence time.Duration) (*HidrawBackend, mouse.Binding) {
	t.Helper()
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	b := &HidrawBackend{registry: fixtureRegistry(t), sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: time.Second, resendDelay: time.Millisecond, silenceDelay: silence, maxAttempts: 4, opener: &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}}
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "A"}, Path: "1:1-4"}
	return b, binding
}

func batchWrites(batch mouse.Batch, setupCopies int) [][]byte {
	var want [][]byte
	for range setupCopies {
		want = append(want, batch.Setup)
	}
	return append(want, batch.Chunks...)
}

func TestMacroAssignmentResendsReport08AfterRejection(t *testing.T) {
	// Writes: 1 report08 (rejected), 2 report08 resent (ACK), 3-5 report09 chunks (final ACK).
	node := newScriptedNode(map[int][][]byte{1: {macroAssignmentRejection}, 2: {remapACK}, 5: {macroACK}})
	b, binding := macroResendFixture(t, node, 0)
	batch := fixtureBatch(3)
	progress, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if progress.Assignment != mouse.MacroAssignmentACKConfirmed || progress.Upload != mouse.MacroUploadConfirmed {
		t.Fatalf("progress = %+v", progress)
	}
	if want := batchWrites(batch, 2); !reflect.DeepEqual(node.payloads(), want) {
		t.Fatalf("writes = %x, want %x", node.payloads(), want)
	}
}

func TestMacroAssignmentResendsReport08AfterSilence(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{2: {remapACK}, 5: {macroACK}})
	b, binding := macroResendFixture(t, node, 5*time.Millisecond)
	batch := fixtureBatch(3)
	progress, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if progress.Assignment != mouse.MacroAssignmentACKConfirmed || progress.Upload != mouse.MacroUploadConfirmed {
		t.Fatalf("progress = %+v", progress)
	}
	if want := batchWrites(batch, 2); !reflect.DeepEqual(node.payloads(), want) {
		t.Fatalf("writes = %x, want %x", node.payloads(), want)
	}
}

func TestMacroAssignmentStopsAfterMaxAttempts(t *testing.T) {
	rejectEach := map[int][][]byte{}
	for i := 1; i <= 4; i++ {
		rejectEach[i] = [][]byte{macroAssignmentRejection}
	}
	node := newScriptedNode(rejectEach)
	b, binding := macroResendFixture(t, node, 0)
	batch := fixtureBatch(3)
	progress, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
	if !IsErrorKind(err, Timeout) {
		t.Fatalf("error = %v, want timeout", err)
	}
	if progress.Assignment != mouse.MacroAssignmentUnknown || progress.Upload != mouse.MacroUploadNotStarted {
		t.Fatalf("progress = %+v", progress)
	}
	if want := batchWrites(batch, 4)[:4]; !reflect.DeepEqual(node.payloads(), want) {
		t.Fatalf("writes = %x, want four report08 copies and no report09", node.payloads())
	}
}

func TestMacroUploadChunksAreNeverResent(t *testing.T) {
	// The final status after the last chunk is a rejection: the chunks must not be written again.
	node := newScriptedNode(map[int][][]byte{1: {remapACK}, 4: {macroAssignmentRejection}})
	b, binding := macroResendFixture(t, node, 0)
	batch := fixtureBatch(3)
	progress, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
	if err == nil || progress.Upload != mouse.MacroUploadPossiblyPartial {
		t.Fatalf("progress = %+v, error = %v", progress, err)
	}
	if want := batchWrites(batch, 1); !reflect.DeepEqual(node.payloads(), want) {
		t.Fatalf("writes = %x, want %x", node.payloads(), want)
	}
}
