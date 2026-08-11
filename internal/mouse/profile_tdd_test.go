package mouse

import (
	"errors"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

func TestDeviceIDRequiresStableSerial(t *testing.T) {
	valid := DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "X6-001"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got, want := valid.Key(), "1d57:fa60:X6-001"; got != want {
		t.Fatalf("Key() = %q, want %q", got, want)
	}

	missingSerial := DeviceID{VendorID: 0x1D57, ProductID: 0xFA60}
	if err := missingSerial.Validate(); !errors.Is(err, ErrAmbiguousIdentity) {
		t.Fatalf("Validate() error = %v, want ErrAmbiguousIdentity", err)
	}
}

func TestProfileRegistryRejectsVIDPIDCollisionsAndFindsProfile(t *testing.T) {
	profile := profileStub{id: "attack-shark-x6", match: transport.X6Match()}
	registry, err := NewProfileRegistry(profile)
	if err != nil {
		t.Fatalf("NewProfileRegistry() error = %v", err)
	}
	if got, ok := registry.Lookup(0x1D57, 0xFA60); !ok || got.ID() != profile.id {
		t.Fatalf("Lookup() = %#v, %t; want %q profile", got, ok, profile.id)
	}

	_, err = NewProfileRegistry(profile, profileStub{id: "replacement", match: transport.X6Match()})
	if !errors.Is(err, ErrProfileCollision) {
		t.Fatalf("NewProfileRegistry() collision error = %v, want ErrProfileCollision", err)
	}
}

type profileStub struct {
	id    string
	match transport.Match
}

func (p profileStub) ID() string             { return p.id }
func (p profileStub) Match() transport.Match { return p.match }
func (p profileStub) HIDFacts() HIDFacts     { return HIDFacts{} }
func (p profileStub) Codec() Codec           { return nil }
