package x6

import "fmt"

const RemapReportLength = 59

type RemapAction string

const (
	RemapOff         RemapAction = "off"
	RemapLeft        RemapAction = "left"
	RemapRight       RemapAction = "right"
	RemapMiddle      RemapAction = "middle"
	RemapBackward    RemapAction = "backward"
	RemapForward     RemapAction = "forward"
	RemapDoubleClick RemapAction = "double_click"
	RemapFire        RemapAction = "fire"
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
	checksum := 0
	for _, value := range report[3:57] {
		checksum += int(value)
	}
	report[57], report[58] = byte(checksum>>8), byte(checksum)
	return report, nil
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
	default:
		return 0
	}
}

func MatchesRemapACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == 0x08
}

func isRemapAction(action RemapAction) bool {
	switch action {
	case RemapOff, RemapLeft, RemapRight, RemapMiddle, RemapForward, RemapBackward, RemapDoubleClick, RemapFire:
		return true
	default:
		return false
	}
}
