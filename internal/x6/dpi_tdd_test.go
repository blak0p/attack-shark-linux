package x6

import "testing"

func TestEncodeDPIReportProducesCompleteValidatedReport(t *testing.T) {
	config := DPIConfig{
		DPI:          [8]int{800, 1600, 3200, 6400, 8000, 12000, 16000, 26000},
		StageMask:    0xFF,
		LiftDistance: 1,
		ActiveStage:  4,
	}

	report, err := EncodeDPIReport(config)
	if err != nil {
		t.Fatalf("EncodeDPIReport() error = %v", err)
	}
	if len(report) != 52 {
		t.Fatalf("report length = %d, want 52", len(report))
	}
	if report[0] != 0x04 || report[1] != 0x38 || report[24] != 4 {
		t.Fatalf("report header/active stage = % x, want complete 0x04 report with active stage 4", report[:25])
	}
}

func TestEncodeDPIReportRejectsIncompleteOrUnsupportedStage(t *testing.T) {
	config := DPIConfig{DPI: [8]int{800, 1600, 3200, 6400, 8000, 12000, 16000, 26000}, StageMask: 0xFF, LiftDistance: 1, ActiveStage: 9}

	if _, err := EncodeDPIReport(config); err == nil {
		t.Fatal("EncodeDPIReport() error = nil, want invalid active stage rejection")
	}
}
