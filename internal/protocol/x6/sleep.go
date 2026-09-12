package x6

import "fmt"

const (
	NormalSleepMinimumMinutes = 0.5
	NormalSleepMaximumMinutes = 60.0
)

func ValidateNormalSleepMinutes(minutes float64) error {
	units := minutes * 2
	if minutes < NormalSleepMinimumMinutes || minutes > NormalSleepMaximumMinutes || units != float64(int(units)) {
		return fmt.Errorf("normal sleep duration %v must be between %.1f and %.1f minutes in 0.5-minute steps", minutes, NormalSleepMinimumMinutes, NormalSleepMaximumMinutes)
	}
	return nil
}

// EncodeNormalSleepReport remains available for callers that only update byte 9.
func EncodeNormalSleepReport(base []byte, minutes float64) ([]byte, error) {
	if err := ValidateNormalSleepMinutes(minutes); err != nil { return nil, err }
	if len(base) != LightingReportLength || base[0] != 0x05 { return nil, fmt.Errorf("normal sleep requires a %d-byte lighting report", LightingReportLength) }
	report := append([]byte(nil), base...)
	report[9] = byte(minutes * 2)
	var checksum byte
	for _, value := range report[3:11] { checksum += value }
	report[12] = checksum
	return report, nil
}

func MatchesNormalSleepACK(report []byte) bool { return MatchesLightingACK(report) }
