package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

const x6ProfileID = "attack-shark-x6"

// NewProfile exposes the existing X6 validation and codec behavior through the
// model-neutral profile contract without changing X6 callers.
func NewProfile() mouse.Profile { return Profile{} }

type Profile struct{}

func (Profile) ID() string             { return x6ProfileID }
func (Profile) Match() transport.Match { return transport.X6Match() }
func (Profile) HIDFacts() mouse.HIDFacts {
	return mouse.HIDFacts{
		StatusInput:             transport.InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83},
		ConfigurationReportSize: protocol.DPIReportLength,
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
