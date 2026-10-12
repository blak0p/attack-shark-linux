package x6

import "testing"

func TestCapturedResetLightingSettingsReport(t *testing.T) {
	report, err := EncodeLightingSettingsReport(DocumentedResetLightingSettings())
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x05, 0x0f, 0x01, 0x50, 0x03, 0xa8, 0x00, 0xff, 0x00, 0x01, 0x04, 0x01, 0xff}
	if string(report) != string(want) {
		t.Fatalf("captured reset settings report = % x, want % x", report, want)
	}
}
