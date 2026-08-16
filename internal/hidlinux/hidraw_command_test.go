//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
	"github.com/alejandro/attack-shark-linux/internal/transport"
)

func TestHidrawSendAndAwaitTable(t *testing.T) {
	tests := []struct {
		name        string
		reports     [][]byte
		writeErr    error
		waitReads   bool
		invalidX6   bool
		wantError   ErrorKind
		wantReports int
	}{
		{
			name:        "writes feature report and receives ACK",
			reports:     [][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}},
			wantReports: 1,
		},
		{
			name:        "passes nonmatching reports before ACK",
			reports:     [][]byte{{0x03, 0x10, 0x40, 0x00, 8}, {0x03, 0x10, 0x50, 0x00, 0x04}},
			wantReports: 2,
		},
		{
			name:      "returns feature write error",
			writeErr:  errors.New("feature write failed"),
			wantError: IO,
		},
		{
			name:      "times out waiting for ACK",
			waitReads: true,
			wantError: Timeout,
		},
		{
			name:      "rejects unvalidated X6 before command I/O",
			invalidX6: true,
			wantError: Mismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixtureRoot(t)
			if tt.invalidX6 {
				writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/1-4:1.2/ep_83/bEndpointAddress"), "0x82\n")
			}
			node := newCommandHidrawNode(tt.reports)
			node.writeErr = tt.writeErr
			if tt.waitReads {
				node.readWait = make(chan struct{})
			}
			opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
			backend := &HidrawBackend{
				sysRoot:     filepath.Join(root, "sys"),
				devRoot:     filepath.Join(root, "dev"),
				readTimeout: 20 * time.Millisecond,
				opener:      opener,
			}

			payload := make([]byte, protocol.DPIReportLength)
			payload[0] = 0x04
			var seen [][]byte
			err := backend.SendAndAwait(context.Background(), payload, func(report []byte) bool {
				seen = append(seen, append([]byte(nil), report...))
				return !protocol.MatchesDPIACK(report)
			})
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("SendAndAwait() error = %v", err)
				}
			} else if !IsErrorKind(err, tt.wantError) {
				t.Fatalf("SendAndAwait() error = %v, want %s", err, tt.wantError)
			}
			if tt.invalidX6 {
				if got := opener.opens(); got != 0 {
					t.Fatalf("hidraw opens = %d, want 0 for unvalidated source", got)
				}
				return
			}
			if got := opener.opens(); got != 1 {
				t.Fatalf("hidraw opens = %d, want 1", got)
			}
			if tt.writeErr == nil && !bytes.Equal(node.written(), payload) {
				t.Fatalf("written report does not match payload")
			}
			if got := len(seen); got != tt.wantReports {
				t.Fatalf("reports seen = %d, want %d", got, tt.wantReports)
			}
		})
	}
}

func TestHidrawSendAndAwaitCoordinatesWithListener(t *testing.T) {
	root := fixtureRoot(t)
	listenerNode := newCommandHidrawNode([][]byte{
		{0x03, 0x10, 0x40, 0x00, 8},
	})
	listenerNode.firstReadStarted = make(chan struct{})
	listenerNode.firstReadRelease = make(chan struct{})
	commandNode := newCommandHidrawNode([][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}})
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), nodes: []hidrawNode{listenerNode, commandNode}}
	backend := &HidrawBackend{
		sysRoot:     filepath.Join(root, "sys"),
		devRoot:     filepath.Join(root, "dev"),
		readTimeout: time.Second,
		opener:      opener,
	}
	if _, err := backend.Enumerate(context.Background(), transport.X6Match()); err != nil {
		t.Fatal(err)
	}
	source := transport.InputSource{Path: "1:1-4"}
	listenerCtx, cancelListener := context.WithCancel(context.Background())
	defer cancelListener()
	statusReports := make(chan []byte, 2)
	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- backend.ReadInterruptIN(listenerCtx, source, func(report []byte) bool {
			statusReports <- append([]byte(nil), report...)
			return true
		})
	}()
	<-listenerNode.firstReadStarted

	applyDone := make(chan error, 1)
	go func() {
		payload := make([]byte, protocol.DPIReportLength)
		payload[0] = 0x04
		applyDone <- backend.SendAndAwait(context.Background(), payload, func(report []byte) bool {
			return !protocol.MatchesDPIACK(report)
		})
	}()

	select {
	case <-commandNode.writeStarted:
		t.Fatal("Apply wrote while listener owned the read turn")
	case <-time.After(20 * time.Millisecond):
	}
	close(listenerNode.firstReadRelease)
	if err := <-applyDone; err != nil {
		t.Fatalf("SendAndAwait() error = %v", err)
	}
	cancelListener()
	<-listenerDone

	select {
	case report := <-statusReports:
		if !bytes.Equal(report, []byte{0x03, 0x10, 0x40, 0x00, 8}) {
			t.Fatalf("listener report = %x, want battery report", report)
		}
	default:
		t.Fatal("listener did not receive the non-ACK status report")
	}
	if got := listenerNode.maxInFlightValue(); got != 1 {
		t.Fatalf("maximum listener node I/O = %d, want 1", got)
	}
	if got := commandNode.maxInFlightValue(); got != 1 {
		t.Fatalf("maximum command node I/O = %d, want 1", got)
	}
}

type countingHidrawOpener struct {
	path  string
	node  hidrawNode
	nodes []hidrawNode
	mu    sync.Mutex
	count int
}

func (o *countingHidrawOpener) OpenNode(path string) (hidrawNode, error) {
	if path != o.path {
		return nil, errors.New("unexpected hidraw path")
	}
	o.mu.Lock()
	o.count++
	if len(o.nodes) > 0 {
		node := o.nodes[0]
		o.nodes = o.nodes[1:]
		o.mu.Unlock()
		return node, nil
	}
	o.mu.Unlock()
	return o.node, nil
}

func (o *countingHidrawOpener) opens() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.count
}

type commandHidrawNode struct {
	mu               sync.Mutex
	reports          [][]byte
	writeErr         error
	writtenReport    []byte
	readWait         chan struct{}
	firstReadStarted chan struct{}
	firstReadRelease chan struct{}
	writeStarted     chan struct{}
	readCount        int
	inFlight         int
	maxInFlight      int
	closeOnce        sync.Once
	closed           chan struct{}
}

func newCommandHidrawNode(reports [][]byte) *commandHidrawNode {
	return &commandHidrawNode{reports: reports, closed: make(chan struct{}), writeStarted: make(chan struct{})}
}

func (n *commandHidrawNode) Read(p []byte) (int, error) {
	n.mu.Lock()
	n.readCount++
	readNumber := n.readCount
	n.inFlight++
	if n.inFlight > n.maxInFlight {
		n.maxInFlight = n.inFlight
	}
	if readNumber == 1 && n.firstReadStarted != nil {
		close(n.firstReadStarted)
	}
	wait := n.readWait
	if readNumber == 1 && n.firstReadRelease != nil {
		wait = n.firstReadRelease
	}
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		n.inFlight--
		n.mu.Unlock()
	}()
	if wait != nil {
		select {
		case <-wait:
		case <-n.closed:
			return 0, context.DeadlineExceeded
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.reports) == 0 {
		return 0, io.EOF
	}
	report := n.reports[0]
	n.reports = n.reports[1:]
	return copy(p, report), nil
}

func (n *commandHidrawNode) Close() error {
	n.closeOnce.Do(func() { close(n.closed) })
	return nil
}

func (n *commandHidrawNode) SendFeatureReport(report []byte) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.inFlight++
	if n.inFlight > n.maxInFlight {
		n.maxInFlight = n.inFlight
	}
	defer func() { n.inFlight-- }()
	select {
	case <-n.writeStarted:
	default:
		close(n.writeStarted)
	}
	select {
	case <-n.closed:
		return 0, errors.New("hidraw node is closed")
	default:
	}
	if n.writeErr != nil {
		return 0, n.writeErr
	}
	n.writtenReport = append([]byte(nil), report...)
	return len(report), nil
}

func (n *commandHidrawNode) written() []byte {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]byte(nil), n.writtenReport...)
}

func (n *commandHidrawNode) maxInFlightValue() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.maxInFlight
}

func (n *commandHidrawNode) isClosed() bool {
	select {
	case <-n.closed:
		return true
	default:
		return false
	}
}
