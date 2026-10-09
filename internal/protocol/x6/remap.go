package x6

import "fmt"

const RemapReportLength = 59

type RemapAction string

const (
	RemapOff               RemapAction = "off"
	RemapLeft              RemapAction = "left"
	RemapRight             RemapAction = "right"
	RemapMiddle            RemapAction = "middle"
	RemapBackward          RemapAction = "backward"
	RemapForward           RemapAction = "forward"
	RemapDoubleClick       RemapAction = "double_click"
	RemapFire              RemapAction = "fire"
	RemapMediaPlayer       RemapAction = "media_player"
	RemapPlayPause         RemapAction = "play_pause"
	RemapStop              RemapAction = "stop"
	RemapPreviousTrack     RemapAction = "previous_track"
	RemapNextTrack         RemapAction = "next_track"
	RemapVolumeUp          RemapAction = "volume_up"
	RemapVolumeDown        RemapAction = "volume_down"
	RemapMute              RemapAction = "mute"
	RemapScrollUp          RemapAction = "scroll_up"
	RemapScrollDown        RemapAction = "scroll_down"
	RemapDPICycle          RemapAction = "dpi_cycle"
	RemapDPIPlus           RemapAction = "dpi_plus"
	RemapDPIMinus          RemapAction = "dpi_minus"
	RemapBrowserCalculator RemapAction = "browser_calculator"
	RemapBrowserEmail      RemapAction = "browser_email"
	RemapBrowserForward    RemapAction = "browser_forward"
	RemapBrowserBackward   RemapAction = "browser_backward"
	RemapBrowserStop       RemapAction = "browser_stop"
	RemapBrowserMyComputer RemapAction = "browser_my_computer"
	RemapBrowserRefresh    RemapAction = "browser_refresh"
	RemapBrowserHome       RemapAction = "browser_home"
	RemapBrowserSearch     RemapAction = "browser_search"
)

type RemapButton struct {
	Button           uint8
	Action           RemapAction
	PreservedDefault string
}

type RemapConfig struct{ Buttons []RemapButton }

var remapBaseline = [RemapReportLength]byte{
	0x08, 0x3b, 0x01, 0x02, 0x00, 0x00, 0x03, 0x00, 0x00, 0x04, 0x00, 0x00,
	0x0d, 0x00, 0x00, 0x0e, 0x00, 0x00, 0x0f, 0x00, 0x00, 0x06, 0x00, 0x00,
	0x05, 0x00, 0x00, 0x3c, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x94,
}

var remapGroupByButton = [7]byte{1, 2, 3, 7, 8, 5, 6}

// MacroAssignment overlays one macro action on a valid pending remap config.
// It is separate from the closed generic remap action catalog.
type MacroAssignment struct {
	Config RemapConfig
	Button uint8
}

// MultiMacroAssignment overlays multiple macro actions on a valid pending remap config.
type MultiMacroAssignment struct {
	Config  RemapConfig
	Buttons []uint8
}

// MacroDestinationForButton maps logical buttons to report08 groups/report09 IDs.
// Groups 05/06 are capture-backed; other mapped groups are authorized extrapolation.
func MacroDestinationForButton(button uint8) (byte, error) {
	if button < 1 || int(button) > len(remapGroupByButton) {
		return 0, fmt.Errorf("unsupported macro button %d", button)
	}
	return remapGroupByButton[button-1], nil
}

// ValidateMacroDestination accepts only existing mapped button groups, not group4.
func ValidateMacroDestination(destination byte) error {
	for _, group := range remapGroupByButton {
		if destination == group {
			return nil
		}
	}
	return fmt.Errorf("unsupported macro destination %d", destination)
}

func EncodeMacroAssignmentReport(assignment MacroAssignment) ([]byte, error) {
	return EncodeMultiMacroAssignmentReport(assignment.Config, []uint8{assignment.Button})
}

// EncodeMultiMacroAssignmentReport overlays multiple macro actions on a valid pending remap config.
// For each button, it writes 0x12, 0, destination at 3 + int(destination-1)*3 and updates the checksum.
func EncodeMultiMacroAssignmentReport(config RemapConfig, buttons []uint8) ([]byte, error) {
	report, err := EncodeRemapReport(config)
	if err != nil {
		return nil, err
	}
	for _, button := range buttons {
		destination, err := MacroDestinationForButton(button)
		if err != nil {
			return nil, err
		}
		offset := 3 + int(destination-1)*3
		report[offset], report[offset+1], report[offset+2] = 0x12, 0, destination
	}
	setRemapChecksum(report)
	return report, nil
}

func DefaultRemapConfig() RemapConfig {
	return RemapConfig{Buttons: []RemapButton{
		{Button: 1, Action: RemapLeft}, {Button: 2, Action: RemapRight}, {Button: 3, Action: RemapMiddle},
		{Button: 4, Action: RemapForward}, {Button: 5, Action: RemapBackward},
		{Button: 6, PreservedDefault: "DPI+"}, {Button: 7, PreservedDefault: "DPI-"},
	}}
}

func ValidateRemapConfig(config RemapConfig) error {
	if len(config.Buttons) != len(remapGroupByButton) {
		return fmt.Errorf("remap configuration has %d buttons, want 7", len(config.Buttons))
	}
	for index, button := range config.Buttons {
		if button.Button != uint8(index+1) {
			return fmt.Errorf("remap button %d is out of order", button.Button)
		}
		if button.Action == "" {
			if index < 5 || button.PreservedDefault == "" {
				return fmt.Errorf("remap button %d has no allowed action", button.Button)
			}
			continue
		}
		if !isRemapAction(button.Action) {
			return fmt.Errorf("remap button %d has unsupported action %q", button.Button, button.Action)
		}
		if button.Button == 1 && (isMultimediaRemapAction(button.Action) || isMouseControlsRemapAction(button.Action)) {
			return fmt.Errorf("remap button 1 does not support action %q", button.Action)
		}
	}
	return nil
}

func EncodeRemapReport(config RemapConfig) ([]byte, error) {
	if err := ValidateRemapConfig(config); err != nil {
		return nil, err
	}
	report := append([]byte(nil), remapBaseline[:]...)
	for index, button := range config.Buttons {
		if button.Action != "" {
			report[3+(remapGroupByButton[index]-1)*3] = remapActionID(button.Action)
		}
	}
	setRemapChecksum(report)
	return report, nil
}

func setRemapChecksum(report []byte) {
	checksum := 0
	for _, value := range report[3:57] {
		checksum += int(value)
	}
	report[57], report[58] = byte(checksum>>8), byte(checksum)
}

func remapActionID(action RemapAction) byte {
	switch action {
	case RemapOff:
		return 0x01
	case RemapLeft:
		return 0x02
	case RemapRight:
		return 0x03
	case RemapMiddle:
		return 0x04
	case RemapBackward:
		return 0x05
	case RemapForward:
		return 0x06
	case RemapDoubleClick:
		return 0x07
	case RemapFire:
		return 0x08
	case RemapMediaPlayer:
		return 0x15
	case RemapPlayPause:
		return 0x18
	case RemapStop:
		return 0x19
	case RemapPreviousTrack:
		return 0x16
	case RemapNextTrack:
		return 0x17
	case RemapVolumeUp:
		return 0x1b
	case RemapVolumeDown:
		return 0x1c
	case RemapMute:
		return 0x1a
	case RemapScrollUp:
		return 0x09
	case RemapScrollDown:
		return 0x0a
	case RemapDPICycle:
		return 0x0d
	case RemapDPIPlus:
		return 0x0e
	case RemapDPIMinus:
		return 0x0f
	case RemapBrowserCalculator:
		return 0x1d
	case RemapBrowserEmail:
		return 0x1e
	case RemapBrowserForward:
		return 0x20
	case RemapBrowserBackward:
		return 0x21
	case RemapBrowserStop:
		return 0x22
	case RemapBrowserMyComputer:
		return 0x23
	case RemapBrowserRefresh:
		return 0x24
	case RemapBrowserHome:
		return 0x25
	case RemapBrowserSearch:
		return 0x26
	default:
		return 0
	}
}

func MatchesRemapACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == 0x08
}

func isRemapAction(action RemapAction) bool {
	switch action {
	case RemapOff, RemapLeft, RemapRight, RemapMiddle, RemapForward, RemapBackward, RemapDoubleClick, RemapFire,
		RemapMediaPlayer, RemapPlayPause, RemapStop, RemapPreviousTrack, RemapNextTrack, RemapVolumeUp, RemapVolumeDown, RemapMute,
		RemapScrollUp, RemapScrollDown, RemapDPICycle, RemapDPIPlus, RemapDPIMinus,
		RemapBrowserCalculator, RemapBrowserEmail, RemapBrowserForward, RemapBrowserBackward, RemapBrowserStop,
		RemapBrowserMyComputer, RemapBrowserRefresh, RemapBrowserHome, RemapBrowserSearch:
		return true
	default:
		return false
	}
}

func isMouseControlsRemapAction(action RemapAction) bool {
	switch action {
	case RemapScrollUp, RemapScrollDown, RemapDPICycle, RemapDPIPlus, RemapDPIMinus:
		return true
	default:
		return false
	}
}

func isMultimediaRemapAction(action RemapAction) bool {
	switch action {
	case RemapMediaPlayer, RemapPlayPause, RemapStop, RemapPreviousTrack, RemapNextTrack, RemapVolumeUp, RemapVolumeDown, RemapMute:
		return true
	default:
		return false
	}
}
