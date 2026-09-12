package x6

import (
	"bytes"
	"testing"
)

func TestLightingSettingsPreservesSelectionAndEncodesSleepAndResponse(t *testing.T) {
	settings := LightingSettings{LightingSelection: LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00}, NormalSleepMinutes: 30.5, ResponseTimeMs: 12}
	got, err := EncodeLightingSettingsReport(settings)
	if err != nil { t.Fatal(err) }
	want := []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x3d, 0x06, 0x02, 0x8c}
	if !bytes.Equal(got, want) { t.Fatalf("report = % x; want % x", got, want) }
}

func TestLightingSettingsRejectsUnsupportedResponse(t *testing.T) {
	settings := DefaultLightingSettings(); settings.ResponseTimeMs = 3
	if _, err := EncodeLightingSettingsReport(settings); err == nil { t.Fatal("accepted unsupported response time") }
}
