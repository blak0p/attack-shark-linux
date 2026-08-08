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
	Path                                  string
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

func X6Match() Match { return Match{VendorID: 0x1D57, ProductID: 0xFA60} }
