package macrocodec

import (
	"bytes"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"reflect"
	"testing"
)

func TestX6LocalMapping(t *testing.T) {
	for _, tt := range []struct {
		name  string
		event macros.Event
		code  byte
	}{
		{"A", macros.Event{Type: macros.Keyboard, Action: macros.Down, DelayMS: 100, KeyCode: "KeyA"}, 0x04},
		{"left control", macros.Event{Type: macros.Keyboard, Action: macros.Up, DelayMS: 1271, KeyCode: "ControlLeft"}, 0xe0},
		{"left shift", macros.Event{Type: macros.Keyboard, Action: macros.Down, DelayMS: 12, KeyCode: "ShiftLeft"}, 0xe1},
		{"left mouse", macros.Event{Type: macros.MouseLeft, Action: macros.Down, DelayMS: 0}, 0xf1},
		{"right mouse", macros.Event{Type: macros.MouseRight, Action: macros.Up, DelayMS: 1}, 0xf2},
		{"middle mouse", macros.Event{Type: macros.MouseMiddle, Action: macros.Down, DelayMS: 200}, 0xf3},
		{"back mouse", macros.Event{Type: macros.MouseBack, Action: macros.Up, DelayMS: 51199}, 0xf4},
		{"forward mouse", macros.Event{Type: macros.MouseForward, Action: macros.Down, DelayMS: 20}, 0xf5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []macros.Event{tt.event, {Type: macros.Keyboard, Action: macros.Up, DelayMS: 234, KeyCode: "KeyA"}}
			before := append([]macros.Event(nil), input...)
			got, err := MapX6LocalEvents(input)
			want := []X6TimedEvent{{Code: tt.code, Action: tt.event.Action, DelayMS: tt.event.DelayMS}, {Code: 4, Action: macros.Up, DelayMS: 234}}
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
		event macros.Event
	}{
		{"unknown local key", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: "UnknownPhysicalKey"}},
		{"F13", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: "F13"}},
		{"media", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: "MediaPlayPause"}},
		{"ISO OEM102", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: "IntlBackslash"}},
		{"generic modifier", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: "Control"}},
		{"verbatim padded key", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: " KeyA "}},
		{"blank key", macros.Event{Type: macros.Keyboard, Action: macros.Down, KeyCode: " \t"}},
		{"stale mouse key", macros.Event{Type: macros.MouseLeft, Action: macros.Down, KeyCode: " "}},
		{"unknown mouse", macros.Event{Type: "mouse_unknown", Action: macros.Down}},
		{"bad action", macros.Event{Type: macros.Keyboard, Action: "click", KeyCode: "KeyA"}},
		{"negative delay", macros.Event{Type: macros.Keyboard, Action: macros.Down, DelayMS: -1, KeyCode: "KeyA"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := []macros.Event{{Type: macros.Keyboard, Action: macros.Down, KeyCode: "KeyA"}, tt.event}
			before := append([]macros.Event(nil), input...)
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
	input := []macros.Event{{Type: macros.Keyboard, Action: macros.Down, DelayMS: 65536, KeyCode: "KeyA"}}
	got, err := MapX6LocalEvents(input)
	if err != nil || got[0].DelayMS != 65536 {
		t.Fatalf("local delay lost: %v, %v", got, err)
	}
	if block, err := EncodeX6TimedBlock(got, 1, 0); err == nil || block != nil {
		t.Fatalf("codec must reject: %v, %v", block, err)
	}
	for _, input := range [][]macros.Event{nil, {}} {
		got, err := MapX6LocalEvents(input)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("empty local mapping: %v, %v", got, err)
		}
		if block, err := EncodeX6TimedBlock(got, 1, 0); err == nil || block != nil {
			t.Fatal("empty block admitted")
		}
	}
}
