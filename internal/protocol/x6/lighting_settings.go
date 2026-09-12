package x6

import "fmt"

const (
	ResponseTimeFactoryMs = 8
	ResponseTimeMinMs     = 2
	ResponseTimeMaxMs     = 32
	ResponseTimeStepMs    = 2
)

// LightingSettings is the complete editable subset of report 0x05.
type LightingSettings struct {
	LightingSelection
	NormalSleepMinutes float64
	ResponseTimeMs     int
}

func DefaultLightingSettings() LightingSettings {
	return LightingSettings{LightingSelection: LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen}, NormalSleepMinutes: NormalSleepMinimumMinutes, ResponseTimeMs: ResponseTimeFactoryMs}
}

func ValidateResponseTime(responseTimeMs int) error {
	if responseTimeMs < ResponseTimeMinMs || responseTimeMs > ResponseTimeMaxMs || responseTimeMs%ResponseTimeStepMs != 0 {
		return fmt.Errorf("X6 response time must be %d-%d ms in %d ms steps, got %d", ResponseTimeMinMs, ResponseTimeMaxMs, ResponseTimeStepMs, responseTimeMs)
	}
	return nil
}

// EncodeLightingSettingsReport preserves the selected report 0x05 lighting
// template while encoding normal sleep in byte 9 and response time in byte 10.
func EncodeLightingSettingsReport(settings LightingSettings) ([]byte, error) {
	if err := ValidateNormalSleepMinutes(settings.NormalSleepMinutes); err != nil { return nil, err }
	if err := ValidateResponseTime(settings.ResponseTimeMs); err != nil { return nil, err }
	report, err := NewLightingOperation(settings.LightingSelection)
	if err != nil { return nil, err }
	report[9] = byte(settings.NormalSleepMinutes * 2)
	report[10] = byte(settings.ResponseTimeMs / 2)
	var checksum byte
	for _, value := range report[3:11] { checksum += value }
	report[12] = checksum
	return report, nil
}

func MatchesLightingSettingsACK(report []byte) bool { return MatchesLightingACK(report) }
