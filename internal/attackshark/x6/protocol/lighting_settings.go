package protocol

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/device"
)

const (
	ResponseTimeFactoryMs = device.ResponseTimeFactoryMs
	ResponseTimeMinMs     = device.ResponseTimeMinMs
	ResponseTimeMaxMs     = device.ResponseTimeMaxMs
	ResponseTimeStepMs    = device.ResponseTimeStepMs
)

// LightingSettings is the complete editable subset of report 0x05.
type LightingSettings = device.LightingSettings

// DocumentedResetLightingSettings preserves the exact captured reset vector.
func DocumentedResetLightingSettings() LightingSettings {
	return device.DocumentedResetLightingSettings()
}

func DefaultLightingSettings() LightingSettings { return device.DefaultLightingSettings() }

func ValidateResponseTime(responseTimeMs int) error {
	if responseTimeMs < ResponseTimeMinMs || responseTimeMs > ResponseTimeMaxMs || responseTimeMs%ResponseTimeStepMs != 0 {
		return fmt.Errorf("X6 response time must be %d-%d ms in %d ms steps, got %d", ResponseTimeMinMs, ResponseTimeMaxMs, ResponseTimeStepMs, responseTimeMs)
	}
	return nil
}

// EncodeLightingSettingsReport preserves the selected report 0x05 lighting
// template while encoding normal sleep in byte 9 and response time in byte 10.
// Bytes 11-12 hold the big-endian 16-bit sum of bytes 3-10.
func EncodeLightingSettingsReport(settings LightingSettings) ([]byte, error) {
	if err := ValidateNormalSleepMinutes(settings.NormalSleepMinutes); err != nil {
		return nil, err
	}
	if err := ValidateResponseTime(settings.ResponseTimeMs); err != nil {
		return nil, err
	}
	report, err := NewLightingOperation(settings.LightingSelection)
	if err != nil {
		return nil, err
	}
	report[9] = byte(settings.NormalSleepMinutes * 2)
	report[10] = byte(settings.ResponseTimeMs)
	var checksum uint16
	for _, value := range report[3:11] {
		checksum += uint16(value)
	}
	report[11] = byte(checksum >> 8)
	report[12] = byte(checksum)
	return report, nil
}

func MatchesLightingSettingsACK(report []byte) bool { return MatchesLightingACK(report) }
