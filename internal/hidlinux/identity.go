package hidlinux

import (
	"bytes"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// profileFor resolves a VID/PID through the registered model profiles. A nil
// registry knows no model, so every candidate is rejected.
func profileFor(registry *mouse.ProfileRegistry, vendorID, productID uint16) (mouse.Profile, bool) {
	if registry == nil {
		return nil, false
	}
	return registry.Lookup(vendorID, productID)
}

func registeredMatches(registry *mouse.ProfileRegistry) []transport.Match {
	if registry == nil {
		return nil
	}
	return registry.Matches()
}

func registeredWiredPairs(registry *mouse.ProfileRegistry) []transport.WiredPair {
	if registry == nil {
		return nil
	}
	return registry.WiredPairs()
}

// candidateConnection reports the USB mode of a registered candidate.
func candidateConnection(registry *mouse.ProfileRegistry, candidate Candidate) transport.Connection {
	if profile, ok := profileFor(registry, candidate.VendorID, candidate.ProductID); ok && profile.HIDFacts().Connection == transport.Wired {
		return transport.Wired
	}
	return transport.Dongle
}

// statusInputFor returns the registered status input of a VID/PID, or the zero
// descriptor when the identity is not registered.
func statusInputFor(registry *mouse.ProfileRegistry, vendorID, productID uint16) transport.InputDescriptor {
	if profile, ok := profileFor(registry, vendorID, productID); ok {
		return profile.HIDFacts().StatusInput
	}
	return transport.InputDescriptor{}
}

// statusEndpoints lists the endpoints inventory diagnostics should prefer: the
// candidate's own profile endpoint, or every registered one for foreign devices.
func statusEndpoints(registry *mouse.ProfileRegistry, candidate Candidate) []uint8 {
	if profile, ok := profileFor(registry, candidate.VendorID, candidate.ProductID); ok {
		return []uint8{profile.HIDFacts().StatusInput.EndpointAddress}
	}
	var endpoints []uint8
	for _, match := range registeredMatches(registry) {
		endpoints = append(endpoints, statusInputFor(registry, match.VendorID, match.ProductID).EndpointAddress)
	}
	return endpoints
}

// reportLengthAccepted reports whether n is a registered configuration report
// size; want is the size to quote in the error when it is not.
func reportLengthAccepted(registry *mouse.ProfileRegistry, n int) (want int, ok bool) {
	for i, match := range registeredMatches(registry) {
		profile, _ := profileFor(registry, match.VendorID, match.ProductID)
		size := profile.HIDFacts().ConfigurationReportSize
		if i == 0 {
			want = size
		}
		if size == n {
			return size, true
		}
	}
	return want, false
}

// hasTopLevelUsage reports whether the report descriptor opens the usage page
// and usage of the profile's status input.
func hasTopLevelUsage(descriptor []byte, input transport.InputDescriptor) bool {
	return bytes.Contains(descriptor, []byte{0x05, byte(input.UsagePage), 0x09, byte(input.Usage)})
}
