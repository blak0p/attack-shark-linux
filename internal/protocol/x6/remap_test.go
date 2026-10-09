package x6

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestMacroAssignmentDestinations(t *testing.T) {
	for i, group := range []byte{1, 2, 3, 7, 8, 5, 6} {
		button := uint8(i + 1)
		got, err := MacroDestinationForButton(button)
		if err != nil || got != group {
			t.Fatalf("button %d: %d %v", button, got, err)
		}
		config := DefaultRemapConfig()
		for j := range config.Buttons {
			config.Buttons[j].Action = RemapFire
		}
		before, _ := EncodeRemapReport(config)
		report, err := EncodeMacroAssignmentReport(MacroAssignment{Config: config, Button: button})
		if err != nil {
			t.Fatal(err)
		}
		offset := 3 + int(group-1)*3
		if !bytes.Equal(report[offset:offset+3], []byte{0x12, 0, group}) {
			t.Fatalf("group: %x", report)
		}
		for j := 0; j < 57; j++ {
			if (j < offset || j >= offset+3) && report[j] != before[j] {
				t.Fatalf("unrelated byte %d changed", j)
			}
		}
		sum := 0
		for _, b := range report[3:57] {
			sum += int(b)
		}
		if int(report[57])*256+int(report[58]) != sum {
			t.Fatal("checksum")
		}
		after, _ := EncodeRemapReport(config)
		if !bytes.Equal(before, after) {
			t.Fatal("config mutated")
		}
	}
	for _, button := range []uint8{0, 8, 255} {
		if _, err := MacroDestinationForButton(button); err == nil {
			t.Fatal("invalid button")
		}
		if _, err := EncodeMacroAssignmentReport(MacroAssignment{DefaultRemapConfig(), button}); err == nil {
			t.Fatal("invalid assignment")
		}
	}
	for _, destination := range []byte{0, 4, 9, 255} {
		if ValidateMacroDestination(destination) == nil {
			t.Fatal("invalid destination")
		}
	}
	invalid := DefaultRemapConfig()
	invalid.Buttons[0].Action = "macro"
	if _, err := EncodeMacroAssignmentReport(MacroAssignment{invalid, 6}); err == nil {
		t.Fatal("invalid config")
	}
}

func TestMacroAssignmentCaptured05And06(t *testing.T) {
	for _, tt := range []struct {
		button uint8
		hex    string
	}{
		{6, "083b010200000300000400000d00001200050f00000600000500003c00000100000100000100000100000100000100000100000a0000090000009d"},
		{7, "083b010200000300000400000d00000e00001200060600000500003c00000100000100000100000100000100000100000100000a0000090000009d"},
	} {
		want, err := hex.DecodeString(tt.hex)
		if err != nil {
			t.Fatal(err)
		}
		got, err := EncodeMacroAssignmentReport(MacroAssignment{DefaultRemapConfig(), tt.button})
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("button %d: %x %v", tt.button, got, err)
		}
	}
}

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
	for _, action := range []RemapAction{"shortcut", "browser_favorites", "macro", "unknown"} {
		invalid := DefaultRemapConfig()
		invalid.Buttons[0].Action = action
		if _, err := EncodeRemapReport(invalid); err == nil {
			t.Fatalf("EncodeRemapReport() accepted excluded action %q", action)
		}
	}
	if !MatchesRemapACK([]byte{0x03, 0x10, 0x50, 0x00, 0x08}) || MatchesRemapACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesRemapACK() did not strictly match report 0x08")
	}
}

func TestRemapMultimediaActionsUseExactIDsAndProtectButtonOne(t *testing.T) {
	multimedia := []struct {
		action RemapAction
		id     byte
	}{
		{RemapMediaPlayer, 0x15}, {RemapPlayPause, 0x18}, {RemapStop, 0x19}, {RemapPreviousTrack, 0x16},
		{RemapNextTrack, 0x17}, {RemapVolumeUp, 0x1b}, {RemapVolumeDown, 0x1c}, {RemapMute, 0x1a},
	}

	for _, tt := range multimedia {
		t.Run(string(tt.action), func(t *testing.T) {
			for button := 2; button <= 7; button++ {
				eligible := DefaultRemapConfig()
				eligible.Buttons[button-1].Action = tt.action
				report, err := EncodeRemapReport(eligible)
				if err != nil {
					t.Fatalf("EncodeRemapReport(%q) error = %v", tt.action, err)
				}
				offset := 3 + (remapGroupByButton[button-1]-1)*3
				if got := report[offset]; got != tt.id {
					t.Fatalf("Button %d action byte = 0x%02x, want 0x%02x", button, got, tt.id)
				}
			}

			protected := DefaultRemapConfig()
			protected.Buttons[0].Action = tt.action
			if _, err := EncodeRemapReport(protected); err == nil {
				t.Fatalf("EncodeRemapReport() accepted Button 1 multimedia action %q", tt.action)
			}
		})
	}
}

func TestMouseControlsUseExactIDsAndRespectPhysicalButtonPolicy(t *testing.T) {
	mouseControls := []struct {
		action RemapAction
		id     byte
	}{
		{RemapScrollUp, 0x09}, {RemapScrollDown, 0x0a}, {RemapDPICycle, 0x0d}, {RemapDPIPlus, 0x0e}, {RemapDPIMinus, 0x0f},
	}
	for _, tt := range mouseControls {
		t.Run(string(tt.action), func(t *testing.T) {
			for button := 2; button <= 7; button++ {
				config := DefaultRemapConfig()
				config.Buttons[button-1].Action = tt.action
				report, err := EncodeRemapReport(config)
				if err != nil {
					t.Fatalf("EncodeRemapReport(Button %d, %q) error = %v", button, tt.action, err)
				}
				offset := 3 + (remapGroupByButton[button-1]-1)*3
				if report[offset] != tt.id {
					t.Fatalf("Button %d action byte = 0x%02x, want 0x%02x", button, report[offset], tt.id)
				}
			}
			blocked := DefaultRemapConfig()
			blocked.Buttons[0].Action = tt.action
			if err := ValidateRemapConfig(blocked); err == nil {
				t.Fatalf("ValidateRemapConfig() accepted Button 1 %q", tt.action)
			}
		})
	}
}

func TestBrowserActionsEncodeExactIDsAndZeroParameters(t *testing.T) {
	for _, tt := range []struct {
		action RemapAction
		id     byte
	}{
		{RemapBrowserCalculator, 0x1d}, {RemapBrowserEmail, 0x1e}, {RemapBrowserForward, 0x20},
		{RemapBrowserBackward, 0x21}, {RemapBrowserStop, 0x22}, {RemapBrowserMyComputer, 0x23},
		{RemapBrowserRefresh, 0x24}, {RemapBrowserHome, 0x25}, {RemapBrowserSearch, 0x26},
	} {
		t.Run(string(tt.action), func(t *testing.T) {
			config := DefaultRemapConfig()
			config.Buttons[0].Action = tt.action
			report, err := EncodeRemapReport(config)
			if err != nil {
				t.Fatal(err)
			}
			if len(report) != 59 || report[0] != 0x08 || report[1] != 0x3b || report[2] != 0x01 {
				t.Fatalf("invalid report shape: %x", report)
			}
			if report[3] != tt.id || report[4] != 0 || report[5] != 0 {
				t.Fatalf("action group = %x, want %02x0000", report[3:6], tt.id)
			}
			checksum := 0
			for _, value := range report[3:57] {
				checksum += int(value)
			}
			if report[57] != byte(checksum>>8) || report[58] != byte(checksum) {
				t.Fatalf("checksum = %x, want %04x", report[57:59], checksum)
			}
		})
	}
}

func TestBrowserActionOnButtonSevenPreservesHiddenGroups(t *testing.T) {
	config := DefaultRemapConfig()
	config.Buttons[6].Action = RemapBrowserHome
	report, err := EncodeRemapReport(config)
	if err != nil {
		t.Fatal(err)
	}
	if report[18] != 0x25 || report[19] != 0 || report[20] != 0 {
		t.Fatalf("Button 7 group = %x, want 250000", report[18:21])
	}
	if report[27] != remapBaseline[27] || report[12] != remapBaseline[12] {
		t.Fatal("Browser action changed hidden groups")
	}
	if MatchesRemapACK([]byte{0x03, 0x10, 0x50, 0x00, 0x1f}) {
		t.Fatal("accepted wrong ACK")
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

func TestEncodeMultiMacroAssignmentReport(t *testing.T) {
	// 1. Single button assignment matches EncodeMacroAssignmentReport
	for i := 1; i <= 7; i++ {
		button := uint8(i)
		config := DefaultRemapConfig()
		want, err := EncodeMacroAssignmentReport(MacroAssignment{Config: config, Button: button})
		if err != nil {
			t.Fatalf("EncodeMacroAssignmentReport(button %d): %v", button, err)
		}
		got, err := EncodeMultiMacroAssignmentReport(config, []uint8{button})
		if err != nil {
			t.Fatalf("EncodeMultiMacroAssignmentReport(button %d): %v", button, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("button %d mismatch: got %x, want %x", button, got, want)
		}
	}

	// 2. Empty or nil buttons slice preserves plain remap report
	plainConfig := DefaultRemapConfig()
	plainWant, err := EncodeRemapReport(plainConfig)
	if err != nil {
		t.Fatal(err)
	}
	plainGot, err := EncodeMultiMacroAssignmentReport(plainConfig, nil)
	if err != nil || !bytes.Equal(plainGot, plainWant) {
		t.Fatalf("nil buttons: got %x, want %x (err: %v)", plainGot, plainWant, err)
	}
	plainGotEmpty, err := EncodeMultiMacroAssignmentReport(plainConfig, []uint8{})
	if err != nil || !bytes.Equal(plainGotEmpty, plainWant) {
		t.Fatalf("empty buttons: got %x, want %x (err: %v)", plainGotEmpty, plainWant, err)
	}

	// 3. Frame 11647 from two-macros.pcapng regression:
	// Buttons 6 (dest 05) and 7 (dest 06) active simultaneously with default remap config.
	const frame11647Hex = "083b010200000300000400000d00001200051200060600000500003c00000100000100000100000100000100000100000100000a000009000000a6"
	want11647, err := hex.DecodeString(frame11647Hex)
	if err != nil {
		t.Fatal(err)
	}
	got11647, err := EncodeMultiMacroAssignmentReport(DefaultRemapConfig(), []uint8{6, 7})
	if err != nil {
		t.Fatalf("EncodeMultiMacroAssignmentReport(6, 7): %v", err)
	}
	if !bytes.Equal(got11647, want11647) {
		t.Fatalf("Frame 11647 mismatch:\ngot:  %x\nwant: %x", got11647, want11647)
	}

	// Order independence of buttons in multi-macro assignment
	got11647Reversed, err := EncodeMultiMacroAssignmentReport(DefaultRemapConfig(), []uint8{7, 6})
	if err != nil {
		t.Fatalf("EncodeMultiMacroAssignmentReport(7, 6): %v", err)
	}
	if !bytes.Equal(got11647Reversed, want11647) {
		t.Fatalf("Frame 11647 reversed buttons mismatch:\ngot:  %x\nwant: %x", got11647Reversed, want11647)
	}

	// Verify MultiMacroAssignment struct
	assignment := MultiMacroAssignment{
		Config:  DefaultRemapConfig(),
		Buttons: []uint8{6, 7},
	}
	if len(assignment.Buttons) != 2 || assignment.Buttons[0] != 6 || assignment.Buttons[1] != 7 {
		t.Fatalf("MultiMacroAssignment struct fields mismatch: %+v", assignment)
	}

	// 4. Invalid buttons rejected
	for _, badButtons := range [][]uint8{
		{0},
		{8},
		{255},
		{6, 0},
		{8, 7},
	} {
		if _, err := EncodeMultiMacroAssignmentReport(DefaultRemapConfig(), badButtons); err == nil {
			t.Fatalf("expected error for bad buttons %v, got nil", badButtons)
		}
	}

	// 5. Invalid remap config rejected
	invalidConfig := DefaultRemapConfig()
	invalidConfig.Buttons[0].Action = "macro"
	if _, err := EncodeMultiMacroAssignmentReport(invalidConfig, []uint8{6, 7}); err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}
