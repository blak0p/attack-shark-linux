package protocol

import (
	"bytes"
	"testing"
)

func TestLightingCustomColorWritesRGBBytesAndChecksum(t *testing.T) {
	tests := []struct {
		name      string
		selection LightingSelection
		want      []byte
	}{
		{"fixed orange", LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "#FF7F00"}, []byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0xff, 0x7f, 0x00, 0x01, 0x04, 0x02, 0x3e}},
		{"breathing yellow lowercase", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingGreen, Color: "#ffff00"}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0xff, 0xff, 0x00, 0x01, 0x04, 0x02, 0xce}},
		// The captured breathing-ff7f00 report is exactly breathing with RGB #007FFF.
		{"breathing matches captured template", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingGreen, Color: "#007FFF"}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x01, 0x04, 0x02, 0x4e}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLightingOperation(tt.selection)
			if err != nil {
				t.Fatalf("NewLightingOperation(%#v) error = %v", tt.selection, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("NewLightingOperation(%#v) = % x; want % x", tt.selection, got, tt.want)
			}
		})
	}
}

func TestLightingCustomColorRejectsUnsupportedModesAndMalformedColors(t *testing.T) {
	for _, selection := range []LightingSelection{
		{Mode: LightingOff, TemplateID: LightingTemplateOff, Color: "#FF0000"},
		{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIDefault, Color: "#FF0000"},
		{Mode: LightingNeon, TemplateID: LightingTemplateNeonOne, Color: "#FF0000"},
		{Mode: LightingColorBreathing, TemplateID: LightingTemplateColorBreathingOne, Color: "#FF0000"},
		{Mode: LightingBreathingDPI, TemplateID: LightingTemplateBreathingDPIOne, Color: "#FF0000"},
		{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "FF7F00"},
		{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "#FF7F0"},
		{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "#FF7F000"},
		{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "#GG0000"},
		{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "red"},
		{Mode: LightingBreathing, TemplateID: LightingTemplateFixedGreen, Color: "#FF0000"},
	} {
		if report, err := NewLightingOperation(selection); err == nil || report != nil {
			t.Fatalf("NewLightingOperation(%#v) = % x, %v; want rejection", selection, report, err)
		}
	}
}

func TestLightingSettingsKeepCustomColorAndRecomputeChecksum(t *testing.T) {
	settings := LightingSettings{LightingSelection: LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen, Color: "#FF7F00"}, NormalSleepMinutes: 0.5, ResponseTimeMs: 4}
	got, err := EncodeLightingSettingsReport(settings)
	want := []byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0xff, 0x7f, 0x00, 0x01, 0x04, 0x02, 0x3e}
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("EncodeLightingSettingsReport(%#v) = % x, %v; want % x", settings, got, err, want)
	}
}
