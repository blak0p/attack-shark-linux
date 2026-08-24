package x6

import (
	"bytes"
	"testing"
)

func TestLightingSelectionsUseOnlyExactCaptureProvenTemplates(t *testing.T) {
	tests := []struct {
		name      string
		selection LightingSelection
		want      []byte
	}{
		{"off", LightingSelection{Mode: LightingOff, TemplateID: LightingTemplateOff}, []byte{0x05, 0x0f, 0x01, 0x00, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xaf}},
		{"fixed green", LightingSelection{Mode: LightingFixed, TemplateID: LightingTemplateFixedGreen}, []byte{0x05, 0x0f, 0x01, 0x10, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xbf}},
		{"breathing green", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingGreen}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xcf}},
		{"breathing fe5ef9", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFE5EF9}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0xf9, 0x5e, 0xfe, 0x01, 0x04, 0x03, 0x25}},
		{"breathing ff7f00", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x01, 0x04, 0x02, 0x4e}},
		{"breathing ffff00", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFFFF00}, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0xff, 0x01, 0x04, 0x02, 0xce}},
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

func TestLightingSelectionsRejectUnsupportedPairsAndReturnImmutableCopies(t *testing.T) {
	for _, selection := range []LightingSelection{
		{Mode: LightingOff, TemplateID: LightingTemplateFixedGreen},
		{Mode: LightingFixed, TemplateID: LightingTemplateBreathingGreen},
		{Mode: LightingBreathing, TemplateID: LightingTemplateOff},
		{Mode: LightingMode(0x30), TemplateID: LightingTemplateID("wave")},
	} {
		if report, err := NewLightingOperation(selection); err == nil || report != nil {
			t.Fatalf("NewLightingOperation(%#v) = % x, %v; want rejection", selection, report, err)
		}
	}

	first, err := NewLightingOperation(LightingSelection{Mode: LightingOff, TemplateID: LightingTemplateOff})
	if err != nil {
		t.Fatal(err)
	}
	first[3] = 0xff
	second, err := NewLightingOperation(LightingSelection{Mode: LightingOff, TemplateID: LightingTemplateOff})
	if err != nil || second[3] != 0x00 {
		t.Fatalf("second immutable report = % x, %v; want original off template", second, err)
	}
}

func TestLightingACKMatchesOnlyTheExactLightingAcknowledgement(t *testing.T) {
	if !MatchesLightingACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) {
		t.Fatal("MatchesLightingACK() rejected the exact acknowledgement")
	}
	for _, report := range [][]byte{
		{0x03, 0x10, 0x50, 0x00, 0x04},
		{0x03, 0x10, 0x50, 0x00, 0x05, 0x00},
		{0x03, 0x10, 0x40, 0x00, 0x05},
	} {
		if MatchesLightingACK(report) {
			t.Fatalf("MatchesLightingACK(% x) = true; want false", report)
		}
	}
}
