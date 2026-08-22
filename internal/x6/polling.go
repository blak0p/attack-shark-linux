package x6

import (
	"encoding/json"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

// DeviceConfig is the durable X6 configuration. DPI remains flattened for
// backward-compatible reads of existing DPI-only records.
type DeviceConfig struct {
	DPIConfig
	PollingRate PollingRate `json:"pollingRate"`
}

func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{DPIConfig: DefaultDPIConfig(), PollingRate: PollingRate1000}
}

// UnmarshalJSON keeps old direct DPI records valid while supplying the polling
// factory default when their new field is absent.
func (c *DeviceConfig) UnmarshalJSON(contents []byte) error {
	*c = DefaultDeviceConfig()
	type plain DeviceConfig
	return json.Unmarshal(contents, (*plain)(c))
}

// PollingRate is the typed X6 polling-rate selection.
type PollingRate = protocol.PollingRate

const (
	PollingRate125  = protocol.PollingRate125
	PollingRate250  = protocol.PollingRate250
	PollingRate500  = protocol.PollingRate500
	PollingRate1000 = protocol.PollingRate1000
)

// NewPollingOperation adapts the X6 0x06 protocol contract to generic transport.
func NewPollingOperation() mouse.CommandOperation { return pollingOperation{} }

type pollingOperation struct{}

func (pollingOperation) Validate(value any) error {
	rate, ok := value.(PollingRate)
	if !ok {
		return fmt.Errorf("X6 polling rate has type %T", value)
	}
	return protocol.ValidatePollingRate(rate)
}

func (pollingOperation) Encode(value any) ([]byte, error) {
	rate, ok := value.(PollingRate)
	if !ok {
		return nil, fmt.Errorf("X6 polling rate has type %T", value)
	}
	return protocol.EncodePollingReport(rate)
}

func (pollingOperation) MatchesACK(report []byte) bool { return protocol.MatchesPollingACK(report) }
