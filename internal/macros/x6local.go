package macros

import "fmt"

// MapX6LocalEvents translates only capture-backed identity candidates offline.
// Opaque local acceptance is broader: unknown identities remain storable, but
// cannot be mapped here. Exact identities are required; no trimming or HID guesses.
// Delays remain source-oriented (after each event), including terminal timing.
// EncodeX6TimedBlock separately validates encoding limits and forces terminal 10.
// This function neither admits assignment nor guarantees physical playback.
// Every error returns nil output and leaves input unchanged.
func MapX6LocalEvents(events []Event) ([]X6TimedEvent, error) {
	if err := validate("offline mapping", events); err != nil {
		return nil, err
	}
	mapped := make([]X6TimedEvent, 0, len(events))
	for i, event := range events {
		var code byte
		if event.Type == Keyboard {
			switch event.KeyCode {
			case "KeyA":
				code = 0x04
			case "ControlLeft":
				code = 0xe0
			case "ShiftLeft":
				code = 0xe1
			default:
				return nil, fmt.Errorf("event %d: unsupported offline key identity %q", i, event.KeyCode)
			}
		} else {
			// Same evidenced f1–f5 ordering as the existing complete-click codec.
			for j, button := range x6SequenceButtons {
				if event.Type == button {
					code = byte(0xf1 + j)
					break
				}
			}
		}
		mapped = append(mapped, X6TimedEvent{Code: code, Action: event.Action, DelayMS: event.DelayMS})
	}
	return mapped, nil
}
