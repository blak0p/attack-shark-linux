package protocol

import (
	"encoding/hex"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/device"
)

const LightingReportLength = 13

type LightingMode = device.LightingMode

const (
	LightingOff            = device.LightingOff
	LightingFixed          = device.LightingFixed
	LightingBreathing      = device.LightingBreathing
	LightingNeon           = device.LightingNeon
	LightingColorBreathing = device.LightingColorBreathing
	LightingStaticDPI      = device.LightingStaticDPI
	LightingBreathingDPI   = device.LightingBreathingDPI
)

type LightingTemplateID = device.LightingTemplateID

const (
	LightingTemplateStaticDPIFactoryReset = device.LightingTemplateStaticDPIFactoryReset
	LightingTemplateOff                   = device.LightingTemplateOff
	LightingTemplateFixedGreen            = device.LightingTemplateFixedGreen
	LightingTemplateBreathingGreen        = device.LightingTemplateBreathingGreen
	LightingTemplateBreathingFE5EF9       = device.LightingTemplateBreathingFE5EF9
	LightingTemplateBreathingFF7F00       = device.LightingTemplateBreathingFF7F00
	LightingTemplateBreathingFFFF00       = device.LightingTemplateBreathingFFFF00
	LightingTemplateStaticDPIDefault      = device.LightingTemplateStaticDPIDefault
	LightingTemplateNeonOne               = device.LightingTemplateNeonOne
	LightingTemplateNeonTwo               = device.LightingTemplateNeonTwo
	LightingTemplateColorBreathingOne     = device.LightingTemplateColorBreathingOne
	LightingTemplateColorBreathingTwo     = device.LightingTemplateColorBreathingTwo
	LightingTemplateBreathingDPIOne       = device.LightingTemplateBreathingDPIOne
	LightingTemplateBreathingDPITwo       = device.LightingTemplateBreathingDPITwo
	LightingTemplateBreathingDPIThree     = device.LightingTemplateBreathingDPIThree
)

type LightingSelection = device.LightingSelection

// customColorModes are the modes whose captured reports carry a single color
// in bytes 6-8. Hardware shows those bytes as RGB.
var customColorModes = map[LightingMode]bool{LightingFixed: true, LightingBreathing: true}

// SupportsCustomColor reports whether mode accepts LightingSelection.Color.
func SupportsCustomColor(mode LightingMode) bool { return customColorModes[mode] }

type lightingTemplate struct {
	mode   LightingMode
	report [LightingReportLength]byte
}

var lightingTemplates = map[LightingTemplateID]lightingTemplate{
	LightingTemplateStaticDPIFactoryReset: {mode: LightingStaticDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x50, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xff}},
	LightingTemplateOff:                   {mode: LightingOff, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x00, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xaf}},
	LightingTemplateFixedGreen:            {mode: LightingFixed, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xbf}},
	LightingTemplateBreathingGreen:        {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xcf}},
	LightingTemplateBreathingFE5EF9:       {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0xf9, 0x5e, 0xfe, 0x01, 0x04, 0x03, 0x25}},
	LightingTemplateBreathingFF7F00:       {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x01, 0x04, 0x02, 0x4e}},
	LightingTemplateBreathingFFFF00:       {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0xff, 0x01, 0x04, 0x02, 0xce}},
	LightingTemplateStaticDPIDefault:      {mode: LightingStaticDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x50, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xfe}},
	LightingTemplateNeonOne:               {mode: LightingNeon, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x30, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xde}},
	LightingTemplateNeonTwo:               {mode: LightingNeon, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x30, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xdf}},
	LightingTemplateColorBreathingOne:     {mode: LightingColorBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x40, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xef}},
	LightingTemplateColorBreathingTwo:     {mode: LightingColorBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x40, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xee}},
	LightingTemplateBreathingDPIOne:       {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0e}},
	LightingTemplateBreathingDPITwo:       {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x04, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x10}},
	LightingTemplateBreathingDPIThree:     {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0f}},
}

// NewLightingOperation validates a closed template selection and returns a copy
// of its complete capture-proven feature report.
func NewLightingOperation(selection LightingSelection) ([]byte, error) {
	template, ok := lightingTemplates[selection.TemplateID]
	if !ok || template.mode != selection.Mode {
		return nil, fmt.Errorf("unsupported lighting selection %q for mode %#x", selection.TemplateID, selection.Mode)
	}
	report := append([]byte(nil), template.report[:]...)
	if selection.Color == "" {
		return report, nil
	}
	if !customColorModes[selection.Mode] {
		return nil, fmt.Errorf("lighting mode %#x does not support a custom color", selection.Mode)
	}
	rgb, err := parseLightingColor(selection.Color)
	if err != nil {
		return nil, err
	}
	copy(report[6:9], rgb[:])
	var checksum uint16
	for _, value := range report[3:11] {
		checksum += uint16(value)
	}
	report[11] = byte(checksum >> 8)
	report[12] = byte(checksum)
	return report, nil
}

func parseLightingColor(color string) ([3]byte, error) {
	var rgb [3]byte
	if len(color) != 7 || color[0] != '#' {
		return rgb, fmt.Errorf("lighting color must be #RRGGBB, got %q", color)
	}
	if _, err := hex.Decode(rgb[:], []byte(color[1:])); err != nil {
		return rgb, fmt.Errorf("lighting color must be #RRGGBB, got %q", color)
	}
	return rgb, nil
}

// MatchesLightingACK accepts only the acknowledgement for report 0x05.
func MatchesLightingACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == 0x05
}
