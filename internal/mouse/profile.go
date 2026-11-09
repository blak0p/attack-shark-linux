// Package mouse defines model-neutral device identity and profile contracts.
package mouse

import (
	"errors"
	"fmt"

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
	if id.VendorID == 0 || id.ProductID == 0 {
		return ErrAmbiguousIdentity
	}
	return nil
}

func (id DeviceID) Key() string {
	if id.Serial == "" {
		return fmt.Sprintf("%04x:%04x", id.VendorID, id.ProductID)
	}
	return fmt.Sprintf("%04x:%04x:%s", id.VendorID, id.ProductID, id.Serial)
}

// HIDFacts contains the model-specific transport facts needed before device I/O.
type HIDFacts struct {
	StatusInput             transport.InputDescriptor
	ConfigurationReportSize int
	// Connection is the USB mode this profile's VID/PID represents.
	Connection transport.Connection
	// FeatureReports lists every feature report this model accepts for a plain
	// bound command; anything else is refused before device I/O.
	FeatureReports []FeatureReport
}

// FeatureReport is one accepted feature report shape: its ID and exact length.
type FeatureReport struct {
	ID     byte
	Length int
}

// AcceptsFeatureReport reports whether payload matches a declared feature report.
func (f HIDFacts) AcceptsFeatureReport(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	for _, report := range f.FeatureReports {
		if payload[0] == report.ID && len(payload) == report.Length {
			return true
		}
	}
	return false
}

// Codec contains model-specific configuration behavior behind a generic seam.
type Codec interface {
	Validate(any) error
	Encode(any) ([]byte, error)
	DecodeStatus([]byte) (any, bool)
	MatchesACK([]byte) bool
	Defaults() any
}

// CommandOperation describes one model-specific command without exposing its
// type or acknowledgement contract to the transport service.
type CommandOperation interface {
	Validate(any) error
	Encode(any) ([]byte, error)
	MatchesACK([]byte) bool
}

// Profile describes one supported VID/PID model without selecting a device.
type Profile interface {
	ID() string
	Match() transport.Match
	HIDFacts() HIDFacts
	Codec() Codec
	AllowsSeriallessIdentity() bool
}

// ProfileRegistry owns the unique VID/PID-to-profile mapping.
type ProfileRegistry struct {
	profiles map[transport.Match]Profile
	ordered  []Profile
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
		registry.ordered = append(registry.ordered, profile)
	}
	return registry, nil
}

func (r *ProfileRegistry) Lookup(vendorID, productID uint16) (Profile, bool) {
	profile, ok := r.profiles[transport.Match{VendorID: vendorID, ProductID: productID}]
	return profile, ok
}

// Matches lists every registered VID/PID in registration order.
func (r *ProfileRegistry) Matches() []transport.Match {
	matches := make([]transport.Match, 0, len(r.ordered))
	for _, profile := range r.ordered {
		matches = append(matches, profile.Match())
	}
	return matches
}

// WiredPairs derives the cable/dongle pairs of profiles sharing one ID.
func (r *ProfileRegistry) WiredPairs() []transport.WiredPair {
	var pairs []transport.WiredPair
	for _, wired := range r.ordered {
		if wired.HIDFacts().Connection != transport.Wired {
			continue
		}
		for _, dongle := range r.ordered {
			if dongle.ID() == wired.ID() && dongle.HIDFacts().Connection == transport.Dongle {
				pairs = append(pairs, transport.WiredPair{Wired: wired.Match(), Dongle: dongle.Match()})
			}
		}
	}
	return pairs
}
