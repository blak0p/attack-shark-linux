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
	// The fixture model's configuration feature report (ID and exact length).
	fixtureConfigID     = 0x04
	fixtureConfigLength = 56
)

// fixtureACK matches the fixture model's five-byte acknowledgement for a report ID.
func fixtureACK(id byte, report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0 && report[4] == id
}

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
	return mouse.HIDFacts{StatusInput: fixtureStatusInput, ConfigurationReportSize: fixtureConfigLength, Connection: p.connection, FeatureReports: []mouse.FeatureReport{
		{ID: fixtureConfigID, Length: fixtureConfigLength}, {ID: 0x06, Length: 9}, {ID: 0x05, Length: 13}, {ID: 0x08, Length: 59},
	}}
}
func (fixtureProfile) Codec() mouse.Codec             { return fixtureCodec{} }
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

// fixtureCodec carries the fixture model's default configuration report.
type fixtureCodec struct{}

func (fixtureCodec) Validate(any) error { return nil }
func (fixtureCodec) Encode(any) ([]byte, error) {
	return fixturePayload(fixtureConfigID, fixtureConfigLength), nil
}
func (fixtureCodec) DecodeStatus([]byte) (any, bool) { return nil, false }
func (fixtureCodec) MatchesACK(report []byte) bool   { return fixtureACK(fixtureConfigID, report) }
func (fixtureCodec) Defaults() any                   { return nil }
