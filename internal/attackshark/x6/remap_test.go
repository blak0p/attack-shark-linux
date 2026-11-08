package x6

import "testing"

func TestRemapOperationValidatesTypedClosedConfiguration(t *testing.T) {
	op := NewRemapOperation()
	if err := op.Validate(DefaultRemapConfig()); err != nil {
		t.Fatalf("Validate(factory) error = %v", err)
	}
	if err := op.Validate("not a remap config"); err == nil {
		t.Fatal("Validate() accepted wrong type")
	}
	invalid := DefaultRemapConfig()
	invalid.Buttons[0].Action = RemapAction("macro")
	if err := op.Validate(invalid); err == nil {
		t.Fatal("Validate() accepted excluded action")
	}
}

func TestRemapOperationEncodesAndMatchesOnlyRemapACK(t *testing.T) {
	op := NewRemapOperation()
	report, err := op.Encode(DefaultRemapConfig())
	if err != nil || len(report) != RemapReportLength {
		t.Fatalf("Encode() = %x, %v", report, err)
	}
	if !op.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x08}) || op.MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x06}) {
		t.Fatal("MatchesACK() did not strictly match remap ACK")
	}
}
