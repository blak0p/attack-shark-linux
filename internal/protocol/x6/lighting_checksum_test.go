package x6

import (
	"bytes"
	"testing"
)

func TestLightingSettingsChecksumCarry(t *testing.T) {
	for _, tt := range []struct {
		name      string
		selection LightingSelection
		sleep     float64
		response  int
		want      []byte
	}{
		{"baseline", LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIFactoryReset}, 0.5, 4, []byte{0x05, 0x0f, 0x01, 0x50, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xff}},
		{"debounce carry", LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIFactoryReset}, 0.5, 5, []byte{0x05, 0x0f, 0x01, 0x50, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x05, 0x02, 0x00}},
		{"sleep carry", LightingSelection{Mode: LightingStaticDPI, TemplateID: LightingTemplateStaticDPIFactoryReset}, 1, 4, []byte{0x05, 0x0f, 0x01, 0x50, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x02, 0x04, 0x02, 0x00}},
		{"green before color boundary", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingGreen}, 0.5, 4, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xcf}},
		{"yellow after color boundary", LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFFFF00}, 0.5, 4, []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0xff, 0xff, 0x01, 0x04, 0x02, 0xce}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := LightingSettings{LightingSelection: tt.selection, NormalSleepMinutes: tt.sleep, ResponseTimeMs: tt.response}
			got, err := EncodeLightingSettingsReport(settings)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("report = % x; want % x", got, tt.want)
			}
		})
	}
}
