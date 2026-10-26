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
	Mode     uint8
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
func admittedSequence(events []macros.Event, repeat int) (macros.X6Sequence, bool) {
	return admittedSequenceWithMode(events, repeat, 0)
}

func admittedSequenceWithMode(events []macros.Event, repeat int, mode uint8) (macros.X6Sequence, bool) {
	if repeat < 1 || repeat > 255 || (len(events) != 2 && len(events) != 4) {
		return macros.X6Sequence{}, false
	}
	sequence := macros.X6Sequence{Repeat: repeat, Mode: mode}
	for i := 0; i < len(events); i += 2 {
		down, up := events[i], events[i+1]
		if down.KeyCode != "" || up.KeyCode != "" || down.Type != up.Type || down.Action != macros.Down || up.Action != macros.Up || down.DelayMS != 0 || up.DelayMS != 0 {
			return macros.X6Sequence{}, false
		}
		sequence.Buttons = append(sequence.Buttons, down.Type)
	}
	// Keep the captured action/count/repeat boundary owned by the shared codec.
	if _, err := macros.EncodeX6SequenceBlock(sequence); err != nil {
		return macros.X6Sequence{}, false
	}
	return sequence, true
}

// StageMacroAssignment replaces the single selected-device assignment without
// writing hardware or modifying other pending remap fields.
func (s *Service) StageMacroAssignment(id string, button uint8, repeat int) RemapSnapshot {
	return s.StageMacroAssignmentWithMode(id, button, repeat, 0)
}

// StageMacroAssignmentWithMode stages assignment playback metadata without I/O.
func (s *Service) StageMacroAssignmentWithMode(id string, button uint8, repeat int, mode uint8) RemapSnapshot {
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(s.remapComponent.legacy, SelectionRequired)
	}
	state := s.remapComponent.stateForBinding(binding)
	m, err := s.ReadMacro(id)
	_, destinationErr := protocol.MacroDestinationForButton(button)
	_, admissionErr := macros.AdmitX6LocalBlock(m.Events, repeat, mode)
	if err != nil || destinationErr != nil || admissionErr != nil {
		return failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		state.err = Error{Code: StaleBinding}
		return remapSnapshotLocked(state)
	}
	if state.macroPendingMap == nil {
		state.macroPendingMap = make(map[uint8]*MacroDraft)
	}
	draft := &MacroDraft{ID: m.ID, Name: m.Name, Button: button, Repeat: repeat, Mode: mode, Events: append([]macros.Event(nil), m.Events...)}
	state.macroPendingMap[button] = draft
	state.macroPending = draft
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

// ClearButtonMacroAssignment clears a specific button's staged macro assignment.
func (s *Service) ClearButtonMacroAssignment(button uint8) RemapSnapshot {
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(s.remapComponent.legacy, SelectionRequired)
	}
	state := s.remapComponent.stateForBinding(binding)
	state.mu.Lock()
	defer state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		state.err = Error{Code: StaleBinding}
		return remapSnapshotLocked(state)
	}
	if state.macroPendingMap != nil {
		delete(state.macroPendingMap, button)
	}
	if state.macroPending != nil && state.macroPending.Button == button {
		state.macroPending = pickMacroDraft(state.macroPendingMap)
	}
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
	state.macroPendingMap = make(map[uint8]*MacroDraft)
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
	state.macroPendingMap = make(map[uint8]*MacroDraft)
	state.revision++
	state.err = Error{}
	return remapSnapshotLocked(state)
}

func (s *Service) GetMacroAssignmentSnapshot() RemapSnapshot { return s.GetRemapSnapshot() }

func (s *Service) macroDraftCurrent(d *MacroDraft) bool {
	m, err := s.ReadMacro(d.ID)
	return err == nil && m.Name == d.Name && reflect.DeepEqual(m.Events, d.Events)
}

// prevalidateMacroBound performs local admission before Apply changes visible
// draft/revision/retry/progress state. It never invokes a command or device I/O.
// Apply repeats freshness checks at dispatch and checks completion for races.
func (c *remapComponent) prevalidateMacroBound(binding Binding, pending x6.RemapConfig, drafts []*MacroDraft) ErrorCode {
	s := c.service
	if !s.bindingCurrent(binding) {
		return StaleBinding
	}
	buttons := make([]uint8, len(drafts))
	seen := make(map[uint8]bool, len(drafts))
	legacy := true
	for i, draft := range drafts {
		if draft == nil || seen[draft.Button] {
			return InvalidConfiguration
		}
		seen[draft.Button] = true
		buttons[i] = draft.Button
		destination, err := protocol.MacroDestinationForButton(draft.Button)
		if err != nil || !s.macroDraftCurrent(draft) {
			return InvalidConfiguration
		}
		if _, err := macros.EncodeX6LocalUpload(destination, draft.Events, draft.Repeat, draft.Mode); err != nil {
			return InvalidConfiguration
		}
		_, valid := admittedSequenceWithMode(draft.Events, draft.Repeat, draft.Mode)
		legacy = legacy && valid
	}
	if _, err := protocol.EncodeMultiMacroAssignmentReport(pending, buttons); err != nil {
		return InvalidConfiguration
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return StaleBinding
	}
	if (legacy && !inventory.MacroSequenceAssignmentsAvailable()) || (!legacy && !inventory.MacroLocalAssignmentsAvailable()) {
		return ApplyFailed
	}
	return ""
}

// The remap apply guards are held. The immutable copy is what reaches transport,
// even if the library changes while I/O is in flight. Completion checks reject
// stale drafts and retain the observed progress instead of claiming full apply.
func (c *remapComponent) applyMacroBound(binding Binding, state *remapState, revision uint64, pending x6.RemapConfig, drafts []*MacroDraft) {
	s := c.service
	if !s.bindingCurrent(binding) {
		failRemap(state, StaleBinding)
		return
	}
	for _, draft := range drafts {
		if !s.macroDraftCurrent(draft) {
			failRemap(state, InvalidConfiguration)
			return
		}
		if _, err := macros.AdmitX6LocalBlock(draft.Events, draft.Repeat, draft.Mode); err != nil {
			failRemap(state, InvalidConfiguration)
			return
		}
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		failRemap(state, StaleBinding)
		return
	}
	items := make([]mouse.MacroSequenceItem, len(drafts))
	localItems := make([]mouse.MacroLocalItem, len(drafts))
	legacy := true
	for i, draft := range drafts {
		sequence, valid := admittedSequenceWithMode(draft.Events, draft.Repeat, draft.Mode)
		legacy = legacy && valid
		localItems[i] = mouse.MacroLocalItem{Button: draft.Button, Events: append([]macros.Event(nil), draft.Events...), Repeat: draft.Repeat, Mode: draft.Mode}
		items[i] = mouse.MacroSequenceItem{
			Button:   draft.Button,
			Sequence: sequence,
		}
	}
	var progress mouse.MacroProgress
	var err error
	if legacy {
		progress, err = inventory.ApplyMultiMacroSequenceAssignmentBound(context.Background(), binding, pending, items)
	} else {
		progress, err = inventory.ApplyMultiMacroLocalAssignmentBound(context.Background(), binding, pending, localItems)
	}
	libraryCurrent := true
	for _, draft := range drafts {
		if !s.macroDraftCurrent(draft) {
			libraryCurrent = false
			break
		}
	}
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
		state.macroAppliedMap = make(map[uint8]*MacroDraft, len(drafts))
		for _, draft := range drafts {
			state.macroAppliedMap[draft.Button] = cloneMacroDraft(draft)
		}
		if state.macroPending != nil && state.macroAppliedMap[state.macroPending.Button] != nil {
			state.macroApplied = cloneMacroDraft(state.macroAppliedMap[state.macroPending.Button])
		} else if len(drafts) > 0 {
			state.macroApplied = cloneMacroDraft(drafts[0])
		} else {
			state.macroApplied = nil
		}
		state.firmware, state.err = "success", Error{}
		// Ordinary config persistence cannot represent this macro overlay. Do not
		// persist an ordinary config as though it described the uploaded assignment.
		state.persistence = "not_supported"
		if s.profiles != nil && s.profiles.store != nil && !binding.SessionOnly {
			macroDrafts := make([]MacroDraft, len(drafts))
			for i, d := range drafts {
				macroDrafts[i] = *cloneMacroDraft(d)
			}
			remapPending := cloneRemapConfig(pending)
			if err := s.saveActiveProfile(binding, func(p *profileConfiguration) {
				p.Device.Remap = &remapPending
				p.Macros = macroDrafts
			}); err != nil {
				retry := cloneRemapConfig(pending)
				state.retry, state.persistence, state.err = &retry, "failed", Error{Code: PersistenceFailed}
				state.mu.Unlock()
				s.emitRemapConfiguration(binding, remapSnapshotOf(state))
				return
			}
			state.persistence = "success"
		}
	}
	state.mu.Unlock()
	s.emitRemapConfiguration(binding, remapSnapshotOf(state))
}
