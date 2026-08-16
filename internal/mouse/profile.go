// Package mouse defines model-neutral device identity and profile contracts.
package mouse

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

var (
	ErrAmbiguousIdentity = errors.New("ambiguous device identity")
	ErrProfileCollision  = errors.New("profile registry collision")
)

// DeviceID is the stable identity for persisted device state. Path is deliberately
// excluded because it changes when the device topology changes.
type DeviceID struct {
	VendorID, ProductID uint16
	Serial              string
}

func (id DeviceID) Validate() error {
	if id.VendorID == 0 || id.ProductID == 0 || strings.TrimSpace(id.Serial) == "" {
		return ErrAmbiguousIdentity
	}
	return nil
}

func (id DeviceID) Key() string {
	return fmt.Sprintf("%04x:%04x:%s", id.VendorID, id.ProductID, id.Serial)
}

// HIDFacts contains the model-specific transport facts needed before device I/O.
type HIDFacts struct {
	StatusInput             transport.InputDescriptor
	ConfigurationReportSize int
}

// Codec contains model-specific configuration behavior behind a generic seam.
type Codec interface {
	Validate(any) error
	Encode(any) ([]byte, error)
	DecodeStatus([]byte) (any, bool)
	MatchesACK([]byte) bool
	Defaults() any
}

// Profile describes one supported VID/PID model without selecting a device.
type Profile interface {
	ID() string
	Match() transport.Match
	HIDFacts() HIDFacts
	Codec() Codec
}

// ProfileRegistry owns the unique VID/PID-to-profile mapping.
type ProfileRegistry struct {
	profiles map[transport.Match]Profile
}

func NewProfileRegistry(profiles ...Profile) (*ProfileRegistry, error) {
	registry := &ProfileRegistry{profiles: make(map[transport.Match]Profile, len(profiles))}
	for _, profile := range profiles {
		if profile == nil || profile.ID() == "" {
			return nil, fmt.Errorf("%w: profile is required", ErrProfileCollision)
		}
		match := profile.Match()
		if match.VendorID == 0 || match.ProductID == 0 {
			return nil, fmt.Errorf("%w: profile %q has an invalid VID/PID", ErrProfileCollision, profile.ID())
		}
		if _, exists := registry.profiles[match]; exists {
			return nil, fmt.Errorf("%w: VID:PID %04x:%04x", ErrProfileCollision, match.VendorID, match.ProductID)
		}
		registry.profiles[match] = profile
	}
	return registry, nil
}

func (r *ProfileRegistry) Lookup(vendorID, productID uint16) (Profile, bool) {
	profile, ok := r.profiles[transport.Match{VendorID: vendorID, ProductID: productID}]
	return profile, ok
}
