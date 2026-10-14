package macros

import (
	"encoding/binary"
	"fmt"
)

// X6TimedEvent is a raw source event, not a local-library event or key mapping.
// DelayMS applies after this event. The writer uses 10 for the terminal event.
// Code is copied verbatim; accepting a byte does not establish device support.
type X6TimedEvent struct {
	Code    byte
	Action  Action
	DelayMS int64
}

// EncodeX6TimedBlock implements only the traced forward host arithmetic.
// It never admits an upload, infers inverse timing, or performs device I/O.
// Limits reject vendor truncation/overflow; they are not firmware limits.
// Input is unchanged and every rejection returns nil output.
func EncodeX6TimedBlock(events []X6TimedEvent, repeat int, mode uint8) ([]byte, error) {
	if len(events) < 1 || len(events) > 46 {
		return nil, fmt.Errorf("X6 requires 1–46 source events")
	}
	if repeat < 1 || repeat > 255 {
		return nil, fmt.Errorf("X6 repeat must be 1–255")
	}
	if mode > 2 {
		return nil, fmt.Errorf("unsupported X6 mode %d", mode)
	}
	// Validate even terminal source timing before replacing it with writer timing.
	for i, e := range events {
		if e.Action != Down && e.Action != Up {
			return nil, fmt.Errorf("event %d: unsupported action", i)
		}
		if e.DelayMS < 0 || e.DelayMS > 65535 {
			return nil, fmt.Errorf("event %d: delay outside unsigned16 range", i)
		}
		if e.DelayMS >= 1271 && e.DelayMS/200 > 255 {
			return nil, fmt.Errorf("event %d: extension quotient exceeds byte", i)
		}
	}
	pairs := make([]byte, 0, 100)
	extensions := 0
	for i, e := range events {
		delay := e.DelayMS
		if i == len(events)-1 {
			delay = 10
		}
		var quotient int64
		if delay >= 1271 {
			quotient = delay / 200
			delay %= 200
			extensions++
			if extensions > 10 {
				return nil, fmt.Errorf("X6 requires at most 10 extensions")
			}
		}
		if delay == 0 {
			delay = 1
		}
		opcode := byte((delay+5)/10) | 1
		if e.Action == Up {
			opcode |= 0x80
		}
		pairs = append(pairs, opcode, e.Code)
		if quotient != 0 {
			pairs = append(pairs, byte(quotient), 3)
		}
		if len(pairs) > 100 {
			return nil, fmt.Errorf("X6 requires at most 50 output pairs")
		}
	}
	b := make([]byte, 128)
	b[0], b[4], b[25] = mode, byte(repeat), byte(len(pairs)/2)
	copy(b[26:126], pairs)
	binary.BigEndian.PutUint16(b[126:], x6Checksum(b))
	return b, nil
}
