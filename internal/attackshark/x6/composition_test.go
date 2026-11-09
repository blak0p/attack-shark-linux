package x6

import (
	"context"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestNewDesktopServicesSharesOneAdapterAcrossStatusAndCommand(t *testing.T) {
	adapter := desktopAdapterFake{}
	status, command := NewDesktopServices(adapter)

	if status.transport != adapter {
		t.Fatal("status service must receive the shared adapter")
	}
	if command.command != adapter {
		t.Fatal("command service must receive the shared adapter")
	}
}

type desktopAdapterFake struct{}

func (desktopAdapterFake) Enumerate(context.Context, transport.Match) ([]transport.Candidate, error) {
	return nil, nil
}

func (desktopAdapterFake) ValidateDescriptor(context.Context, transport.Candidate, transport.InputDescriptor) (transport.InputSource, error) {
	return transport.InputSource{}, nil
}

func (desktopAdapterFake) ReadInterruptIN(context.Context, transport.InputSource, func([]byte) bool) error {
	return nil
}

func (desktopAdapterFake) SendAndAwait(context.Context, []byte, func([]byte) bool) error { return nil }
