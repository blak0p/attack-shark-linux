package x6

import "fmt"

const LightingReportLength = 13

type LightingMode byte

const (
	LightingOff            LightingMode = 0x00
	LightingFixed          LightingMode = 0x10
	LightingBreathing      LightingMode = 0x20
	LightingNeon           LightingMode = 0x30
	LightingColorBreathing LightingMode = 0x40
	LightingStaticDPI      LightingMode = 0x50
	LightingBreathingDPI   LightingMode = 0x60
)

type LightingTemplateID string

const (
	LightingTemplateOff               LightingTemplateID = "off"
	LightingTemplateFixedGreen        LightingTemplateID = "fixed-green"
	LightingTemplateBreathingGreen    LightingTemplateID = "breathing-green"
	LightingTemplateBreathingFE5EF9   LightingTemplateID = "breathing-fe5ef9"
	LightingTemplateBreathingFF7F00   LightingTemplateID = "breathing-ff7f00"
	LightingTemplateBreathingFFFF00   LightingTemplateID = "breathing-ffff00"
	LightingTemplateStaticDPIDefault  LightingTemplateID = "static-dpi-default"
	LightingTemplateNeonOne           LightingTemplateID = "neon-one"
	LightingTemplateNeonTwo           LightingTemplateID = "neon-two"
	LightingTemplateColorBreathingOne LightingTemplateID = "color-breathing-one"
	LightingTemplateColorBreathingTwo LightingTemplateID = "color-breathing-two"
	LightingTemplateBreathingDPIOne   LightingTemplateID = "breathing-dpi-one"
	LightingTemplateBreathingDPITwo   LightingTemplateID = "breathing-dpi-two"
	LightingTemplateBreathingDPIThree LightingTemplateID = "breathing-dpi-three"
)

type LightingSelection struct {
	Mode       LightingMode
	TemplateID LightingTemplateID
}

type lightingTemplate struct {
	mode   LightingMode
	report [LightingReportLength]byte
}

var lightingTemplates = map[LightingTemplateID]lightingTemplate{
	LightingTemplateOff:               {mode: LightingOff, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x00, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xaf}},
	LightingTemplateFixedGreen:        {mode: LightingFixed, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xbf}},
	LightingTemplateBreathingGreen:    {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xcf}},
	LightingTemplateBreathingFE5EF9:   {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0xf9, 0x5e, 0xfe, 0x01, 0x04, 0x03, 0x25}},
	LightingTemplateBreathingFF7F00:   {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x01, 0x04, 0x02, 0x4e}},
	LightingTemplateBreathingFFFF00:   {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0xff, 0x01, 0x04, 0x02, 0xce}},
	LightingTemplateStaticDPIDefault:  {mode: LightingStaticDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x50, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xfe}},
	LightingTemplateNeonOne:           {mode: LightingNeon, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x30, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xde}},
	LightingTemplateNeonTwo:           {mode: LightingNeon, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x30, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xdf}},
	LightingTemplateColorBreathingOne: {mode: LightingColorBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x40, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xef}},
	LightingTemplateColorBreathingTwo: {mode: LightingColorBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x40, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xee}},
	LightingTemplateBreathingDPIOne:   {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0e}},
	LightingTemplateBreathingDPITwo:   {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x04, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x10}},
	LightingTemplateBreathingDPIThree: {mode: LightingBreathingDPI, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x60, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0f}},
}

// NewLightingOperation validates a closed template selection and returns a copy
// of its complete capture-proven feature report.
func NewLightingOperation(selection LightingSelection) ([]byte, error) {
	template, ok := lightingTemplates[selection.TemplateID]
	if !ok || template.mode != selection.Mode {
		return nil, fmt.Errorf("unsupported lighting selection %q for mode %#x", selection.TemplateID, selection.Mode)
	}
	return append([]byte(nil), template.report[:]...), nil
}

// MatchesLightingACK accepts only the acknowledgement for report 0x05.
func MatchesLightingACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == 0x05
}
