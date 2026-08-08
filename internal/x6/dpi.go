package x6

import (
	"fmt"

	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
)

// DPIConfig represents the entire documented 0x04 configuration report.
type DPIConfig = protocol.DPIConfig

func DefaultDPIConfig() DPIConfig {
	return protocol.DefaultDPIConfig()
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
