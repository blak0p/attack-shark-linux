package x6

import "testing"

func TestDecodeValidatedInputReport(t *testing.T) {
	tests := []struct {
		name   string
		report []byte
		want   ReportStatus
	}{
		{
			name:   "short report leaves battery unavailable",
			report: []byte{0x03, 0x40, 0x00, 0x00},
			want:   ReportStatus{},
		},
		{
			name:   "wrong report ID leaves battery unavailable",
			report: []byte{0x04, 0x40, 0x00, 0x00, 0x05},
			want:   ReportStatus{},
		},
		{
			name:   "heartbeat reports an empty battery",
			report: []byte{0x03, 0x40, 0x00, 0x00, 0x00},
			want:   ReportStatus{BatteryPercent: 0, BatteryAvailable: true},
		},
		{
			name:   "heartbeat reports a full battery",
			report: []byte{0x03, 0x40, 0x00, 0x00, 0x0A},
			want:   ReportStatus{BatteryPercent: 100, BatteryAvailable: true},
		},
		{
			name:   "heartbeat above the accepted range leaves battery unavailable",
			report: []byte{0x03, 0x40, 0x00, 0x00, 0x0B},
			want:   ReportStatus{},
		},
		{
			name:   "ack does not supply a battery value",
			report: []byte{0x03, 0x50, 0x00, 0x00, 0x05},
			want:   ReportStatus{},
		},
		{
			name:   "unknown event does not supply a battery value",
			report: []byte{0x03, 0x99, 0x00, 0x00, 0x05},
			want:   ReportStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecodeValidatedInputReport(tt.report); got != tt.want {
				t.Fatalf("DecodeValidatedInputReport(%#v) = %#v, want %#v", tt.report, got, tt.want)
			}
		})
	}
}
