package x6

import "fmt"

const (
	dpiReportID     = 0x04
	dpiReportLength = 52
)

// DPIConfig is the complete configuration represented by report 0x04.
type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}

func DefaultDPIConfig() DPIConfig {
	return DPIConfig{StageMask: 0x3f, LiftDistance: 1, DPI: [8]int{800, 1200, 1600, 3200, 5600, 400, 50, 25650}, ActiveStage: 4, Colors: [8][3]byte{{0xff, 0, 0}, {0, 0xff, 0}, {0, 0, 0xff}, {0xff, 0xff, 0}, {0, 0xff, 0xff}, {0xff, 0, 0xff}, {0xff, 0x40, 0}, {0xff, 0xff, 0xff}}}
}

func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	if config.StageMask == 0 || config.LiftDistance > 1 || config.ActiveStage < 1 || config.ActiveStage > 8 {
		return nil, &ServiceError{InvalidDPI, fmt.Errorf("invalid DPI configuration controls")}
	}
	report := make([]byte, dpiReportLength)
	report[0], report[1], report[2] = dpiReportID, 0x38, 0x01
	if config.AngleControl {
		report[3], report[6] = 1, 1
	}
	if config.RippleControl {
		report[4] = 1
	}
	report[5], report[7], report[24], report[49] = config.StageMask, config.LiftDistance, config.ActiveStage, 1
	for stage, dpi := range config.DPI {
		if dpi < 50 || dpi%50 != 0 || dpi > 3276800 {
			return nil, &ServiceError{InvalidDPI, fmt.Errorf("stage %d DPI %d is invalid", stage+1, dpi)}
		}
		raw := uint16(dpi/50 - 1)
		report[8+stage], report[16+stage] = byte(raw), byte(raw>>8)
		copy(report[25+stage*3:], config.Colors[stage][:])
	}
	checksum := 0
	for _, value := range report[3:50] {
		checksum += int(value)
	}
	report[50], report[51] = byte(checksum>>8), byte(checksum)
	return report, nil
}

func matchesDPIACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0 && report[4] == dpiReportID
}
