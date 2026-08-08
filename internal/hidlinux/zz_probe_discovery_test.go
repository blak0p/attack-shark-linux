//go:build linux && probe

package hidlinux

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

func TestZZProbeHidrawDiscoveryAgainstRealSysfs(t *testing.T) {
	backend := NewHidrawBackend()
	candidates, err := backend.Enumerate(context.Background(), transport.X6Match())
	if err != nil {
		t.Fatalf("Enumerate error: %v", err)
	}
	t.Logf("candidates: %+v", candidates)
	source, err := backend.ValidateDescriptor(context.Background(), candidates[0], transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83})
	if err != nil {
		t.Fatalf("ValidateDescriptor error: %v", err)
	}
	t.Logf("validated source: %+v", source)
	backend.mu.Lock()
	candidate := backend.sources[source.Path]
	backend.mu.Unlock()
	hidDir, err := backend.hidrawDeviceDir(candidate)
	if err != nil {
		t.Fatalf("hidrawDeviceDir error: %v", err)
	}
	t.Logf("HID dir: %s", filepath.Base(hidDir))
	path, err := backend.hidrawPath(candidate)
	if err != nil {
		t.Fatalf("hidrawPath error: %v", err)
	}
	t.Logf("resolved hidraw node: %s", path)

	node, err := backend.opener.OpenNode(path)
	if err != nil {
		t.Fatalf("open hidraw node: %v", err)
	}
	defer node.Close()
	buffer := make([]byte, 64)
	bounded, cancel := context.WithTimeout(context.Background(), statusReadDeadline)
	defer cancel()
	result := make(chan [2]any, 1)
	go func() {
		n, readErr := node.Read(buffer)
		result <- [2]any{n, readErr}
	}()
	select {
	case r := <-result:
		n, nOK := r[0].(int)
		readErr, errOK := r[1].(error)
		if !nOK {
			t.Fatalf("unexpected read result: %v", r)
		}
		if !errOK {
			readErr = nil
		}
		if readErr != nil {
			t.Fatalf("hidraw read error: %v", readErr)
		}
		t.Logf("read %d bytes: % x", n, buffer[:n])
	case <-bounded.Done():
		t.Fatalf("hidraw read timed out (%v)", bounded.Err())
	}
}
