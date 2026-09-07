package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

// DPIConfig represents the entire documented 0x04 configuration report.
type DPIConfig = protocol.DPIConfig

func DefaultDPIConfig() DPIConfig {
	return protocol.DefaultDPIConfig()
}

func DocumentedResetDPIConfig() DPIConfig {
	return protocol.DocumentedResetDPIConfig()
}

func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	report, err := protocol.EncodeDPIReport(config)
	if err != nil {
		return nil, &ServiceError{InvalidDPI, fmt.Errorf("%w", err)}
	}
	return report, nil
}

func matchesDPIACK(report []byte) bool {
	return protocol.MatchesDPIACK(report)
}

// NewDPIOperation adapts the X6 0x04 protocol contract to generic transport.
func NewDPIOperation() mouse.CommandOperation { return dpiOperation{} }

type dpiOperation struct{}

func (dpiOperation) Validate(value any) error {
	config, ok := value.(DPIConfig)
	if !ok {
		return fmt.Errorf("X6 DPI configuration has type %T", value)
	}
	_, err := protocol.EncodeDPIReport(config)
	return err
}

func (dpiOperation) Encode(value any) ([]byte, error) {
	config, ok := value.(DPIConfig)
	if !ok {
		return nil, fmt.Errorf("X6 DPI configuration has type %T", value)
	}
	return protocol.EncodeDPIReport(config)
}

func (dpiOperation) MatchesACK(report []byte) bool { return protocol.MatchesDPIACK(report) }
