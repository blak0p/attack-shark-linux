package x6

import (
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

type LightingMode = protocol.LightingMode
type LightingTemplateID = protocol.LightingTemplateID
type LightingSelection = protocol.LightingSelection

type LightingOption struct {
	Mode       LightingMode
	TemplateID LightingTemplateID
	Label      string
	CSSColor   string
}

const (
	LightingOff       = protocol.LightingOff
	LightingFixed     = protocol.LightingFixed
	LightingBreathing = protocol.LightingBreathing

	LightingTemplateOff             = protocol.LightingTemplateOff
	LightingTemplateFixedGreen      = protocol.LightingTemplateFixedGreen
	LightingTemplateBreathingGreen  = protocol.LightingTemplateBreathingGreen
	LightingTemplateBreathingFE5EF9 = protocol.LightingTemplateBreathingFE5EF9
	LightingTemplateBreathingFF7F00 = protocol.LightingTemplateBreathingFF7F00
	LightingTemplateBreathingFFFF00 = protocol.LightingTemplateBreathingFFFF00
)

// NewLightingOperation adapts the closed X6 lighting catalog to generic transport.
func NewLightingOperation() mouse.CommandOperation { return lightingOperation{} }

type lightingOperation struct{}

var lightingOptions = []LightingOption{
	{Mode: LightingOff, TemplateID: LightingTemplateOff, Label: "Off"},
	{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Label: "Fixed Green", CSSColor: "#00FF00"},
	{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingGreen, Label: "Breathing Green", CSSColor: "#00FF00"},
	{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFE5EF9, Label: "Breathing #FE5EF9", CSSColor: "#FE5EF9"},
	{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00, Label: "Breathing #FF7F00", CSSColor: "#FF7F00"},
	{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFFFF00, Label: "Breathing #FFFF00", CSSColor: "#FFFF00"},
}

func LightingOptions() []LightingOption { return append([]LightingOption(nil), lightingOptions...) }

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
