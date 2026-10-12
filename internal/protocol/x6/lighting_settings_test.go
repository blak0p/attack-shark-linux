package x6

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLightingSettingsPreservesSelectionAndEncodesSleepAndResponse(t *testing.T) {
	for _, ms := range []int{2, 3, 8, 12, 25} {
		t.Run(fmt.Sprint(ms), func(t *testing.T) {
			settings := LightingSettings{LightingSelection: LightingSelection{Mode: LightingBreathing, TemplateID: LightingTemplateBreathingFF7F00}, NormalSleepMinutes: 30.5, ResponseTimeMs: ms}
			got, err := EncodeLightingSettingsReport(settings)
			if err != nil { t.Fatal(err) }
			want := []byte{0x05, 0x0f, 0x01, 0x20, 0x03, 0xa8, 0x00, 0x7f, 0xff, 0x3d, byte(ms), 0x02, byte(0x86 + ms)}
			if !bytes.Equal(got, want) { t.Fatalf("report = % x; want % x", got, want) }
		})
	}
}

func TestLightingSettingsRejectsUnsupportedResponse(t *testing.T) {
	for _, ms := range []int{-1, 1, 26, 32} {
		t.Run(fmt.Sprint(ms), func(t *testing.T) {
			settings := DefaultLightingSettings(); settings.ResponseTimeMs = ms
			if _, err := EncodeLightingSettingsReport(settings); err == nil { t.Fatal("accepted unsupported response time") }
		})
	}
}

func TestLightingSettingsFactoryResponseRemainsEightMilliseconds(t *testing.T) {
	if got := DefaultLightingSettings().ResponseTimeMs; got != 8 { t.Fatalf("factory response = %d; want 8", got) }
}
