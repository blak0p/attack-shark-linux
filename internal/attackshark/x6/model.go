package x6

import (
	"github.com/blak0p/attack-shark-linux/internal/attackshark/x6/macrocodec"
	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// Model returns the X6 driver behind the model-neutral device.Model seam.
func Model() device.Model { return model{} }

type model struct{}

func (model) DPIOperation() mouse.CommandOperation      { return NewDPIOperation() }
func (model) PollingOperation() mouse.CommandOperation  { return NewPollingOperation() }
func (model) LightingOperation() mouse.CommandOperation { return NewLightingOperation() }
func (model) LightingSettingsOperation() mouse.CommandOperation {
	return NewLightingSettingsOperation()
}
func (model) RemapOperation() mouse.CommandOperation { return NewRemapOperation() }

func (model) ValidateNormalSleepMinutes(minutes float64) error {
	return ValidateNormalSleepMinutes(minutes)
}
func (model) ValidateResponseTime(responseTimeMs int) error {
	return ValidateResponseTime(responseTimeMs)
}
func (model) LightingEffects() []device.LightingEffect { return LightingEffects() }

func (model) AdmitMacro(button uint8, events []macros.Event, repeat int, mode uint8) error {
	if _, err := protocol.MacroDestinationForButton(button); err != nil {
		return err
	}
	_, err := macrocodec.AdmitX6LocalBlock(events, repeat, mode)
	return err
}

func (model) ValidateMacroAssignments(pending device.RemapConfig, specs []device.MacroSpec) error {
	buttons := make([]uint8, len(specs))
	for i, spec := range specs {
		buttons[i] = spec.Button
		destination, err := protocol.MacroDestinationForButton(spec.Button)
		if err != nil {
			return err
		}
		if _, err := macrocodec.EncodeX6LocalUpload(destination, spec.Events, spec.Repeat, spec.Mode); err != nil {
			return err
		}
	}
	_, err := protocol.EncodeMultiMacroAssignmentReport(pending, buttons)
	return err
}

// MacroAssignmentBatch uses the legacy click-sequence upload when every macro
// fits it, and the exact local-event upload otherwise.
func (model) MacroAssignmentBatch(pending device.RemapConfig, specs []device.MacroSpec) (mouse.Batch, error) {
	items := make([]MacroSequenceItem, len(specs))
	localItems := make([]MacroLocalItem, len(specs))
	legacy := true
	for i, spec := range specs {
		sequence, valid := admittedSequenceWithMode(spec.Events, spec.Repeat, spec.Mode)
		legacy = legacy && valid
		localItems[i] = MacroLocalItem{Button: spec.Button, Events: append([]macros.Event(nil), spec.Events...), Repeat: spec.Repeat, Mode: spec.Mode}
		items[i] = MacroSequenceItem{Button: spec.Button, Sequence: sequence}
	}
	if legacy {
		return MultiMacroSequenceBatch(pending, items)
	}
	return MultiMacroLocalBatch(pending, localItems)
}

func admittedSequence(events []macros.Event, repeat int) (macrocodec.X6Sequence, bool) {
	return admittedSequenceWithMode(events, repeat, 0)
}

func admittedSequenceWithMode(events []macros.Event, repeat int, mode uint8) (macrocodec.X6Sequence, bool) {
	if repeat < 1 || repeat > 255 || (len(events) != 2 && len(events) != 4) {
		return macrocodec.X6Sequence{}, false
	}
	sequence := macrocodec.X6Sequence{Repeat: repeat, Mode: mode}
	for i := 0; i < len(events); i += 2 {
		down, up := events[i], events[i+1]
		if down.KeyCode != "" || up.KeyCode != "" || down.Type != up.Type || down.Action != macros.Down || up.Action != macros.Up || down.DelayMS != 0 || up.DelayMS != 0 {
			return macrocodec.X6Sequence{}, false
		}
		sequence.Buttons = append(sequence.Buttons, down.Type)
	}
	// Keep the captured action/count/repeat boundary owned by the shared codec.
	if _, err := macrocodec.EncodeX6SequenceBlock(sequence); err != nil {
		return macrocodec.X6Sequence{}, false
	}
	return sequence, true
}
