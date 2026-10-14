package macros

import (
	"bytes"
	"reflect"
	"testing"
)

func TestX6AdmissionUploadFraming(t *testing.T) {
	events := []Event{{Type: Keyboard, KeyCode: "KeyA", Action: Down, DelayMS: 10}, {Type: Keyboard, KeyCode: "KeyA", Action: Up}}
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
	events := []Event{{Type: Keyboard, KeyCode: "ShiftLeft", Action: Down, DelayMS: 100}, {Type: Keyboard, KeyCode: "KeyA", Action: Down, DelayMS: 100}, {Type: Keyboard, KeyCode: "KeyA", Action: Up, DelayMS: 100}, {Type: Keyboard, KeyCode: "ShiftLeft", Action: Up, DelayMS: 100}}
	block, err := AdmitX6LocalBlock(events, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(block[26:34], []byte{0x0b, 0xe1, 0x0b, 0x04, 0x8b, 0x04, 0x81, 0xe1}) || !bytes.Equal(block[126:], []byte{2, 0xf1}) {
		t.Fatalf("captured block mismatch: %x", block)
	}
}

func TestX6AdmissionRejectsUnsafeLocal(t *testing.T) {
	a := Event{Type: Keyboard, KeyCode: "KeyA", Action: Down}
	up := a
	up.Action = Up
	for _, tc := range []struct {
		name   string
		events []Event
		repeat int
		mode   uint8
	}{
		{"incomplete", []Event{a}, 1, 0},
		{"unmatched", []Event{up, a}, 1, 0},
		{"repeated down", []Event{a, a, up}, 1, 0},
		{"unsupported", []Event{{Type: Keyboard, KeyCode: "F13", Action: Down}, {Type: Keyboard, KeyCode: "F13", Action: Up}}, 1, 0},
		{"padded", []Event{{Type: Keyboard, KeyCode: " KeyA", Action: Down}, {Type: Keyboard, KeyCode: " KeyA", Action: Up}}, 1, 0},
		{"mouse ambiguity", []Event{{Type: MouseLeft, KeyCode: "KeyA", Action: Down}, {Type: MouseLeft, Action: Up}}, 1, 0},
		{"repeat", []Event{a, up}, 0, 0},
		{"mode", []Event{a, up}, 1, 3},
		{"overflow", []Event{{Type: Keyboard, KeyCode: "KeyA", Action: Down, DelayMS: 51200}, up}, 1, 0},
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
	clicks := func(count int) []Event {
		var events []Event
		for i := 0; i < count; i++ {
			events = append(events, Event{Type: MouseLeft, Action: Down}, Event{Type: MouseLeft, Action: Up})
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
			before := append([]Event(nil), events...)
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
	events := []Event{{Type: Keyboard, KeyCode: "ControlLeft", Action: Down, DelayMS: 51199}, {Type: Keyboard, KeyCode: "ShiftLeft", Action: Down}, {Type: Keyboard, KeyCode: "KeyA", Action: Down}, {Type: MouseRight, Action: Down}, {Type: Keyboard, KeyCode: "ControlLeft", Action: Up}, {Type: Keyboard, KeyCode: "ShiftLeft", Action: Up}, {Type: Keyboard, KeyCode: "KeyA", Action: Up}, {Type: MouseRight, Action: Up, DelayMS: 51199}}
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
		events := []Event{{Type: button, Action: Down}, {Type: button, Action: Up}}
		got, err := AdmitX6LocalBlock(events, 255, 2)
		if err != nil {
			t.Fatal(err)
		}
		want, err := EncodeX6SequenceBlock(X6Sequence{Buttons: []EventType{button}, Repeat: 255, Mode: 2})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("legacy bytes changed for %s", button)
		}
	}
}
