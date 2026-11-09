package x6

import (
	"fmt"

	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

const x6ProfileID = "attack-shark-x6"

// The X6 shares one vendor ID across its dongle and cable modes.
var (
	dongleMatch = transport.Match{VendorID: 0x1D57, ProductID: 0xFA60}
	wiredMatch  = transport.Match{VendorID: 0x1D57, ProductID: 0xFA61}
)

// NewProfile exposes the existing X6 validation and codec behavior through the
// model-neutral profile contract without changing X6 callers.
func NewProfile() mouse.Profile { return Profile{} }

// NewWiredProfile keeps the cable PID in identity while sharing the X6 codec.
func NewWiredProfile() mouse.Profile { return wiredProfile{} }

type wiredProfile struct{ Profile }

func (wiredProfile) Match() transport.Match { return wiredMatch }

func (p wiredProfile) HIDFacts() mouse.HIDFacts {
	facts := p.Profile.HIDFacts()
	facts.Connection = transport.Wired
	return facts
}

type Profile struct{}

func (Profile) ID() string                     { return x6ProfileID }
func (Profile) Match() transport.Match         { return dongleMatch }
func (Profile) AllowsSeriallessIdentity() bool { return true }
func (Profile) HIDFacts() mouse.HIDFacts {
	return mouse.HIDFacts{
		StatusInput:             transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83},
		ConfigurationReportSize: protocol.DPIReportLength,
		Connection:              transport.Dongle,
		FeatureReports: []mouse.FeatureReport{
			{ID: 0x04, Length: protocol.DPIReportLength},
			{ID: 0x06, Length: protocol.PollingReportLength},
			{ID: 0x05, Length: protocol.LightingReportLength},
			{ID: 0x08, Length: protocol.RemapReportLength},
		},
	}
}
func (Profile) Codec() mouse.Codec { return dpiCodec{} }

type dpiCodec struct{}

func (dpiCodec) Validate(configuration any) error {
	_, err := dpiConfig(configuration)
	return err
}

func (dpiCodec) Encode(configuration any) ([]byte, error) {
	config, err := dpiConfig(configuration)
	if err != nil {
		return nil, err
	}
	return EncodeDPIReport(config)
}

func (dpiCodec) DecodeStatus(report []byte) (any, bool) {
	return protocol.DecodeStatusReport(report)
}

func (dpiCodec) MatchesACK(report []byte) bool { return matchesDPIACK(report) }
func (dpiCodec) Defaults() any                 { return DefaultDPIConfig() }

func dpiConfig(configuration any) (DPIConfig, error) {
	config, ok := configuration.(DPIConfig)
	if !ok {
		return DPIConfig{}, fmt.Errorf("X6 configuration has type %T", configuration)
	}
	if _, err := EncodeDPIReport(config); err != nil {
		return DPIConfig{}, err
	}
	return config, nil
}
