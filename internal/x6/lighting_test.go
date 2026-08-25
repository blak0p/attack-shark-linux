package x6

import (
	"bytes"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

func TestLightingOperationAdaptsOnlyValidatedSelections(t *testing.T) {
	selection := LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00}
	operation := NewLightingOperation()
	var _ mouse.CommandOperation = operation

	if err := operation.Validate(selection); err != nil {
		t.Fatalf("Validate(%#v) error = %v", selection, err)
	}
	got, err := operation.Encode(selection)
	want, wantErr := protocol.NewLightingOperation(protocol.LightingSelection(selection))
	if err != nil || wantErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("Encode(%#v) = % x, %v; want % x, %v", selection, got, err, want, wantErr)
	}
	if err := operation.Validate(LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateBreathingGreen}); err == nil {
		t.Fatal("Validate() accepted an unsupported mode/template pair")
	}
	if !operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) || operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) {
		t.Fatal("MatchesACK() did not isolate the lighting acknowledgement")
	}
}

func TestLightingEffectsExposeBatchOneColorsAndBatchTwoSpeeds(t *testing.T) {
	effects := LightingEffects()
	if len(effects) != 7 {
		t.Fatalf("LightingEffects() returned %d effects; want 7", len(effects))
	}
	want := []struct {
		mode       LightingMode
		label      string
		defaultID  LightingTemplateID
		speedCount int
		colorCount int
	}{{LightingOff, "Off", LightingTemplateOff, 0, 0}, {LightingFixed, "Fixed", LightingTemplateFixedGreen, 0, 1}, {LightingBreathing, "Breathing", LightingTemplateBreathingGreen, 0, 4}, {LightingStaticDPI, "Static DPI", LightingTemplateStaticDPIDefault, 0, 0}, {LightingNeon, "Neon", LightingTemplateNeonOne, 2, 0}, {LightingColorBreathing, "Color Breathing", LightingTemplateColorBreathingOne, 2, 0}, {LightingBreathingDPI, "Breathing DPI", LightingTemplateBreathingDPIOne, 3, 0}}
	for index, expected := range want {
		effect := effects[index]
		if effect.Mode != expected.mode || effect.Label != expected.label || effect.DefaultTemplateID != expected.defaultID || len(effect.SpeedVariants) != expected.speedCount || len(effect.ColorTemplates) != expected.colorCount {
			t.Fatalf("effect %d = %#v; want mode=%#x label=%q default=%q speeds=%d colors=%d", index, effect, expected.mode, expected.label, expected.defaultID, expected.speedCount, expected.colorCount)
		}
		for _, variant := range effect.SpeedVariants {
			if variant.TemplateID == "" {
				t.Fatalf("effect %q exposed an incomplete speed template: %#v", effect.Label, variant)
			}
		}
	}
	if got := effects[2].ColorTemplates; len(got) != 4 || got[0].CSSColor != "#00FF00" || got[1].CSSColor != "#FE5EF9" || got[2].CSSColor != "#FF7F00" || got[3].CSSColor != "#FFFF00" {
		t.Fatalf("breathing color templates = %#v; want exact capture-backed colors", got)
	}
	if got := effects[6].SpeedVariants; len(got) != 3 || got[0].TemplateID != LightingTemplateBreathingDPIOne || got[1].TemplateID != LightingTemplateBreathingDPITwo || got[2].TemplateID != LightingTemplateBreathingDPIThree {
		t.Fatalf("breathing DPI speed templates = %#v; want capture order", got)
	}
	effects[1].ColorTemplates[0].CSSColor = "mutated"
	if LightingEffects()[1].ColorTemplates[0].CSSColor == "mutated" {
		t.Fatal("LightingEffects returned a mutable catalog copy")
	}
}
