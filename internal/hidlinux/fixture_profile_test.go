package hidlinux

import (
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// The fixture model mimics a dongle/cable device without importing a driver.
const (
	fixtureVendorID       = 0x1D57
	fixtureDongleProduct  = 0xFA60
	fixtureWiredProduct   = 0xFA61
	fixtureStatusEndpoint = 0x83
)

var fixtureStatusInput = transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: fixtureStatusEndpoint}

type fixtureProfile struct {
	product    uint16
	connection transport.Connection
}

func (fixtureProfile) ID() string { return "fixture-model" }
func (p fixtureProfile) Match() transport.Match {
	return transport.Match{VendorID: fixtureVendorID, ProductID: p.product}
}
func (p fixtureProfile) HIDFacts() mouse.HIDFacts {
	return mouse.HIDFacts{StatusInput: fixtureStatusInput, ConfigurationReportSize: 56, Connection: p.connection}
}
func (fixtureProfile) Codec() mouse.Codec             { return nil }
func (fixtureProfile) AllowsSeriallessIdentity() bool { return true }

func fixtureDongleMatch() transport.Match {
	return transport.Match{VendorID: fixtureVendorID, ProductID: fixtureDongleProduct}
}

func fixtureRegistry(t testing.TB) *mouse.ProfileRegistry {
	t.Helper()
	registry, err := newFixtureRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func newFixtureRegistry() (*mouse.ProfileRegistry, error) {
	return mouse.NewProfileRegistry(
		fixtureProfile{product: fixtureDongleProduct, connection: transport.Dongle},
		fixtureProfile{product: fixtureWiredProduct, connection: transport.Wired},
	)
}
