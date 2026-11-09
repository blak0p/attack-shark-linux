package x6

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

func mustHex(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The emergency reset writes these documented values; the model must encode
// them to the captured bytes.
func TestModelEncodesCapturedResetBytes(t *testing.T) {
	model := Model()
	for _, tt := range []struct {
		name      string
		operation mouse.CommandOperation
		value     any
		want      string
	}{
		{"dpi", model.DPIOperation(), device.DocumentedResetDPIConfig(), "04380100003f00010f171f3f6f070000000000000002000002ff000000ff000000ffffff0000ffffff00ffff4000ffffff010e7200000000"},
		{"settings", model.LightingSettingsOperation(), device.DocumentedResetLightingSettings(), "050f015003a800ff00010401ff"},
		{"polling", model.PollingOperation(), device.PollingRate1000, "06090101fe00000000"},
		{"remap", model.RemapOperation(), device.DefaultRemapConfig(), "083b010200000300000400000d00000e00000f00000600000500003c00000100000100000100000100000100000100000100000a00000900000094"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.operation.Encode(tt.value)
			if err != nil || !reflect.DeepEqual(got, mustHex(t, tt.want)) {
				t.Fatalf("Encode() = %x, %v; want %s", got, err, tt.want)
			}
		})
	}
}

func TestModelValidationMatchesProtocolLimits(t *testing.T) {
	model := Model()
	if model.ValidateResponseTime(device.ResponseTimeMinMs) != nil || model.ValidateResponseTime(device.ResponseTimeMaxMs+1) == nil {
		t.Fatal("response time limits changed")
	}
	if model.ValidateNormalSleepMinutes(device.NormalSleepMinimumMinutes) != nil || model.ValidateNormalSleepMinutes(0.75) == nil {
		t.Fatal("normal sleep limits changed")
	}
	if got := model.LightingEffects(); len(got) != len(LightingEffects()) {
		t.Fatalf("LightingEffects() = %d entries, want %d", len(got), len(LightingEffects()))
	}
}

func modelKey(code string, action macros.Action) macros.Event {
	return macros.Event{Type: macros.Keyboard, KeyCode: code, Action: action, DelayMS: 10}
}

func modelClicks(count int) []macros.Event {
	var events []macros.Event
	for i := 0; i < count; i++ {
		events = append(events, macros.Event{Type: macros.MouseLeft, Action: macros.Down}, macros.Event{Type: macros.MouseLeft, Action: macros.Up})
	}
	return events
}

func TestModelAdmitMacro(t *testing.T) {
	model := Model()
	click := modelClicks(1)
	for _, tt := range []struct {
		name   string
		button uint8
		events []macros.Event
		repeat int
		mode   uint8
		ok     bool
	}{
		{"click", 6, click, 1, 0, true},
		{"three clicks", 1, modelClicks(3), 255, 2, true},
		{"button zero", 0, click, 1, 0, false},
		{"button eight", 8, click, 1, 0, false},
		{"repeat zero", 6, click, 0, 0, false},
		{"repeat 256", 6, click, 256, 0, false},
		{"mode 3", 6, click, 1, 3, false},
		{"empty", 6, nil, 1, 0, false},
		{"oversize", 7, modelClicks(24), 1, 0, false},
		{"simultaneous aliases", 7, []macros.Event{modelKey("Enter", macros.Down), modelKey("NumpadEnter", macros.Down), modelKey("Enter", macros.Up), modelKey("NumpadEnter", macros.Up)}, 1, 0, false},
		{"alias release", 7, []macros.Event{modelKey("Enter", macros.Down), modelKey("NumpadEnter", macros.Up)}, 1, 0, false},
		{"repeated down", 7, []macros.Event{modelKey("Enter", macros.Down), modelKey("Enter", macros.Down), modelKey("Enter", macros.Up)}, 1, 0, false},
		{"prototype identity", 7, []macros.Event{modelKey("constructor", macros.Down), modelKey("constructor", macros.Up)}, 1, 0, false},
		{"unsupported key", 7, []macros.Event{modelKey("F13", macros.Down), modelKey("F13", macros.Up)}, 1, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := model.AdmitMacro(tt.button, tt.events, tt.repeat, tt.mode); (err == nil) != tt.ok {
				t.Fatalf("AdmitMacro() error = %v, want ok=%t", err, tt.ok)
			}
		})
	}
}

func TestAdmittedSequenceRejectsInvalidData(t *testing.T) {
	for _, events := range [][]macros.Event{
		{{Type: "keyboard", Action: macros.Down}, {Type: "keyboard", Action: macros.Up}},
		{{Type: macros.MouseLeft, Action: "unknown"}, {Type: macros.MouseLeft, Action: macros.Up}},
		{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: -1}, {Type: macros.MouseLeft, Action: macros.Up}},
	} {
		if _, ok := admittedSequence(events, 1); ok {
			t.Fatal("invalid data admitted")
		}
	}
}

func sameBatchBytes(got, want mouse.Batch) bool {
	return reflect.DeepEqual(got.Setup, want.Setup) && reflect.DeepEqual(got.Chunks, want.Chunks) && got.Spacing == want.Spacing
}

// Click-only macros use the legacy sequence upload; anything else uses the
// exact local-event upload. Either way the pending remap rides in report 08.
func TestModelMacroAssignmentBatchSelectsUploadKind(t *testing.T) {
	model := Model()
	pending := device.DefaultRemapConfig()
	pending.Buttons[3].Action = device.RemapFire
	clicks := modelClicks(2)
	chord := []macros.Event{{Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up, DelayMS: 100}}

	sequence, ok := admittedSequenceWithMode(clicks, 3, 1)
	if !ok {
		t.Fatal("clicks are not a legacy sequence")
	}
	legacyWant, err := MultiMacroSequenceBatch(pending, []MacroSequenceItem{{Button: 6, Sequence: sequence}})
	if err != nil {
		t.Fatal(err)
	}
	legacyGot, err := model.MacroAssignmentBatch(pending, []device.MacroSpec{{Button: 6, Events: clicks, Repeat: 3, Mode: 1}})
	if err != nil || !sameBatchBytes(legacyGot, legacyWant) {
		t.Fatalf("legacy batch = %+v, %v; want %+v", legacyGot, err, legacyWant)
	}

	localWant, err := MultiMacroLocalBatch(pending, []MacroLocalItem{{Button: 6, Events: clicks, Repeat: 255}, {Button: 7, Events: chord, Repeat: 1, Mode: 2}})
	if err != nil {
		t.Fatal(err)
	}
	localGot, err := model.MacroAssignmentBatch(pending, []device.MacroSpec{{Button: 6, Events: clicks, Repeat: 255}, {Button: 7, Events: chord, Repeat: 1, Mode: 2}})
	if err != nil || !sameBatchBytes(localGot, localWant) {
		t.Fatalf("local batch = %+v, %v; want %+v", localGot, err, localWant)
	}
}

func TestModelValidateMacroAssignments(t *testing.T) {
	model := Model()
	pending := device.DefaultRemapConfig()
	click := modelClicks(1)
	if err := model.ValidateMacroAssignments(pending, []device.MacroSpec{{Button: 6, Events: click, Repeat: 1}}); err != nil {
		t.Fatalf("valid assignment rejected: %v", err)
	}
	if err := model.ValidateMacroAssignments(pending, []device.MacroSpec{{Button: 8, Events: click, Repeat: 1}}); err == nil {
		t.Fatal("button 8 accepted")
	}
	if err := model.ValidateMacroAssignments(device.RemapConfig{}, []device.MacroSpec{{Button: 6, Events: click, Repeat: 1}}); err == nil {
		t.Fatal("invalid pending remap accepted")
	}
}
