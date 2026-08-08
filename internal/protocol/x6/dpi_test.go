package x6

import "testing"

func TestDecodeStatusReportHeartbeatCarriesBattery(t *testing.T) {
	report, ok := DecodeStatusReport([]byte{0x03, 0x10, 0x40, 0x01, 0x0a})
	if !ok {
		t.Fatal("DecodeStatusReport() ok = false; want heartbeat decoded")
	}
	if !report.BatteryAvailable || report.Battery != 100 {
		t.Fatalf("DecodeStatusReport() = %+v; want battery 100%%", report)
	}
	if report.StageAvailable {
		t.Fatalf("DecodeStatusReport() stage available on heartbeat; want false")
	}
}

func TestDecodeStatusReportDPIBottonCarriesStage(t *testing.T) {
	report, ok := DecodeStatusReport([]byte{0x03, 0x10, 0x10, 0x03, 0x00})
	if !ok {
		t.Fatal("DecodeStatusReport() ok = false; want DPI button event decoded")
	}
	if !report.StageAvailable || report.ActiveStage != 3 {
		t.Fatalf("DecodeStatusReport() = %+v; want active stage 3", report)
	}
	if report.BatteryAvailable {
		t.Fatalf("DecodeStatusReport() battery available on DPI event; want false")
	}
}

func TestDecodeStatusReportRejectsAckAndUnknownReports(t *testing.T) {
	if _, ok := DecodeStatusReport([]byte{0x03, 0x10, 0x50, 0x00, 0x04}); ok {
		t.Fatal("DecodeStatusReport() accepted the 0x50 configuration ACK")
	}
	if _, ok := DecodeStatusReport([]byte{0x03, 0x10, 0x70, 0x00, 0x00}); ok {
		t.Fatal("DecodeStatusReport() accepted an unknown status sub-report")
	}
	if _, ok := DecodeStatusReport([]byte{0x04, 0x38, 0x01}); ok {
		t.Fatal("DecodeStatusReport() accepted a non-status report")
	}
}

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
