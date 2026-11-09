package x6

import "github.com/blak0p/attack-shark-linux/internal/transport"

type Match = transport.Match
type Candidate = transport.Candidate
type Connection = transport.Connection
type InputDescriptor = transport.InputDescriptor
type InputSource = transport.InputSource
type PassiveInputTransport = transport.PassiveInputTransport
type CommandTransport = transport.CommandTransport

const (
	Dongle = transport.Dongle
	Wired  = transport.Wired
)

type AppliedDPIStore interface{ SaveApplied(DPIConfig) error }
