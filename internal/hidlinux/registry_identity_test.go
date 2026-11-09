package hidlinux

import (
	"context"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type otherModelProfile struct{ fixtureProfile }

func (otherModelProfile) Match() transport.Match { return transport.Match{VendorID: 0x0A0A, ProductID: 0x0B0B} }
func (otherModelProfile) HIDFacts() mouse.HIDFacts {
	return mouse.HIDFacts{
		StatusInput:             transport.InputDescriptor{InterfaceNumber: 1, UsagePage: 0x0C, Usage: 0x01, EndpointAddress: 0x81},
		ConfigurationReportSize: 32,
		Connection:              transport.Dongle,
	}
}

func TestAdapterIdentityComesFromRegisteredProfile(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(otherModelProfile{})
	if err != nil {
		t.Fatal(err)
	}
	facts := otherModelProfile{}.HIDFacts()
	other := Candidate{
		VendorID: 0x0A0A, ProductID: 0x0B0B, Bus: 1, PortPath: "1-3",
		Interfaces: []InterfaceDescriptor{{Number: 1, AlternateSetting: 0, Class: hidClass, Endpoints: []uint8{0x81}}},
	}
	discovery := &statusDiscovery{candidates: []Candidate{other, validCandidate()}}
	adapter := NewStatusAdapter(discovery, &statusUSB{}, fakeSysfs{descriptor: []byte{0x05, 0x0C, 0x09, 0x01}}, registry)

	found, err := adapter.Enumerate(context.Background(), otherModelProfile{}.Match())
	if err != nil || len(found) != 1 {
		t.Fatalf("Enumerate() = %v, %v; want the registered model only", found, err)
	}
	if _, err := adapter.ValidateDescriptor(context.Background(), found[0], facts.StatusInput); err != nil {
		t.Fatalf("ValidateDescriptor() error = %v", err)
	}
	if _, err := adapter.ValidateDescriptor(context.Background(), found[0], fixtureStatusInput); !IsErrorKind(err, Mismatch) {
		t.Fatalf("ValidateDescriptor() with another model's input error = %v, want mismatch", err)
	}
	if _, err := adapter.Enumerate(context.Background(), fixtureDongleMatch()); !IsErrorKind(err, NotFound) {
		t.Fatalf("Enumerate() of an unregistered identity error = %v, want not found", err)
	}
}

func TestEmptyRegistryRejectsEveryCandidate(t *testing.T) {
	registry, err := mouse.NewProfileRegistry()
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewStatusAdapter(&statusDiscovery{candidates: []Candidate{validCandidate()}}, &statusUSB{}, fakeSysfs{descriptor: x6ReportDescriptor}, registry)
	if _, err := adapter.Enumerate(context.Background(), fixtureDongleMatch()); !IsErrorKind(err, NotFound) {
		t.Fatalf("Enumerate() error = %v, want not found", err)
	}
}
