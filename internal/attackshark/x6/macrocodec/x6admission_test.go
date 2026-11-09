package macrocodec

import (
	"bytes"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"reflect"
	"testing"
)

func TestX6AdmissionUploadFraming(t *testing.T) {
	events := []macros.Event{{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 10}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up}}
	reports, err := EncodeX6LocalUpload(5, events, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 3 || !bytes.Equal(reports[0][:4], []byte{9, 0x40, 5, 0}) || !bytes.Equal(reports[1][:4], []byte{9, 0x40, 5, 1}) || !bytes.Equal(reports[2][:4], []byte{9, 0x0c, 5, 2}) || !bytes.Equal(reports[0][30:34], []byte{1, 4, 0x81, 4}) {
		t.Fatalf("unexpected captured framing: %x", reports)
	}
	for _, destination := range []byte{0, 4, 255} {
		if got, err := EncodeX6LocalUpload(destination, events, 1, 0); err == nil || got != nil {
			t.Fatal("invalid destination admitted")
		}
	}
	if got, err := EncodeX6LocalUpload(5, nil, 1, 0); err == nil || got != nil {
		t.Fatal("nil admitted")
	}
}

func TestX6AdmissionCapturedChord(t *testing.T) {
	events := []macros.Event{{Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up, DelayMS: 100}}
	block, err := AdmitX6LocalBlock(events, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(block[26:34], []byte{0x0b, 0xe1, 0x0b, 0x04, 0x8b, 0x04, 0x81, 0xe1}) || !bytes.Equal(block[126:], []byte{2, 0xf1}) {
		t.Fatalf("captured block mismatch: %x", block)
	}
}

func TestX6AdmissionRejectsUnsafeLocal(t *testing.T) {
	a := macros.Event{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down}
	up := a
	up.Action = macros.Up
	for _, tc := range []struct {
		name   string
		events []macros.Event
		repeat int
		mode   uint8
	}{
		{"incomplete", []macros.Event{a}, 1, 0},
		{"unmatched", []macros.Event{up, a}, 1, 0},
		{"repeated down", []macros.Event{a, a, up}, 1, 0},
		{"unsupported", []macros.Event{{Type: macros.Keyboard, KeyCode: "F13", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "F13", Action: macros.Up}}, 1, 0},
		{"padded", []macros.Event{{Type: macros.Keyboard, KeyCode: " KeyA", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: " KeyA", Action: macros.Up}}, 1, 0},
		{"mouse ambiguity", []macros.Event{{Type: macros.MouseLeft, KeyCode: "KeyA", Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}}, 1, 0},
		{"repeat", []macros.Event{a, up}, 0, 0},
		{"mode", []macros.Event{a, up}, 1, 3},
		{"overflow", []macros.Event{{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 51200}, up}, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block, err := AdmitX6LocalBlock(tc.events, tc.repeat, tc.mode)
			if err == nil || block != nil {
				t.Fatalf("unsafe input admitted: %x %v", block, err)
			}
		})
	}
}

func TestX6AdmissionCapacityAndOverlappingModifiers(t *testing.T) {
	clicks := func(count int) []macros.Event {
		var events []macros.Event
		for i := 0; i < count; i++ {
			events = append(events, macros.Event{Type: macros.MouseLeft, Action: macros.Down}, macros.Event{Type: macros.MouseLeft, Action: macros.Up})
		}
		return events
	}
	for _, tc := range []struct {
		name       string
		source     int
		extensions int
		valid      bool
		pairs      byte
	}{
		{"46 sources", 46, 0, true, 46}, {"48 sources", 48, 0, false, 0}, {"50 pairs", 46, 4, true, 50}, {"51 pairs", 46, 5, false, 0}, {"10 extensions", 12, 10, true, 22}, {"11 extensions", 14, 11, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := clicks(tc.source / 2)
			for i := 0; i < tc.extensions; i++ {
				events[i].DelayMS = 1271
			}
			before := append([]macros.Event(nil), events...)
			block, err := AdmitX6LocalBlock(events, 255, 2)
			if tc.valid {
				if err != nil || block[25] != tc.pairs {
					t.Fatal(err, block)
				}
			} else if err == nil || block != nil {
				t.Fatal("capacity admitted")
			}
			if !reflect.DeepEqual(events, before) {
				t.Fatal("events changed")
			}
		})
	}
	events := []macros.Event{{Type: macros.Keyboard, KeyCode: "ControlLeft", Action: macros.Down, DelayMS: 51199}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down}, {Type: macros.MouseRight, Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "ControlLeft", Action: macros.Up}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up}, {Type: macros.MouseRight, Action: macros.Up, DelayMS: 51199}}
	block, err := AdmitX6LocalBlock(events, 1, 0)
	if err != nil || block[25] != 9 {
		t.Fatal(err, block)
	}
	if !bytes.Equal(block[26:30], []byte{0x15, 0xe0, 255, 3}) || !bytes.Equal(block[42:44], []byte{0x81, 0xf2}) {
		t.Fatalf("extension/terminal mismatch: %x", block)
	}
	events[len(events)-1].DelayMS = 51200
	if block, err := AdmitX6LocalBlock(events, 1, 0); err == nil || block != nil {
		t.Fatal("terminal source overflow admitted")
	}
}

func TestX6AdmissionLegacyBytes(t *testing.T) {
	for _, button := range x6SequenceButtons {
		events := []macros.Event{{Type: button, Action: macros.Down}, {Type: button, Action: macros.Up}}
		got, err := AdmitX6LocalBlock(events, 255, 2)
		if err != nil {
			t.Fatal(err)
		}
		want, err := EncodeX6SequenceBlock(X6Sequence{Buttons: []macros.EventType{button}, Repeat: 255, Mode: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("legacy bytes changed for %s", button)
		}
	}
}
