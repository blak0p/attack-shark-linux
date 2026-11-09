package desktop

import (
	"context"
	"errors"
	"github.com/blak0p/attack-shark-linux/internal/device"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

type remapState struct {
	macroPendingMap            map[uint8]*MacroDraft
	macroAppliedMap            map[uint8]*MacroDraft
	macroPending, macroApplied *MacroDraft
	macroProgress              mouse.MacroProgress
	mu                         sync.Mutex
	applyMu                    sync.Mutex
	pending, applied, factory  device.RemapConfig
	retry                      *device.RemapConfig
	revision                   uint64
	firmware, persistence      string
	err                        Error
}

type remapComponent struct {
	service *Service
	states  map[DeviceID]*remapState
	legacy  *remapState
}

func newRemapState(applied, factory device.RemapConfig) *remapState {
	return &remapState{
		applied:         cloneRemapConfig(applied),
		pending:         cloneRemapConfig(applied),
		factory:         cloneRemapConfig(factory),
		macroPendingMap: make(map[uint8]*MacroDraft),
		macroAppliedMap: make(map[uint8]*MacroDraft),
	}
}

func (c *remapComponent) currentState() *remapState {
	binding, ok := c.service.selectedBinding()
	if !ok {
		return c.legacy
	}
	return c.stateForBinding(binding)
}

func (c *remapComponent) stateForBinding(binding Binding) *remapState {
	s := c.service
	s.mu.Lock()
	defer s.mu.Unlock()
	state := c.states[binding.ID]
	if state == nil {
		factory := device.DefaultRemapConfig()
		applied := factory
		if s.remapPersistence != nil && !binding.SessionOnly {
			if persisted, err := s.remapPersistence.Load(binding); err == nil && persisted.Remap != nil {
				applied = cloneRemapConfig(*persisted.Remap)
			}
		}
		state = newRemapState(applied, factory)
		c.states[binding.ID] = state
	}
	return state
}

func (c *remapComponent) snapshot() RemapSnapshot { return remapSnapshotOf(c.currentState()) }

func (c *remapComponent) apply(config device.RemapConfig) RemapSnapshot {
	// Freeze caller-owned data before waiting on either serialization lock.
	config = cloneRemapConfig(config)
	s := c.service
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(c.legacy, SelectionRequired)
	}
	// Keep this state paired with the captured binding, even if selection changes
	// while persistence is loaded or the inventory write is in flight.
	state := c.stateForBinding(binding)
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	if err := s.model.RemapOperation().Validate(config); err != nil {
		return failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	if len(state.macroPendingMap) == 0 && state.macroPending == nil && len(state.macroAppliedMap) == 0 && state.macroApplied == nil && remapConfigsEqual(state.applied, config) && state.firmware == "success" {
		retry := state.retry != nil
		snapshot := remapSnapshotLocked(state)
		state.mu.Unlock()
		if retry {
			return c.retryBound(binding, state)
		}
		return snapshot
	}
	observedRevision := state.revision
	var drafts []*MacroDraft
	for b := uint8(1); b <= 7; b++ {
		if d := state.macroPendingMap[b]; d != nil {
			drafts = append(drafts, cloneMacroDraft(d))
		}
	}
	if len(drafts) == 0 && state.macroPending != nil {
		drafts = append(drafts, cloneMacroDraft(state.macroPending))
	}
	state.mu.Unlock()
	if len(drafts) > 0 {
		if code := c.prevalidateMacroBound(binding, config, drafts); code != "" {
			return failRemap(state, code)
		}
	}
	// Staging is intentionally allowed during validation. Never overwrite a newer
	// draft or attribute this operation to a replacement selection.
	state.mu.Lock()
	if state.revision != observedRevision || !s.bindingCurrent(binding) {
		state.mu.Unlock()
		return failRemap(state, StaleBinding)
	}
	state.pending = cloneRemapConfig(config)
	state.revision++
	revision := state.revision
	state.firmware, state.persistence, state.retry, state.err = "pending", "", nil, Error{}
	state.mu.Unlock()
	if len(drafts) > 0 {
		c.applyMacroBound(binding, state, revision, cloneRemapConfig(config), drafts)
	} else {
		c.applyBound(binding, state, revision, cloneRemapConfig(config))
	}
	return remapSnapshotOf(state)
}

// The caller holds operationMu and state.applyMu. Inventory serializes the
// physical write; saveDeviceConfig serializes shared DeviceConfig mutations.
func (c *remapComponent) applyBound(binding Binding, state *remapState, revision uint64, pending device.RemapConfig) {
	s := c.service
	if !s.bindingCurrent(binding) {
		failRemap(state, StaleBinding)
		return
	}
	s.mu.Lock()
	inventory, persistence := s.inventory, s.remapPersistence
	s.mu.Unlock()
	if inventory == nil {
		failRemap(state, StaleBinding)
		return
	}
	if err := inventory.ApplyOperationBound(context.Background(), binding, s.model.RemapOperation(), pending); err != nil {
		if errors.Is(err, mouse.ErrStaleBinding) || errors.Is(err, mouse.ErrRevisionChanged) {
			failRemap(state, StaleBinding)
		} else {
			failRemap(state, errorCode(err, false))
		}
		return
	}
	state.mu.Lock()
	if state.revision != revision || !s.bindingCurrent(binding) {
		state.mu.Unlock()
		failRemap(state, StaleBinding)
		return
	}
	state.applied, state.firmware, state.err = cloneRemapConfig(pending), "success", Error{}
	state.macroApplied = nil
	state.macroAppliedMap = make(map[uint8]*MacroDraft)
	state.mu.Unlock()
	if binding.SessionOnly || (persistence == nil && (s.profiles == nil || s.profiles.store == nil)) {
		s.emitRemapConfiguration(binding, remapSnapshotOf(state))
		return
	}
	deviceErr := error(nil)
	if persistence != nil {
		deviceErr = s.saveDeviceConfig(binding, persistence, func(config *device.DeviceConfig) { config.Remap = &pending })
	}
	profileErr := s.saveActiveProfile(binding, func(p *profileConfiguration) {
		remapPending := cloneRemapConfig(pending)
		p.Device.Remap = &remapPending
		p.Macros = nil
	})
	if (persistence != nil && deviceErr != nil) || profileErr != nil {
		state.mu.Lock()
		retry := cloneRemapConfig(pending)
		state.retry, state.persistence, state.err = &retry, "failed", Error{Code: PersistenceFailed}
		state.mu.Unlock()
		s.emitRemapConfiguration(binding, remapSnapshotOf(state))
		return
	}
	state.mu.Lock()
	state.persistence = "success"
	state.mu.Unlock()
	s.emitRemapConfiguration(binding, remapSnapshotOf(state))
}

func (c *remapComponent) retryPersistence() RemapSnapshot {
	s := c.service
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, ok := s.selectedBinding()
	if !ok {
		return failRemap(c.legacy, SelectionRequired)
	}
	state := c.stateForBinding(binding)
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	return c.retryBound(binding, state)
}

// retryBound never writes firmware and is also used by an identical ApplyRemap.
func (c *remapComponent) retryBound(binding Binding, state *remapState) RemapSnapshot {
	s := c.service
	s.mu.Lock()
	persistence := s.remapPersistence
	s.mu.Unlock()
	state.mu.Lock()
	if state.retry == nil {
		snapshot := remapSnapshotLocked(state)
		state.mu.Unlock()
		return snapshot
	}
	retry := cloneRemapConfig(*state.retry)
	state.mu.Unlock()
	if binding.SessionOnly {
		return remapSnapshotOf(state)
	}
	deviceErr := error(nil)
	if persistence != nil {
		deviceErr = s.saveDeviceConfig(binding, persistence, func(config *device.DeviceConfig) { config.Remap = &retry })
	}
	profileErr := s.saveActiveProfile(binding, func(p *profileConfiguration) {
		remapRetry := cloneRemapConfig(retry)
		p.Device.Remap = &remapRetry
		state.mu.Lock()
		numMacros := len(state.macroAppliedMap)
		state.mu.Unlock()
		if numMacros > 0 {
			state.mu.Lock()
			var drafts []MacroDraft
			for b := uint8(1); b <= 7; b++ {
				if d := state.macroAppliedMap[b]; d != nil {
					drafts = append(drafts, *cloneMacroDraft(d))
				}
			}
			state.mu.Unlock()
			p.Macros = drafts
		} else {
			p.Macros = nil
		}
	})
	if (persistence != nil && deviceErr != nil) || profileErr != nil {
		return failRemap(state, PersistenceFailed)
	}
	state.mu.Lock()
	state.retry, state.persistence, state.err = nil, "success", Error{}
	state.mu.Unlock()
	return remapSnapshotOf(state)
}

// The reset orchestrator holds operationMu before reconciliation.
func (c *remapComponent) reconcileFactoryReset() {
	state := c.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	defaults := device.DefaultRemapConfig()
	state.pending, state.applied, state.retry = cloneRemapConfig(defaults), cloneRemapConfig(defaults), nil
	state.revision++
	state.macroPending, state.macroApplied, state.macroProgress = nil, nil, mouse.MacroProgress{}
	state.macroPendingMap = make(map[uint8]*MacroDraft)
	state.macroAppliedMap = make(map[uint8]*MacroDraft)
	state.firmware, state.persistence, state.err = "success", "success", Error{}
}

func failRemap(state *remapState, code ErrorCode) RemapSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.firmware, state.err = "failed", Error{Code: code}
	return remapSnapshotLocked(state)
}

func remapSnapshotOf(state *remapState) RemapSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return remapSnapshotLocked(state)
}

func pickMacroDraft(m map[uint8]*MacroDraft) *MacroDraft {
	for b := uint8(1); b <= 7; b++ {
		if d := m[b]; d != nil {
			return cloneMacroDraft(d)
		}
	}
	return nil
}

func remapSnapshotLocked(state *remapState) RemapSnapshot {
	actions := []device.RemapAction{
		device.RemapOff, device.RemapLeft, device.RemapRight, device.RemapMiddle, device.RemapForward, device.RemapBackward, device.RemapDoubleClick, device.RemapFire,
		device.RemapMediaPlayer, device.RemapPlayPause, device.RemapStop, device.RemapPreviousTrack, device.RemapNextTrack, device.RemapVolumeUp, device.RemapVolumeDown, device.RemapMute,
		device.RemapScrollUp, device.RemapScrollDown, device.RemapDPICycle, device.RemapDPIPlus, device.RemapDPIMinus,
		device.RemapBrowserCalculator, device.RemapBrowserEmail, device.RemapBrowserForward, device.RemapBrowserBackward, device.RemapBrowserStop,
		device.RemapBrowserMyComputer, device.RemapBrowserRefresh, device.RemapBrowserHome, device.RemapBrowserSearch,
	}
	drafts := make(map[uint8]MacroDraft, len(state.macroPendingMap))
	for b, d := range state.macroPendingMap {
		if d != nil {
			drafts[b] = *cloneMacroDraft(d)
		}
	}
	appliedDrafts := make(map[uint8]MacroDraft, len(state.macroAppliedMap))
	for b, d := range state.macroAppliedMap {
		if d != nil {
			appliedDrafts[b] = *cloneMacroDraft(d)
		}
	}
	pendingDraft := cloneMacroDraft(state.macroPending)
	if pendingDraft == nil && len(drafts) > 0 {
		pendingDraft = pickMacroDraft(state.macroPendingMap)
	}
	appliedDraft := cloneMacroDraft(state.macroApplied)
	if appliedDraft == nil && len(appliedDrafts) > 0 {
		appliedDraft = pickMacroDraft(state.macroAppliedMap)
	}
	return RemapSnapshot{
		MacroPending:       pendingDraft,
		MacroApplied:       appliedDraft,
		MacroDrafts:        drafts,
		MacroAppliedDrafts: appliedDrafts,
		MacroProgress:      state.macroProgress,
		Pending:            cloneRemapConfig(state.pending),
		Applied:            cloneRemapConfig(state.applied),
		Factory:            cloneRemapConfig(state.factory),
		Actions:            actions,
		Revision:           state.revision,
		Firmware:           state.firmware,
		Persistence:        state.persistence,
		RetryAvailable:     state.retry != nil,
		Error:              state.err,
	}
}

func cloneRemapConfig(config device.RemapConfig) device.RemapConfig {
	return device.RemapConfig{Buttons: append([]device.RemapButton(nil), config.Buttons...)}
}
func remapConfigsEqual(left, right device.RemapConfig) bool {
	if len(left.Buttons) != len(right.Buttons) {
		return false
	}
	for index := range left.Buttons {
		if left.Buttons[index] != right.Buttons[index] {
			return false
		}
	}
	return true
}
