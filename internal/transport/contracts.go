// Package transport owns generic transport identities without HID implementation.
package transport

import "context"

type Match struct{ VendorID, ProductID uint16 }

type Connection string

const (
	Dongle Connection = "dongle"
	Wired  Connection = "wired"
)

type Candidate struct {
	Path, Serial                          string
	VendorID, ProductID, UsagePage, Usage uint16
	InterfaceNumber                       int
	Connection                            Connection
}

type InputDescriptor struct {
	InterfaceNumber  int
	UsagePage, Usage uint16
	EndpointAddress  uint8
}

type InputSource struct{ Path string }

type PassiveInputTransport interface {
	Enumerate(context.Context, Match) ([]Candidate, error)
	ValidateDescriptor(context.Context, Candidate, InputDescriptor) (InputSource, error)
	ReadInterruptIN(context.Context, InputSource, func([]byte) bool) error
}

type CommandTransport interface {
	SendAndAwait(context.Context, []byte, func([]byte) bool) error
}

// MultiEnumerator discovers several USB identities in one coherent scan.
// It is optional so existing single-match transports remain compatible.
type MultiEnumerator interface {
	EnumerateMatches(context.Context, ...Match) ([]Candidate, error)
}

// Enumerator is the discovery half of PassiveInputTransport.
type Enumerator interface {
	Enumerate(context.Context, Match) ([]Candidate, error)
}

// EnumerateAll scans every match together when the backend supports it and
// otherwise falls back to the first match.
func EnumerateAll(ctx context.Context, backend Enumerator, matches ...Match) ([]Candidate, error) {
	if combined, ok := backend.(MultiEnumerator); ok {
		return combined.EnumerateMatches(ctx, matches...)
	}
	return backend.Enumerate(ctx, matches[0])
}

// WiredPair names the cable and dongle identities of one device model.
type WiredPair struct{ Wired, Dongle Match }

// PreferWired suppresses only the dongle in an eligible one-cable/one-dongle
// pair of the same model. Callers must validate candidates first. It neither
// merges identities nor resolves multiple devices of either USB mode; raw
// discovery stays unchanged.
func PreferWired(eligible []Candidate, pairs ...WiredPair) []Candidate {
	if len(eligible) != 2 {
		return eligible
	}
	for _, pair := range pairs {
		wired, dongle := -1, -1
		for i, c := range eligible {
			switch {
			case c.VendorID == pair.Wired.VendorID && c.ProductID == pair.Wired.ProductID && c.Connection == Wired:
				wired = i
			case c.VendorID == pair.Dongle.VendorID && c.ProductID == pair.Dongle.ProductID && c.Connection == Dongle:
				dongle = i
			}
		}
		if wired >= 0 && dongle >= 0 {
			return []Candidate{eligible[wired]}
		}
	}
	return eligible
}
