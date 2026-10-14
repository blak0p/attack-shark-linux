package macros

import (
	"fmt"
	"reflect"
	"testing"
)

// Independent expected converter rows from the supplied official PE evidence,
// not values read back out of x6keys.json. Browser names describe positions.
func officialExpectedKeys() map[string]byte {
	expected := map[string]byte{}
	for i := 0; i < 26; i++ {
		expected[fmt.Sprintf("Key%c", 'A'+i)] = byte(4 + i)
	}
	for i := 1; i <= 9; i++ {
		expected[fmt.Sprintf("Digit%d", i)] = byte(0x1d + i)
	}
	expected["Digit0"] = 0x27
	for i, key := range []string{"Enter", "Escape", "Backspace", "Tab", "Space", "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash"} {
		expected[key] = byte(0x28 + i)
	}
	for i, key := range []string{"Semicolon", "Quote", "Backquote", "Comma", "Period", "Slash", "CapsLock"} {
		expected[key] = byte(0x33 + i)
	}
	for i := 1; i <= 12; i++ {
		expected[fmt.Sprintf("F%d", i)] = byte(0x39 + i)
	}
	for i, key := range []string{"PrintScreen", "ScrollLock", "Pause", "Insert", "Home", "PageUp", "Delete", "End", "PageDown", "ArrowRight", "ArrowLeft", "ArrowDown", "ArrowUp", "NumLock", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd"} {
		expected[key] = byte(0x46 + i)
	}
	for i := 1; i <= 9; i++ {
		expected[fmt.Sprintf("Numpad%d", i)] = byte(0x58 + i)
	}
	expected["Numpad0"] = 0x62
	expected["NumpadDecimal"] = 0x63
	expected["ContextMenu"] = 0x65
	for i, key := range []string{"ControlLeft", "ShiftLeft", "AltLeft", "MetaLeft", "ControlRight", "ShiftRight", "AltRight", "MetaRight"} {
		expected[key] = byte(0xe0 + i)
	}
	expected["NumpadEnter"] = 0x28 // explicit inferential VK_RETURN alias, not separator
	return expected
}

func TestX6OfficialEveryPhysicalIdentity(t *testing.T) {
	expected := officialExpectedKeys()
	if len(expected) != 104 || !reflect.DeepEqual(x6KeyboardCodes, expected) {
		t.Fatalf("canonical table differs from independent 104-identity expectation: got %v", x6KeyboardCodes)
	}
	for key, code := range expected {
		t.Run(key, func(t *testing.T) {
			events := []Event{{Type: Keyboard, KeyCode: key, Action: Down, DelayMS: 100}, {Type: Keyboard, KeyCode: key, Action: Up, DelayMS: 234}}
			before := append([]Event(nil), events...)
			got, err := MapX6LocalEvents(events)
			want := []X6TimedEvent{{Code: code, Action: Down, DelayMS: 100}, {Code: code, Action: Up, DelayMS: 234}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v/%v want %v", got, err, want)
			}
			block, err := AdmitX6LocalBlock(events, 1, 0)
			if err != nil || !reflect.DeepEqual(block[26:30], []byte{0x0b, code, 0x81, code}) {
				t.Fatalf("encoded key/terminal rule: %x/%v", block, err)
			}
			if !reflect.DeepEqual(events, before) {
				t.Fatal("source changed")
			}
		})
	}
}

func TestX6OfficialUnsupportedExactIdentities(t *testing.T) {
	for _, key := range []string{"__proto__", "constructor", "toString", "Unknown", "keya", " KeyA", "KeyA ", "F13", "F24", "IntlBackslash", "MediaPlayPause", "BrowserBack", "Control", "Shift", "Alt", "NumpadSeparator", "OEM8", "Lang1", "Convert", "NonConvert"} {
		t.Run(key, func(t *testing.T) {
			events := []Event{{Type: Keyboard, KeyCode: key, Action: Down}, {Type: Keyboard, KeyCode: key, Action: Up}}
			before := append([]Event(nil), events...)
			got, err := EncodeX6LocalUpload(5, events, 1, 0)
			if err == nil || got != nil || !reflect.DeepEqual(events, before) {
				t.Fatalf("unsupported key admitted or lost: %v/%v", got, err)
			}
		})
	}
}

func TestX6OfficialAliasBalance(t *testing.T) {
	key := func(code string, action Action) Event {
		return Event{Type: Keyboard, KeyCode: code, Action: action, DelayMS: 10}
	}
	for _, tt := range []struct {
		name   string
		events []Event
		valid  bool
	}{
		{"sequential aliases", []Event{key("Enter", Down), key("Enter", Up), key("NumpadEnter", Down), key("NumpadEnter", Up)}, true},
		{"simultaneous aliases", []Event{key("Enter", Down), key("NumpadEnter", Down), key("Enter", Up), key("NumpadEnter", Up)}, false},
		{"mismatched alias release", []Event{key("Enter", Down), key("NumpadEnter", Up)}, false},
		{"exact release", []Event{key("NumpadEnter", Down), key("NumpadEnter", Up)}, true},
		{"repeated exact down", []Event{key("Enter", Down), key("Enter", Down), key("Enter", Up)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]Event(nil), tt.events...)
			block, err := AdmitX6LocalBlock(tt.events, 1, 0)
			if (err == nil) != tt.valid || !reflect.DeepEqual(tt.events, before) {
				t.Fatalf("got %x/%v valid=%t", block, err, tt.valid)
			}
			if !tt.valid && block != nil {
				t.Fatal("partial output on rejection")
			}
			if tt.valid {
				for i := range tt.events {
					if block[27+2*i] != 0x28 {
						t.Fatal("alias raw identity changed")
					}
				}
			}
		})
	}
}
