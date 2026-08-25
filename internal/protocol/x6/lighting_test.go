package x6

import (
	"bytes"
	"testing"
)

func TestLightingSelectionsUseOnlyCaptureBackedTemplates(t *testing.T) {
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
		{"static dpi", LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIDefault}, []byte{0x05, 0x0f, 0x01, 0x50, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xfe}},
		{"neon variant one", LightingSelection{Mode: LightingNeon, TemplateID: LightingTemplateNeonOne}, []byte{0x05, 0x0f, 0x01, 0x30, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xde}},
		{"neon variant two", LightingSelection{Mode: LightingNeon, TemplateID: LightingTemplateNeonTwo}, []byte{0x05, 0x0f, 0x01, 0x30, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xdf}},
		{"color breathing variant one", LightingSelection{Mode: LightingColorBreathing, TemplateID: LightingTemplateColorBreathingOne}, []byte{0x05, 0x0f, 0x01, 0x40, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xef}},
		{"color breathing variant two", LightingSelection{Mode: LightingColorBreathing, TemplateID: LightingTemplateColorBreathingTwo}, []byte{0x05, 0x0f, 0x01, 0x40, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x01, 0xee}},
		{"breathing dpi variant one", LightingSelection{Mode: LightingBreathingDPI, TemplateID: LightingTemplateBreathingDPIOne}, []byte{0x05, 0x0f, 0x01, 0x60, 0x02, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0e}},
		{"breathing dpi variant two", LightingSelection{Mode: LightingBreathingDPI, TemplateID: LightingTemplateBreathingDPITwo}, []byte{0x05, 0x0f, 0x01, 0x60, 0x04, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x10}},
		{"breathing dpi variant three", LightingSelection{Mode: LightingBreathingDPI, TemplateID: LightingTemplateBreathingDPIThree}, []byte{0x05, 0x0f, 0x01, 0x60, 0x03, 0xa8, 0x00, 0x00, 0xff, 0x01, 0x04, 0x02, 0x0f}},
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
		{Mode: LightingStaticDPI, TemplateID: LightingTemplateNeonOne},
		{Mode: LightingNeon, TemplateID: LightingTemplateColorBreathingOne},
		{Mode: LightingColorBreathing, TemplateID: LightingTemplateBreathingDPIOne},
		{Mode: LightingBreathingDPI, TemplateID: LightingTemplateStaticDPIDefault},
		{Mode: LightingStaticDPI, TemplateID: LightingTemplateID("static-dpi-stage-change")},
		{Mode: LightingMode(0x10), TemplateID: LightingTemplateID("old-mode")},
		{Mode: LightingNeon, TemplateID: ""},
	} {
		if report, err := NewLightingOperation(selection); err == nil || report != nil {
			t.Fatalf("NewLightingOperation(%#v) = % x, %v; want rejection", selection, report, err)
		}
	}

	first, err := NewLightingOperation(LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIDefault})
	if err != nil {
		t.Fatal(err)
	}
	first[3] = 0xff
	second, err := NewLightingOperation(LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIDefault})
	if err != nil || second[3] != 0x50 {
		t.Fatalf("second immutable report = % x, %v; want original static DPI template", second, err)
	}
}

func TestLightingACKMatchesOnlyTheExactLightingAcknowledgement(t *testing.T) {
	if !MatchesLightingACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) {
		t.Fatal("MatchesLightingACK() rejected the exact acknowledgement")
	}
	for _, report := range [][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}, {0x03, 0x10, 0x50, 0x00, 0x05, 0x00}, {0x03, 0x10, 0x40, 0x00, 0x05}} {
		if MatchesLightingACK(report) {
			t.Fatalf("MatchesLightingACK(% x) = true; want false", report)
		}
	}
}
