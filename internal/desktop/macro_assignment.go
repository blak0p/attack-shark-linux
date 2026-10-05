package desktop

import (
	"context"
	"reflect"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

// MacroDraft is one local assignment, not a firmware slot inventory. Events are
// frozen at staging; later library edits require explicit restaging.
type MacroDraft struct {
	ID, Name string
	Button   uint8
	Repeat   int
	Events   []macros.Event
}

func cloneMacroDraft(d *MacroDraft) *MacroDraft {
	if d == nil {
		return nil
	}
	copy := *d
	copy.Events = append([]macros.Event(nil), d.Events...)
	return &copy
}
func admittedClick(events []macros.Event, repeat int) (macros.X6Click, bool) {
	if repeat < 1 || repeat > 255 || len(events) != 2 {
		return macros.X6Click{}, false
	}
	down, up := events[0], events[1]
	if down.Type != up.Type || (down.Type != macros.MouseLeft && down.Type != macros.MouseRight) || down.Action != macros.Down || up.Action != macros.Up || down.DelayMS != 0 || up.DelayMS != 0 {
		return macros.X6Click{}, false
	}
	return macros.X6Click{Button: down.Type, Repeat: repeat}, true
}

// StageMacroAssignment replaces the single selected-device assignment without
// writing hardware or modifying other pending remap fields.
func (s *Service) StageMacroAssignment(id string, button uint8, repeat int) RemapSnapshot {
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(s.remapComponent.legacy, SelectionRequired)
	}
	state := s.remapComponent.stateForBinding(binding)
	m, err := s.ReadMacro(id)
	_, destinationErr := protocol.MacroDestinationForButton(button)
	_, valid := admittedClick(m.Events, repeat)
	if err != nil || destinationErr != nil || !valid {
		return failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		state.err = Error{Code: StaleBinding}
		return remapSnapshotLocked(state)
	}
	state.macroPending = &MacroDraft{ID: m.ID, Name: m.Name, Button: button, Repeat: repeat, Events: append([]macros.Event(nil), m.Events...)}
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

// StageRemap stages ordinary fields without changing the separate macro overlay.
// Returning the target button to an ordinary action requires ClearMacroAssignment.
func (s *Service) StageRemap(config x6.RemapConfig) RemapSnapshot {
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(s.remapComponent.legacy, SelectionRequired)
	}
	state := s.remapComponent.stateForBinding(binding)
	if err := x6.NewRemapOperation().Validate(config); err != nil {
		return failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		state.err = Error{Code: StaleBinding}
		return remapSnapshotLocked(state)
	}
	state.pending = cloneRemapConfig(config)
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

// ClearMacroAssignment returns to the ordinary remap draft, never a device write.
func (s *Service) ClearMacroAssignment() RemapSnapshot {
	state := s.currentRemapState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.macroPending = nil
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

// DiscardRemap restores ordinary applied fields but deliberately does not restore
// a previously applied macro: no readback or independent slots are inferred.
func (s *Service) DiscardRemap() RemapSnapshot {
	state := s.currentRemapState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.pending = cloneRemapConfig(state.applied)
	state.macroPending = nil
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

func (s *Service) GetMacroAssignmentSnapshot() RemapSnapshot { return s.GetRemapSnapshot() }

func (s *Service) macroDraftCurrent(d *MacroDraft) bool {
	m, err := s.ReadMacro(d.ID)
	return err == nil && m.Name == d.Name && reflect.DeepEqual(m.Events, d.Events)
}

// The remap apply guards are held. The immutable copy is what reaches transport,
// even if the library changes while I/O is in flight. Completion checks reject
// stale drafts and retain the observed progress instead of claiming full apply.
func (c *remapComponent) applyMacroBound(binding Binding, state *remapState, revision uint64, pending x6.RemapConfig, draft *MacroDraft) {
	s := c.service
	if !s.bindingCurrent(binding) {
		failRemap(state, StaleBinding)
		return
	}
	if !s.macroDraftCurrent(draft) {
		failRemap(state, InvalidConfiguration)
		return
	}
	click, valid := admittedClick(draft.Events, draft.Repeat)
	if !valid {
		failRemap(state, InvalidConfiguration)
		return
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		failRemap(state, StaleBinding)
		return
	}
	progress, err := inventory.ApplyMacroAssignmentBound(context.Background(), binding, protocol.MacroAssignment{Config: pending, Button: draft.Button}, click)
	libraryCurrent := s.macroDraftCurrent(draft)
	state.mu.Lock()
	state.macroProgress = progress
	if state.revision != revision || !s.bindingCurrent(binding) || !libraryCurrent {
		state.firmware, state.err = "failed", Error{Code: StaleBinding}
	} else if err != nil {
		state.firmware, state.err = "failed", Error{Code: errorCode(err, false)}
	} else if progress.Assignment != mouse.MacroAssignmentACKConfirmed || progress.Upload != mouse.MacroUploadConfirmed {
		state.firmware, state.err = "failed", Error{Code: ApplyFailed}
	} else {
		state.applied = cloneRemapConfig(pending)
		state.macroApplied = cloneMacroDraft(draft)
		state.firmware, state.err = "success", Error{}
		// Ordinary config persistence cannot represent this macro overlay. Do not
		// persist an ordinary config as though it described the uploaded assignment.
		state.persistence = "not_supported"
	}
	state.mu.Unlock()
	s.emitRemapConfiguration(binding, remapSnapshotOf(state))
}
