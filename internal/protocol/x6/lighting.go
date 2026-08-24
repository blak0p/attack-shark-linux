package x6

import "fmt"

const LightingReportLength = 13

type LightingMode byte

const (
	LightingOff       LightingMode = 0x00
	LightingFixed     LightingMode = 0x10
	LightingBreathing LightingMode = 0x20
)

type LightingTemplateID string

const (
	LightingTemplateOff             LightingTemplateID = "off"
	LightingTemplateFixedGreen      LightingTemplateID = "fixed-green"
	LightingTemplateBreathingGreen  LightingTemplateID = "breathing-green"
	LightingTemplateBreathingFE5EF9 LightingTemplateID = "breathing-fe5ef9"
	LightingTemplateBreathingFF7F00 LightingTemplateID = "breathing-ff7f00"
	LightingTemplateBreathingFFFF00 LightingTemplateID = "breathing-ffff00"
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
	LightingTemplateOff:             {mode: LightingOff, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x00, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xaf}},
	LightingTemplateFixedGreen:      {mode: LightingFixed, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xbf}},
	LightingTemplateBreathingGreen:  {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xcf}},
	LightingTemplateBreathingFE5EF9: {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0xf9, 0x5e, 0xfe, 0x01, 0x04, 0x03, 0x25}},
	LightingTemplateBreathingFF7F00: {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x01, 0x04, 0x02, 0x4e}},
	LightingTemplateBreathingFFFF00: {mode: LightingBreathing, report: [LightingReportLength]byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0xff, 0x01, 0x04, 0x02, 0xce}},
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
