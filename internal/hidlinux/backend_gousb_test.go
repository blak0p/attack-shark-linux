package hidlinux

import (
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/transport"
	"github.com/google/gousb"
)

func TestAdapterBackendBuildsOneSharedTransportAdapter(t *testing.T) {
	backend := newGousbBackend(&gousbContextFake{})
	adapter := NewGousbAdapter(backend)

	var passive transport.PassiveInputTransport = adapter
	var command transport.CommandTransport = adapter
	if passive == nil || command == nil {
		t.Fatal("NewGousbAdapter() must provide one non-nil adapter for both transport contracts")
	}
}

func TestGousbBackendCloseIsIdempotent(t *testing.T) {
	context := &gousbContextFake{}
	backend := newGousbBackend(context)

	if err := backend.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if context.closes != 1 {
		t.Fatalf("context Close() calls = %d, want 1", context.closes)
	}
}

type gousbContextFake struct{ closes int }

func (f *gousbContextFake) Close() error {
	f.closes++
	return nil
}

func (*gousbContextFake) OpenDevices(func(*gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
	return nil, nil
}
