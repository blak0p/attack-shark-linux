package macros

import (
	"fmt"
	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

// EncodeX6LocalUpload admits local source events before using captured framing.
// It intentionally provides no arbitrary preencoded-block entry point.
func EncodeX6LocalUpload(destination byte, events []Event, repeat int, mode uint8) ([][]byte, error) {
	if err := x6.ValidateMacroDestination(destination); err != nil {
		return nil, err
	}
	block, err := AdmitX6LocalBlock(events, repeat, mode)
	if err != nil {
		return nil, err
	}
	return encodeX6Framing(destination, block), nil
}

// AdmitX6LocalBlock validates a complete local take before forward encoding.
// Only the exact evidence-backed mapper identities are admitted. Success is
// host-side admission, not measured playback or an inverse timing guarantee.
// No event is dropped, reordered, or synthesized; failures return nil output.
func AdmitX6LocalBlock(events []Event, repeat int, mode uint8) ([]byte, error) {
	frozen := append([]Event(nil), events...)
	mapped, err := MapX6LocalEvents(frozen)
	if err != nil {
		return nil, err
	}
	held := make(map[byte]string)
	for i, event := range mapped {
		identity := string(frozen[i].Type) + ":" + frozen[i].KeyCode
		switch event.Action {
		case Down:
			if _, exists := held[event.Code]; exists {
				return nil, fmt.Errorf("event %d: repeated down", i)
			}
			held[event.Code] = identity
		case Up:
			if held[event.Code] != identity {
				return nil, fmt.Errorf("event %d: unmatched up", i)
			}
			delete(held, event.Code)
		default:
			return nil, fmt.Errorf("event %d: unsupported action", i)
		}
	}
	if len(held) != 0 {
		return nil, fmt.Errorf("incomplete X6 event sequence")
	}
	return EncodeX6TimedBlock(mapped, repeat, mode)
}
