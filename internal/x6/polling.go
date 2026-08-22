package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

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
