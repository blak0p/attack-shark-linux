package mouse

import (
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestDeviceIDUsesOptionalSerialInCanonicalKey(t *testing.T) {
	for _, tt := range []struct {
		name string
		id   DeviceID
		key  string
	}{
		{name: "serialless", id: DeviceID{VendorID: 0x1D57, ProductID: 0xFA60}, key: "1d57:fa60"},
		{name: "serial-bearing", id: DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "X6-001"}, key: "1d57:fa60:X6-001"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.id.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if got := tt.id.Key(); got != tt.key {
				t.Fatalf("Key() = %q, want %q", got, tt.key)
			}
		})
	}

	for _, id := range []DeviceID{
		{ProductID: 0xFA60, Serial: "X6-001"},
		{VendorID: 0x1D57, Serial: "X6-001"},
	} {
		if err := id.Validate(); !errors.Is(err, ErrAmbiguousIdentity) {
			t.Fatalf("Validate() error = %v, want ErrAmbiguousIdentity", err)
		}
	}
}

func TestProfileAuthorizesSeriallessIdentityExplicitly(t *testing.T) {
	for _, tt := range []struct {
		name       string
		profile    profileStub
		authorized bool
	}{
		{name: "authorized", profile: profileStub{seriallessDurable: true}, authorized: true},
		{name: "not authorized", profile: profileStub{}, authorized: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.AllowsSeriallessIdentity(); got != tt.authorized {
				t.Fatalf("AllowsSeriallessIdentity() = %t, want %t", got, tt.authorized)
			}
		})
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
	id                string
	match             transport.Match
	seriallessDurable bool
}

func (p profileStub) ID() string                     { return p.id }
func (p profileStub) Match() transport.Match         { return p.match }
func (p profileStub) HIDFacts() HIDFacts             { return HIDFacts{} }
func (p profileStub) Codec() Codec                   { return nil }
func (p profileStub) AllowsSeriallessIdentity() bool { return p.seriallessDurable }
