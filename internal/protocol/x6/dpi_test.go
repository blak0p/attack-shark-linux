package x6

import "testing"

func TestDPIReportContractIs52BytesAndAcceptsOnlyMatchingACK(t *testing.T) {
	if DPIReportLength != 52 {
		t.Fatalf("DPIReportLength = %d, want 52", DPIReportLength)
	}
	if !MatchesDPIACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesDPIACK() rejected documented acknowledgement")
	}
	if MatchesDPIACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) {
		t.Fatal("MatchesDPIACK() accepted acknowledgement for a different report")
	}
}

func TestPureProtocolEncodesCompleteDPIAndDecodesOnlyBatteryReports(t *testing.T) {
	config := DefaultDPIConfig()
	config.DPI[0] = 1600
	report, err := EncodeDPIReport(config)
	if err != nil || len(report) != DPIReportLength || report[8] != 31 {
		t.Fatalf("EncodeDPIReport() = % x, %v; want a complete 52-byte report with 1600 DPI", report, err)
	}

	battery, available := DecodeBatteryStatus([]byte{0x03, 0x10, 0x40, 0x00, 8})
	if !available || battery != 80 {
		t.Fatalf("DecodeBatteryStatus() = %d, %t; want 80, true", battery, available)
	}
	if _, available := DecodeBatteryStatus([]byte{0x03, 0x10, 0x50, 0x00, 8}); available {
		t.Fatal("DecodeBatteryStatus() accepted a non-battery report")
	}
}
