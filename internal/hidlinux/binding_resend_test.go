//go:build linux

package hidlinux

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

// scriptedNode releases scripted reports: initial ones after the first write,
// and each entry of perWrite after the matching write number.
type scriptedNode struct {
	mu      sync.Mutex
	writes  int
	pending chan []byte
	closed  chan struct{}
	once    sync.Once
	script  map[int][][]byte
}

func newScriptedNode(script map[int][][]byte) *scriptedNode {
	return &scriptedNode{pending: make(chan []byte, 32), closed: make(chan struct{}), script: script}
}

func (n *scriptedNode) Read(p []byte) (int, error) {
	select {
	case r := <-n.pending:
		return copy(p, r), nil
	case <-n.closed:
		return 0, io.EOF
	}
}
func (n *scriptedNode) Close() error { n.once.Do(func() { close(n.closed) }); return nil }
func (n *scriptedNode) SendFeatureReport(r []byte) (int, error) {
	n.mu.Lock()
	n.writes++
	reports := n.script[n.writes]
	n.mu.Unlock()
	for _, r := range reports {
		n.pending <- r
	}
	return len(r), nil
}
func (n *scriptedNode) writeCount() int { n.mu.Lock(); defer n.mu.Unlock(); return n.writes }

var (
	rejectMotion = []byte{0x03, 0x10, 0xff, 0xff, 0x00}
	heartbeat    = []byte{0x03, 0x10, 0x40, 0x01, 0x0a}
	dpiACK       = []byte{0x03, 0x10, 0x50, 0x00, 0x04}
)

func resendHarness(t *testing.T, node *scriptedNode, delay time.Duration, readTimeout time.Duration) error {
	t.Helper()
	return resendHarnessWithSilence(t, node, delay, 0, readTimeout)
}

func resendHarnessWithSilence(t *testing.T, node *scriptedNode, delay, silence, readTimeout time.Duration) error {
	t.Helper()
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: readTimeout, resendDelay: delay, silenceDelay: silence, maxAttempts: 4, opener: opener}
	payload := make([]byte, protocol.DPIReportLength)
	payload[0] = 0x04
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}, ProfileID: "x6", Path: "1:1-4"}
	return backend.SendAndAwaitBound(context.Background(), binding, payload, func(r []byte) bool { return !protocol.MatchesDPIACK(r) })
}

func TestSendAndAwaitBoundResendsAfterMotionRejection(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{1: {rejectMotion}, 2: {dpiACK}})
	if err := resendHarness(t, node, time.Millisecond, time.Second); err != nil {
		t.Fatalf("error = %v", err)
	}
	if got := node.writeCount(); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
}

func TestSendAndAwaitBoundHeartbeatDoesNotResend(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{1: {heartbeat, dpiACK}})
	if err := resendHarness(t, node, time.Millisecond, time.Second); err != nil {
		t.Fatalf("error = %v", err)
	}
	if got := node.writeCount(); got != 1 {
		t.Fatalf("writes = %d, want 1", got)
	}
}

func TestSendAndAwaitBoundLateACKAfterRejectionSkipsResend(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{1: {rejectMotion, dpiACK}})
	if err := resendHarness(t, node, 200*time.Millisecond, time.Second); err != nil {
		t.Fatalf("error = %v", err)
	}
	if got := node.writeCount(); got != 1 {
		t.Fatalf("writes = %d, want 1", got)
	}
}

func TestSendAndAwaitBoundGivesUpAfterFourRejections(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{1: {rejectMotion}, 2: {rejectMotion}, 3: {rejectMotion}, 4: {rejectMotion}})
	err := resendHarness(t, node, time.Millisecond, time.Second)
	if !IsErrorKind(err, Timeout) || DiagnosticOperation(err) != "ack_failure" {
		t.Fatalf("error = %v, want ack_failure timeout", err)
	}
	if got := node.writeCount(); got != 4 {
		t.Fatalf("writes = %d, want 4", got)
	}
}

func TestSendAndAwaitBoundResendsAfterSilence(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{2: {dpiACK}})
	if err := resendHarnessWithSilence(t, node, time.Millisecond, 20*time.Millisecond, time.Second); err != nil {
		t.Fatalf("error = %v", err)
	}
	if got := node.writeCount(); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
}

func TestSendAndAwaitBoundGivesUpAfterFourSilentAttempts(t *testing.T) {
	node := newScriptedNode(map[int][][]byte{})
	err := resendHarnessWithSilence(t, node, time.Millisecond, 20*time.Millisecond, 200*time.Millisecond)
	if !IsErrorKind(err, Timeout) || DiagnosticOperation(err) != "ack_failure" {
		t.Fatalf("error = %v, want ack_failure timeout", err)
	}
	if got := node.writeCount(); got != 4 {
		t.Fatalf("writes = %d, want 4", got)
	}
}
