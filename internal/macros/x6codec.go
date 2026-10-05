package macros

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

// X6Click selects only the captured single-button press/release template.
// It is not a local Macro: no names, IDs, event lists or timing are encoded.
// Repeat accepts the decided UI range 1–255, not a proven playback range.
// Only left repeats 1/2/3/5 and right repeat 1 have captured evidence.
type X6Click struct {
	Button EventType
	Repeat int
}

// EncodeX6Block produces the 128-byte logical block, without report headers.
// All non-selected bytes are fixed to the captured template. Other repeats
// and right repeats above 1 extrapolate the observed one-byte field.
func EncodeX6Block(click X6Click) ([]byte, error) {
	if click.Repeat < 1 || click.Repeat > 255 {
		return nil, fmt.Errorf("X6 repeat must be in UI range 1–255: %d", click.Repeat)
	}
	var code byte
	switch click.Button {
	case MouseLeft:
		code = 0xf1
	case MouseRight:
		code = 0xf2
	default:
		return nil, fmt.Errorf("unsupported X6 button %q", click.Button)
	}
	b := make([]byte, 128)
	b[4] = byte(click.Repeat)
	b[25], b[26], b[27], b[28], b[29] = 2, 1, code, 0x81, code
	binary.BigEndian.PutUint16(b[126:], x6Checksum(b))
	return b, nil
}

// DecodeX6Block rejects any layout outside the exact supported template,
// even if its additive checksum is valid. It does not infer timing semantics.
func DecodeX6Block(b []byte) (X6Click, error) {
	if len(b) != 128 {
		return X6Click{}, fmt.Errorf("X6 block length must be 128: %d", len(b))
	}
	if binary.BigEndian.Uint16(b[126:]) != x6Checksum(b) {
		return X6Click{}, fmt.Errorf("invalid X6 checksum")
	}
	click := X6Click{Repeat: int(b[4])}
	switch b[27] {
	case 0xf1:
		click.Button = MouseLeft
	case 0xf2:
		click.Button = MouseRight
	default:
		return X6Click{}, fmt.Errorf("unsupported X6 event code %02x", b[27])
	}
	expected, err := EncodeX6Block(click)
	if err != nil {
		return X6Click{}, err
	}
	if !bytes.Equal(b, expected) {
		return X6Click{}, fmt.Errorf("unsupported X6 block layout")
	}
	return click, nil
}

// x6Checksum is only used for 128-byte blocks. Supported templates cannot
// overflow uint16; this is not an overflow rule for arbitrary future layouts.
func x6Checksum(b []byte) uint16 {
	var sum uint16
	for _, v := range b[:126] {
		sum += uint16(v)
	}
	return sum
}

var x6Headers = [3][4]byte{{9, 0x40, 5, 0}, {9, 0x40, 5, 1}, {9, 0x0c, 5, 2}}
var x6ChunkSizes = [3]int{60, 60, 8}

// EncodeX6Reports returns three independent 64-byte report09 buffers, ordered
// as captured, with block ID 05 and zero final padding. No transport is invoked.
func EncodeX6Reports(click X6Click) ([][]byte, error) {
	return EncodeX6Upload(X6Upload{Destination: 5, Click: click})
}

// X6Upload carries the button-group destination independently of macro actions.
// It does not imply slot allocation or independent simultaneous assignments.
type X6Upload struct {
	Destination byte
	Click       X6Click
}

// EncodeX6Upload uses the same strict template with a checked destination.
func EncodeX6Upload(upload X6Upload) ([][]byte, error) {
	if err := x6.ValidateMacroDestination(upload.Destination); err != nil {
		return nil, err
	}
	b, err := EncodeX6Block(upload.Click)
	if err != nil {
		return nil, err
	}
	reports := make([][]byte, 3)
	offset := 0
	for i, size := range x6ChunkSizes {
		reports[i] = make([]byte, 64)
		copy(reports[i], x6Headers[i][:])
		reports[i][2] = upload.Destination
		copy(reports[i][4:4+size], b[offset:offset+size])
		offset += size
	}
	return reports, nil
}

// DecodeX6Reports requires exactly the captured framing, including sequence,
// lengths and zero padding; it then applies strict logical-block validation.
func DecodeX6Reports(reports [][]byte) (X6Click, error) {
	upload, err := DecodeX6Upload(reports)
	if err != nil {
		return X6Click{}, err
	}
	if upload.Destination != 5 {
		return X6Click{}, fmt.Errorf("legacy X6 reports require destination 05")
	}
	return upload.Click, nil
}

// DecodeX6Upload preserves the checked destination and rejects mixed IDs.
// Framing, padding and logical-block admission remain identical to the legacy API.
func DecodeX6Upload(reports [][]byte) (X6Upload, error) {
	if len(reports) != 3 {
		return X6Upload{}, fmt.Errorf("X6 requires exactly three reports")
	}
	if len(reports[0]) != 64 {
		return X6Upload{}, fmt.Errorf("X6 report 0 length must be 64")
	}
	destination := reports[0][2]
	if err := x6.ValidateMacroDestination(destination); err != nil {
		return X6Upload{}, err
	}
	b := make([]byte, 0, 128)
	for i, size := range x6ChunkSizes {
		r := reports[i]
		if len(r) != 64 {
			return X6Upload{}, fmt.Errorf("X6 report %d length must be 64", i)
		}
		header := x6Headers[i]
		header[2] = destination
		if !bytes.Equal(r[:4], header[:]) {
			return X6Upload{}, fmt.Errorf("unsupported X6 report %d header", i)
		}
		for _, v := range r[4+size:] {
			if v != 0 {
				return X6Upload{}, fmt.Errorf("nonzero X6 report %d padding", i)
			}
		}
		b = append(b, r[4:4+size]...)
	}
	click, err := DecodeX6Block(b)
	if err != nil {
		return X6Upload{}, err
	}
	return X6Upload{Destination: destination, Click: click}, nil
}
