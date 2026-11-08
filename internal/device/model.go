package device

import (
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// MacroSpec is one local macro assignment handed to a model for admission and
// encoding.
type MacroSpec struct {
	Button uint8
	Events []macros.Event
	Repeat int
	Mode   uint8
}

// Model is what generic code needs from one mouse model's driver: the
// operations that encode settings for the wire, the model's validation rules,
// its lighting catalog and macro admission/encoding. The application wiring
// injects the concrete model; generic packages never import it.
type Model interface {
	DPIOperation() mouse.CommandOperation
	PollingOperation() mouse.CommandOperation
	LightingOperation() mouse.CommandOperation
	LightingSettingsOperation() mouse.CommandOperation
	RemapOperation() mouse.CommandOperation

	ValidateNormalSleepMinutes(minutes float64) error
	ValidateResponseTime(responseTimeMs int) error
	LightingEffects() []LightingEffect

	// AdmitMacro checks that one macro can be assigned to the logical button.
	AdmitMacro(button uint8, events []macros.Event, repeat int, mode uint8) error
	// ValidateMacroAssignments checks every encoding step of a pending remap
	// plus its macro assignments without producing I/O.
	ValidateMacroAssignments(pending RemapConfig, specs []MacroSpec) error
	// MacroAssignmentBatch encodes the pending remap and its macro uploads as
	// one transaction for TargetedService.ApplyBatchBound.
	MacroAssignmentBatch(pending RemapConfig, specs []MacroSpec) (mouse.Batch, error)
}
