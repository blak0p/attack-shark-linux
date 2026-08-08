// Package x6 contains pure, hardware-free facts about the X6 protocol.
package x6

import "fmt"

const DPIReportLength = 52

type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}

func DefaultDPIConfig() DPIConfig {
	// Factory template dictated by the user (docs/config-dictada.md): six stages
	// ending at the sensor max. Stages 7-8 hold the minimum encodable value but
	// are masked off by StageMask 0x3f. Colors match the factory dwords
	// 0xff/0xff00/0xff0000/0xffff/0xffff00/0xff00ff/0x40ff/0xffffff.
	return DPIConfig{StageMask: 0x3f, LiftDistance: 1, DPI: [8]int{800, 1200, 1600, 3200, 5600, 26000, 50, 50}, ActiveStage: 4, Colors: [8][3]byte{{0xff, 0, 0}, {0, 0xff, 0}, {0, 0, 0xff}, {0xff, 0xff, 0}, {0, 0xff, 0xff}, {0xff, 0, 0xff}, {0xff, 0x40, 0}, {0xff, 0xff, 0xff}}}
}

func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	if config.StageMask == 0 || config.LiftDistance > 1 || config.ActiveStage < 1 || config.ActiveStage > 8 {
		return nil, fmt.Errorf("invalid DPI configuration controls")
	}
	report := make([]byte, DPIReportLength)
	report[0], report[1], report[2] = 0x04, 0x38, 0x01
	if config.AngleControl {
		report[3], report[6] = 1, 1
	}
	if config.RippleControl {
		report[4] = 1
	}
	report[5], report[7], report[24], report[49] = config.StageMask, config.LiftDistance, config.ActiveStage, 1
	for stage, dpi := range config.DPI {
		if dpi < 50 || dpi%50 != 0 || dpi > 3276800 {
			return nil, fmt.Errorf("stage %d DPI %d is invalid", stage+1, dpi)
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

func MatchesDPIACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0 && report[4] == 0x04
}

func DecodeBatteryStatus(report []byte) (int, bool) {
	if len(report) != 5 || report[0] != 0x03 || report[1] != 0x10 || report[2] != 0x40 {
		return 0, false
	}
	return int(report[4]) * 10, true
}

// StatusReport is the dongle-pushed status event decoded from an interrupt
// 0x83 report. Exactly one field is reported per event: a heartbeat carries the
// battery, and a physical DPI button press carries the new active stage.
type StatusReport struct {
	Battery          int
	BatteryAvailable bool
	ActiveStage      byte
	StageAvailable   bool
}

// DecodeStatusReport decodes a dongle status report (report ID 0x03). It
// returns false for anything else, including the 0x50 configuration ACK.
func DecodeStatusReport(report []byte) (StatusReport, bool) {
	if len(report) != 5 || report[0] != 0x03 || report[1] != 0x10 {
		return StatusReport{}, false
	}
	switch report[2] {
	case 0x40:
		return StatusReport{Battery: int(report[4]) * 10, BatteryAvailable: true}, true
	case 0x10:
		return StatusReport{ActiveStage: report[3], StageAvailable: true}, true
	default:
		return StatusReport{}, false
	}
}
