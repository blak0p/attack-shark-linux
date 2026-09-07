package x6

import "testing"

func TestRemapEncodesOnlyMappedPhysicalButtons(t *testing.T) {
	config := DefaultRemapConfig()
	config.Buttons[3].Action = RemapDoubleClick
	config.Buttons[5].Action = RemapFire
	report, err := EncodeRemapReport(config)
	if err != nil {
		t.Fatalf("EncodeRemapReport() error = %v", err)
	}
	if len(report) != RemapReportLength {
		t.Fatalf("report length = %d, want %d", len(report), RemapReportLength)
	}
	if report[3+(7-1)*3] != 0x07 || report[3+(5-1)*3] != 0x08 {
		t.Fatalf("mapped actions = %x %x; want double click and fire", report[21], report[15])
	}
	if report[3+(4-1)*3] != 0x0d || report[3+(9-1)*3] != 0x3c {
		t.Fatalf("hidden groups changed: g4=%x g9=%x", report[12], report[27])
	}
	checksum := 0
	for _, value := range report[3:57] {
		checksum += int(value)
	}
	if report[57] != byte(checksum>>8) || report[58] != byte(checksum) {
		t.Fatalf("checksum = %x %x, want %04x", report[57], report[58], checksum)
	}
}

func TestRemapAcceptsOnlyClosedActionsAndExactACK(t *testing.T) {
	for _, action := range []RemapAction{RemapOff, RemapLeft, RemapRight, RemapMiddle, RemapForward, RemapBackward, RemapDoubleClick, RemapFire} {
		config := DefaultRemapConfig()
		config.Buttons[0].Action = action
		if _, err := EncodeRemapReport(config); err != nil {
			t.Fatalf("EncodeRemapReport(%q) error = %v", action, err)
		}
	}
	invalid := DefaultRemapConfig()
	invalid.Buttons[0].Action = RemapAction("dpi_plus")
	if _, err := EncodeRemapReport(invalid); err == nil {
		t.Fatal("EncodeRemapReport() accepted excluded action")
	}
	if !MatchesRemapACK([]byte{0x03, 0x10, 0x50, 0x00, 0x08}) || MatchesRemapACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesRemapACK() did not strictly match report 0x08")
	}
}

func TestRemapDefaultsPreserveDPIMarkersAndReturnCopies(t *testing.T) {
	first := DefaultRemapConfig()
	second := DefaultRemapConfig()
	if first.Buttons[0].Action != RemapLeft || first.Buttons[4].Action != RemapBackward || first.Buttons[5].Action != "" || first.Buttons[5].PreservedDefault != "DPI+" || first.Buttons[6].PreservedDefault != "DPI-" {
		t.Fatalf("factory defaults = %#v", first)
	}
	first.Buttons[0].Action = RemapFire
	if second.Buttons[0].Action != RemapLeft {
		t.Fatal("DefaultRemapConfig() returned aliased button storage")
	}
}
