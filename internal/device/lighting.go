package device

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
	LightingTemplateStaticDPIFactoryReset LightingTemplateID = "static-dpi-factory-reset"
	LightingTemplateOff                   LightingTemplateID = "off"
	LightingTemplateFixedGreen            LightingTemplateID = "fixed-green"
	LightingTemplateBreathingGreen        LightingTemplateID = "breathing-green"
	LightingTemplateBreathingFE5EF9       LightingTemplateID = "breathing-fe5ef9"
	LightingTemplateBreathingFF7F00       LightingTemplateID = "breathing-ff7f00"
	LightingTemplateBreathingFFFF00       LightingTemplateID = "breathing-ffff00"
	LightingTemplateStaticDPIDefault      LightingTemplateID = "static-dpi-default"
	LightingTemplateNeonOne               LightingTemplateID = "neon-one"
	LightingTemplateNeonTwo               LightingTemplateID = "neon-two"
	LightingTemplateColorBreathingOne     LightingTemplateID = "color-breathing-one"
	LightingTemplateColorBreathingTwo     LightingTemplateID = "color-breathing-two"
	LightingTemplateBreathingDPIOne       LightingTemplateID = "breathing-dpi-one"
	LightingTemplateBreathingDPITwo       LightingTemplateID = "breathing-dpi-two"
	LightingTemplateBreathingDPIThree     LightingTemplateID = "breathing-dpi-three"
)

type LightingSelection struct {
	Mode       LightingMode
	TemplateID LightingTemplateID
	// Color optionally overrides the template color as "#RRGGBB". Only modes
	// whose effect reports CustomColor accept it; empty keeps the template.
	Color string `json:",omitempty"`
}

// LightingSettings is the complete editable subset of report 0x05.
type LightingSettings struct {
	LightingSelection
	NormalSleepMinutes float64
	ResponseTimeMs     int
}

// DocumentedResetLightingSettings preserves the exact captured reset vector.
func DocumentedResetLightingSettings() LightingSettings {
	return LightingSettings{LightingSelection: LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIFactoryReset}, NormalSleepMinutes: 0.5, ResponseTimeMs: 4}
}

func DefaultLightingSettings() LightingSettings {
	return LightingSettings{LightingSelection: LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen}, NormalSleepMinutes: NormalSleepMinimumMinutes, ResponseTimeMs: ResponseTimeFactoryMs}
}

type LightingSpeedVariant struct {
	TemplateID LightingTemplateID
}

type LightingColorTemplate struct {
	TemplateID LightingTemplateID
	CSSColor   string
}

type LightingEffect struct {
	Mode              LightingMode
	Label             string
	DefaultTemplateID LightingTemplateID
	SpeedVariants     []LightingSpeedVariant
	ColorTemplates    []LightingColorTemplate
	// CustomColor reports that LightingSelection.Color is accepted.
	CustomColor bool
}
