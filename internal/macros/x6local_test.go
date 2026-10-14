package macros

import (
	"bytes"
	"reflect"
	"testing"
)

func TestX6LocalMapping(t *testing.T) {
	for _, tt := range []struct {
		name  string
		event Event
		code  byte
	}{
		{"A", Event{Type: Keyboard, Action: Down, DelayMS: 100, KeyCode: "KeyA"}, 0x04},
		{"left control", Event{Type: Keyboard, Action: Up, DelayMS: 1271, KeyCode: "ControlLeft"}, 0xe0},
		{"left shift", Event{Type: Keyboard, Action: Down, DelayMS: 12, KeyCode: "ShiftLeft"}, 0xe1},
		{"left mouse", Event{Type: MouseLeft, Action: Down, DelayMS: 0}, 0xf1},
		{"right mouse", Event{Type: MouseRight, Action: Up, DelayMS: 1}, 0xf2},
		{"middle mouse", Event{Type: MouseMiddle, Action: Down, DelayMS: 200}, 0xf3},
		{"back mouse", Event{Type: MouseBack, Action: Up, DelayMS: 51199}, 0xf4},
		{"forward mouse", Event{Type: MouseForward, Action: Down, DelayMS: 20}, 0xf5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []Event{tt.event, {Type: Keyboard, Action: Up, DelayMS: 234, KeyCode: "KeyA"}}
			before := append([]Event(nil), input...)
			got, err := MapX6LocalEvents(input)
			want := []X6TimedEvent{{Code: tt.code, Action: tt.event.Action, DelayMS: tt.event.DelayMS}, {Code: 4, Action: Up, DelayMS: 234}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, %v; want %v", got, err, want)
			}
			block, err := EncodeX6TimedBlock(got, 1, 0)
			expected, expectedErr := EncodeX6TimedBlock(want, 1, 0)
			if err != nil || expectedErr != nil || !bytes.Equal(block, expected) {
				t.Fatalf("forward timing mismatch: %v / %v", err, expectedErr)
			}
			got[0].DelayMS = 9
			if !reflect.DeepEqual(input, before) {
				t.Fatal("input changed or aliased")
			}
		})
	}
}

func TestX6LocalMappingRejectsWithoutLoss(t *testing.T) {
	for _, tt := range []struct {
		name  string
		event Event
	}{
		{"unknown local key", Event{Type: Keyboard, Action: Down, KeyCode: "UnknownPhysicalKey"}},
		{"F13", Event{Type: Keyboard, Action: Down, KeyCode: "F13"}},
		{"media", Event{Type: Keyboard, Action: Down, KeyCode: "MediaPlayPause"}},
		{"ISO OEM102", Event{Type: Keyboard, Action: Down, KeyCode: "IntlBackslash"}},
		{"generic modifier", Event{Type: Keyboard, Action: Down, KeyCode: "Control"}},
		{"verbatim padded key", Event{Type: Keyboard, Action: Down, KeyCode: " KeyA "}},
		{"blank key", Event{Type: Keyboard, Action: Down, KeyCode: " \t"}},
		{"stale mouse key", Event{Type: MouseLeft, Action: Down, KeyCode: " "}},
		{"unknown mouse", Event{Type: "mouse_unknown", Action: Down}},
		{"bad action", Event{Type: Keyboard, Action: "click", KeyCode: "KeyA"}},
		{"negative delay", Event{Type: Keyboard, Action: Down, DelayMS: -1, KeyCode: "KeyA"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []Event{{Type: Keyboard, Action: Down, KeyCode: "KeyA"}, tt.event}
			before := append([]Event(nil), input...)
			got, err := MapX6LocalEvents(input)
			if err == nil || got != nil {
				t.Fatalf("got %v, %v; want nil and error", got, err)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("rejected input changed")
			}
		})
	}
}

func TestX6LocalMappingLeavesCodecAdmissionSeparate(t *testing.T) {
	input := []Event{{Type: Keyboard, Action: Down, DelayMS: 65536, KeyCode: "KeyA"}}
	got, err := MapX6LocalEvents(input)
	if err != nil || got[0].DelayMS != 65536 {
		t.Fatalf("local delay lost: %v, %v", got, err)
	}
	if block, err := EncodeX6TimedBlock(got, 1, 0); err == nil || block != nil {
		t.Fatalf("codec must reject: %v, %v", block, err)
	}
	for _, input := range [][]Event{nil, {}} {
		got, err := MapX6LocalEvents(input)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("empty local mapping: %v, %v", got, err)
		}
		if block, err := EncodeX6TimedBlock(got, 1, 0); err == nil || block != nil {
			t.Fatal("empty block admitted")
		}
	}
}
