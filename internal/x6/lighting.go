package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

type LightingMode = protocol.LightingMode
type LightingTemplateID = protocol.LightingTemplateID
type LightingSelection = protocol.LightingSelection

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
}

const (
	LightingOff            = protocol.LightingOff
	LightingFixed          = protocol.LightingFixed
	LightingBreathing      = protocol.LightingBreathing
	LightingNeon           = protocol.LightingNeon
	LightingColorBreathing = protocol.LightingColorBreathing
	LightingStaticDPI      = protocol.LightingStaticDPI
	LightingBreathingDPI   = protocol.LightingBreathingDPI

	LightingTemplateOff               = protocol.LightingTemplateOff
	LightingTemplateFixedGreen        = protocol.LightingTemplateFixedGreen
	LightingTemplateBreathingGreen    = protocol.LightingTemplateBreathingGreen
	LightingTemplateBreathingFE5EF9   = protocol.LightingTemplateBreathingFE5EF9
	LightingTemplateBreathingFF7F00   = protocol.LightingTemplateBreathingFF7F00
	LightingTemplateBreathingFFFF00   = protocol.LightingTemplateBreathingFFFF00
	LightingTemplateStaticDPIDefault  = protocol.LightingTemplateStaticDPIDefault
	LightingTemplateNeonOne           = protocol.LightingTemplateNeonOne
	LightingTemplateNeonTwo           = protocol.LightingTemplateNeonTwo
	LightingTemplateColorBreathingOne = protocol.LightingTemplateColorBreathingOne
	LightingTemplateColorBreathingTwo = protocol.LightingTemplateColorBreathingTwo
	LightingTemplateBreathingDPIOne   = protocol.LightingTemplateBreathingDPIOne
	LightingTemplateBreathingDPITwo   = protocol.LightingTemplateBreathingDPITwo
	LightingTemplateBreathingDPIThree = protocol.LightingTemplateBreathingDPIThree
)

var lightingEffects = []LightingEffect{
	{Mode: LightingOff, Label: "Off", DefaultTemplateID: LightingTemplateOff},
	{Mode: LightingFixed, Label: "Fixed", DefaultTemplateID: LightingTemplateFixedGreen, ColorTemplates: []LightingColorTemplate{{TemplateID: LightingTemplateFixedGreen, CSSColor: "#00FF00"}}},
	{Mode: LightingBreathing, Label: "Breathing", DefaultTemplateID: LightingTemplateBreathingGreen, ColorTemplates: []LightingColorTemplate{
		{TemplateID: LightingTemplateBreathingGreen, CSSColor: "#00FF00"},
		{TemplateID: LightingTemplateBreathingFE5EF9, CSSColor: "#FE5EF9"},
		{TemplateID: LightingTemplateBreathingFF7F00, CSSColor: "#FF7F00"},
		{TemplateID: LightingTemplateBreathingFFFF00, CSSColor: "#FFFF00"},
	}},
	{Mode: LightingStaticDPI, Label: "Static DPI", DefaultTemplateID: LightingTemplateStaticDPIDefault},
	{Mode: LightingNeon, Label: "Neon", DefaultTemplateID: LightingTemplateNeonOne, SpeedVariants: []LightingSpeedVariant{{TemplateID: LightingTemplateNeonOne}, {TemplateID: LightingTemplateNeonTwo}}},
	{Mode: LightingColorBreathing, Label: "Color Breathing", DefaultTemplateID: LightingTemplateColorBreathingOne, SpeedVariants: []LightingSpeedVariant{{TemplateID: LightingTemplateColorBreathingOne}, {TemplateID: LightingTemplateColorBreathingTwo}}},
	{Mode: LightingBreathingDPI, Label: "Breathing DPI", DefaultTemplateID: LightingTemplateBreathingDPIOne, SpeedVariants: []LightingSpeedVariant{{TemplateID: LightingTemplateBreathingDPIOne}, {TemplateID: LightingTemplateBreathingDPITwo}, {TemplateID: LightingTemplateBreathingDPIThree}}},
}

// LightingEffects returns a deep copy of the closed capture-backed catalog.
func LightingEffects() []LightingEffect {
	effects := make([]LightingEffect, len(lightingEffects))
	for i, effect := range lightingEffects {
		effects[i] = effect
		effects[i].SpeedVariants = append([]LightingSpeedVariant(nil), effect.SpeedVariants...)
		effects[i].ColorTemplates = append([]LightingColorTemplate(nil), effect.ColorTemplates...)
	}
	return effects
}

// NewLightingOperation adapts the closed X6 lighting catalog to generic transport.
func NewLightingOperation() mouse.CommandOperation { return lightingOperation{} }

type lightingOperation struct{}

func (lightingOperation) Validate(value any) error {
	selection, ok := value.(LightingSelection)
	if !ok {
		return fmt.Errorf("X6 lighting selection has type %T", value)
	}
	_, err := protocol.NewLightingOperation(selection)
	return err
}

func (lightingOperation) Encode(value any) ([]byte, error) {
	selection, ok := value.(LightingSelection)
	if !ok {
		return nil, fmt.Errorf("X6 lighting selection has type %T", value)
	}
	return protocol.NewLightingOperation(selection)
}

func (lightingOperation) MatchesACK(report []byte) bool { return protocol.MatchesLightingACK(report) }
