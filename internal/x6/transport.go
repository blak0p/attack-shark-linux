package x6

import "context"

type Match struct{ VendorID, ProductID uint16 }
type Candidate struct {
	Path                                  string
	VendorID, ProductID, UsagePage, Usage uint16
	InterfaceNumber                       int
	Connection                            Connection
}
type Connection string

const (
	Dongle Connection = "dongle"
	Wired  Connection = "wired"
)

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
