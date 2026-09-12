package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

type LightingSettings = protocol.LightingSettings

const (
	ResponseTimeFactoryMs = protocol.ResponseTimeFactoryMs
	ResponseTimeMinMs = protocol.ResponseTimeMinMs
	ResponseTimeMaxMs = protocol.ResponseTimeMaxMs
	ResponseTimeStepMs = protocol.ResponseTimeStepMs
)

func DefaultLightingSettings() LightingSettings { return protocol.DefaultLightingSettings() }
func ValidateResponseTime(responseTimeMs int) error { return protocol.ValidateResponseTime(responseTimeMs) }

func NewLightingSettingsOperation() mouse.CommandOperation { return lightingSettingsOperation{} }
type lightingSettingsOperation struct{}
func (lightingSettingsOperation) Validate(value any) error {
	settings, ok := value.(LightingSettings)
	if !ok { return fmt.Errorf("X6 lighting settings have type %T", value) }
	_, err := protocol.EncodeLightingSettingsReport(settings)
	return err
}
func (lightingSettingsOperation) Encode(value any) ([]byte, error) {
	settings, ok := value.(LightingSettings)
	if !ok { return nil, fmt.Errorf("X6 lighting settings have type %T", value) }
	return protocol.EncodeLightingSettingsReport(settings)
}
func (lightingSettingsOperation) MatchesACK(report []byte) bool { return protocol.MatchesLightingSettingsACK(report) }
