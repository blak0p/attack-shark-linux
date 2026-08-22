package x6

import (
	"bytes"
	"testing"
)

func TestPollingRateEncodesExactReports(t *testing.T) {
	tests := []struct {
		name string
		rate PollingRate
		want []byte
	}{
		{"125 Hz", PollingRate125, []byte{0x06, 0x09, 0x01, 0x08, 0xf7, 0x00, 0x00, 0x00, 0x00}},
		{"250 Hz", PollingRate250, []byte{0x06, 0x09, 0x01, 0x04, 0xfb, 0x00, 0x00, 0x00, 0x00}},
		{"500 Hz", PollingRate500, []byte{0x06, 0x09, 0x01, 0x02, 0xfd, 0x00, 0x00, 0x00, 0x00}},
		{"1000 Hz", PollingRate1000, []byte{0x06, 0x09, 0x01, 0x01, 0xfe, 0x00, 0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodePollingReport(tt.rate)
			if err != nil {
				t.Fatalf("EncodePollingReport(%d) error = %v", tt.rate, err)
			}
			if len(got) != PollingReportLength || !bytes.Equal(got, tt.want) {
				t.Fatalf("EncodePollingReport(%d) = % x; want % x", tt.rate, got, tt.want)
			}
		})
	}
}

func TestPollingRateRejectsUnsupportedValues(t *testing.T) {
	for _, rate := range []PollingRate{0, 126, 1001} {
		if err := ValidatePollingRate(rate); err == nil {
			t.Fatalf("ValidatePollingRate(%d) error = nil; want rejection", rate)
		}
		if report, err := EncodePollingReport(rate); err == nil || report != nil {
			t.Fatalf("EncodePollingReport(%d) = % x, %v; want no report and an error", rate, report, err)
		}
	}
}

func TestPollingACKIsExactAndIndependentFromDPI(t *testing.T) {
	if !MatchesPollingACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) {
		t.Fatal("MatchesPollingACK() rejected the polling acknowledgement")
	}
	for _, report := range [][]byte{
		{0x03, 0x10, 0x50, 0x00, 0x04},
		{0x03, 0x10, 0x50, 0x00, 0x06, 0x00},
		{0x03, 0x10, 0x40, 0x00, 0x06},
	} {
		if MatchesPollingACK(report) {
			t.Fatalf("MatchesPollingACK(% x) = true; want false", report)
		}
	}
}
