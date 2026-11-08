package x6

import (
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

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
	if len(report) != 56 {
		t.Fatalf("report length = %d, want 56", len(report))
	}
	if report[0] != 0x04 || report[1] != 0x38 || report[24] != 4 {
		t.Fatalf("report header/active stage = % x, want complete 0x04 report with active stage 4", report[:25])
	}
}

func TestNewDPIOperationAdaptsValidatedReportsAndACKs(t *testing.T) {
	operation := NewDPIOperation()
	if err := operation.Validate(DefaultDPIConfig()); err != nil {
		t.Fatalf("Validate(DefaultDPIConfig()) error = %v", err)
	}
	if err := operation.Validate("not a DPI config"); err == nil {
		t.Fatal("Validate() error = nil for an invalid value type")
	}
	report, err := operation.Encode(DefaultDPIConfig())
	if err != nil || len(report) != 56 {
		t.Fatalf("Encode(DefaultDPIConfig()) = % x, %v; want 56-byte report", report, err)
	}
	if !operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesACK() rejected the DPI acknowledgement")
	}
	if operation.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) {
		t.Fatal("MatchesACK() accepted another operation acknowledgement")
	}
	var _ mouse.CommandOperation = operation
}

func TestEncodeDPIReportRejectsIncompleteOrUnsupportedStage(t *testing.T) {
	config := DPIConfig{DPI: [8]int{800, 1600, 3200, 6400, 8000, 12000, 16000, 26000}, StageMask: 0xFF, LiftDistance: 1, ActiveStage: 9}

	if _, err := EncodeDPIReport(config); err == nil {
		t.Fatal("EncodeDPIReport() error = nil, want invalid active stage rejection")
	}
}
