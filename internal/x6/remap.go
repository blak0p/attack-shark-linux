package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

type RemapAction = protocol.RemapAction
type RemapButton = protocol.RemapButton
type RemapConfig = protocol.RemapConfig

const (
	RemapReportLength = protocol.RemapReportLength
	RemapOff          = protocol.RemapOff
	RemapLeft         = protocol.RemapLeft
	RemapRight        = protocol.RemapRight
	RemapMiddle       = protocol.RemapMiddle
	RemapForward      = protocol.RemapForward
	RemapBackward     = protocol.RemapBackward
	RemapDoubleClick  = protocol.RemapDoubleClick
	RemapFire         = protocol.RemapFire
)

func DefaultRemapConfig() RemapConfig { return protocol.DefaultRemapConfig() }

func NewRemapOperation() mouse.CommandOperation { return remapOperation{} }

type remapOperation struct{}

func (remapOperation) Validate(value any) error {
	config, ok := value.(RemapConfig)
	if !ok {
		return fmt.Errorf("X6 remap configuration has type %T", value)
	}
	return protocol.ValidateRemapConfig(config)
}

func (remapOperation) Encode(value any) ([]byte, error) {
	config, ok := value.(RemapConfig)
	if !ok {
		return nil, fmt.Errorf("X6 remap configuration has type %T", value)
	}
	return protocol.EncodeRemapReport(config)
}

func (remapOperation) MatchesACK(report []byte) bool { return protocol.MatchesRemapACK(report) }
