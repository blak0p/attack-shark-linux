package transport

import (
	"context"
	"testing"
)

func TestX6MatchUsesCapturedDeviceIdentity(t *testing.T) {
	match := X6Match()
	if match.VendorID != 0x1D57 || match.ProductID != 0xFA60 {
		t.Fatalf("X6Match() = %#v, want Attack Shark X6 identity", match)
	}
}

func TestGenericContractsRepresentPassiveAndCommandTransport(t *testing.T) {
	var _ PassiveInputTransport = passiveTransportStub{}
	var _ CommandTransport = commandTransportStub{}

	candidate := Candidate{Path: "/dev/hidraw0", Connection: Dongle}
	descriptor := InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}
	if candidate.Connection != Dongle || descriptor.EndpointAddress != 0x83 {
		t.Fatalf("generic transport values = %#v, %#v", candidate, descriptor)
	}
}

func TestCandidateCarriesStableSerialAlongsideTransientPath(t *testing.T) {
	candidate := Candidate{Path: "/dev/hidraw0", VendorID: 0x1D57, ProductID: 0xFA60, Serial: "X6-001"}
	if candidate.Serial != "X6-001" {
		t.Fatalf("Candidate.Serial = %q, want stable device serial", candidate.Serial)
	}
}

type passiveTransportStub struct{}

func (passiveTransportStub) Enumerate(context.Context, Match) ([]Candidate, error) { return nil, nil }
func (passiveTransportStub) ValidateDescriptor(context.Context, Candidate, InputDescriptor) (InputSource, error) {
	return InputSource{}, nil
}
func (passiveTransportStub) ReadInterruptIN(context.Context, InputSource, func([]byte) bool) error {
	return nil
}

type commandTransportStub struct{}

func (commandTransportStub) SendAndAwait(context.Context, []byte, func([]byte) bool) error {
	return nil
}
