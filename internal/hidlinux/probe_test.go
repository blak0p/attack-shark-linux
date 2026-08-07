package hidlinux

import "testing"

func TestProbeRequiresExplicitHardwareAuthorization(t *testing.T) {
	for _, tt := range []struct {
		name string
		flag bool
		gate string
		want bool
	}{
		{"missing flag", false, "1", false},
		{"missing environment gate", true, "", false},
		{"explicit authorization", true, "1", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProbeEnabled(tt.flag, tt.gate); got != tt.want {
				t.Fatalf("ProbeEnabled(%t, %q) = %t, want %t", tt.flag, tt.gate, got, tt.want)
			}
		})
	}
}

func TestProbeMatchesOnlyTheExpectedAcknowledgement(t *testing.T) {
	for _, tt := range []struct {
		name   string
		report []byte
		want   bool
	}{
		{"matching report", []byte{0x03, 0x10, 0x50, 0x00, 0x04}, true},
		{"heartbeat is not an acknowledgement", []byte{0x03, 0x10, 0x40, 0x01, 0x0a}, false},
		{"wrong report ID", []byte{0x03, 0x10, 0x50, 0x00, 0x05}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesProbeACK(tt.report, 0x04); got != tt.want {
				t.Fatalf("MatchesProbeACK(%x, 0x04) = %t, want %t", tt.report, got, tt.want)
			}
		})
	}
}
