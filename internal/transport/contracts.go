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

func X6Match() Match      { return Match{VendorID: 0x1D57, ProductID: 0xFA60} }
func X6WiredMatch() Match { return Match{VendorID: 0x1D57, ProductID: 0xFA61} }

// X6Enumerator discovers both supported USB modes in one coherent scan.
// It is optional so existing single-match transports remain compatible.
type X6Enumerator interface {
	EnumerateX6(context.Context) ([]Candidate, error)
}

func EnumerateX6(ctx context.Context, backend PassiveInputTransport) ([]Candidate, error) {
	if combined, ok := backend.(X6Enumerator); ok {
		return combined.EnumerateX6(ctx)
	}
	return backend.Enumerate(ctx, X6Match())
}
