package desktop

import (
	"context"
	"errors"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type remapState struct {
	mu                        sync.Mutex
	applyMu                   sync.Mutex
	pending, applied, factory x6.RemapConfig
	retry                     *x6.RemapConfig
	revision                  uint64
	firmware, persistence     string
	err                       Error
}

type remapComponent struct {
	service *Service
	states  map[DeviceID]*remapState
	legacy  *remapState
}

func newRemapState(applied, factory x6.RemapConfig) *remapState {
	return &remapState{applied: cloneRemapConfig(applied), pending: cloneRemapConfig(applied), factory: cloneRemapConfig(factory)}
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
		factory := x6.DefaultRemapConfig()
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

func (c *remapComponent) apply(config x6.RemapConfig) RemapSnapshot {
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
	if err := x6.NewRemapOperation().Validate(config); err != nil {
		return failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	if remapConfigsEqual(state.applied, config) && state.firmware == "success" {
		retry := state.retry != nil
		snapshot := remapSnapshotLocked(state)
		state.mu.Unlock()
		if retry {
			return c.retryBound(binding, state)
		}
		return snapshot
	}
	state.pending = cloneRemapConfig(config)
	state.revision++
	revision := state.revision
	state.firmware, state.persistence, state.retry, state.err = "pending", "", nil, Error{}
	state.mu.Unlock()
	c.applyBound(binding, state, revision, cloneRemapConfig(config))
	return remapSnapshotOf(state)
}

// The caller holds operationMu and state.applyMu. Inventory serializes the
// physical write; saveDeviceConfig serializes shared DeviceConfig mutations.
func (c *remapComponent) applyBound(binding Binding, state *remapState, revision uint64, pending x6.RemapConfig) {
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
	if err := inventory.ApplyOperationBound(context.Background(), binding, x6.NewRemapOperation(), pending); err != nil {
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
	state.mu.Unlock()
	if binding.SessionOnly || persistence == nil {
		s.emitRemapConfiguration(binding, remapSnapshotOf(state))
		return
	}
	if err := s.saveDeviceConfig(binding, persistence, func(config *x6.DeviceConfig) { config.Remap = &pending }); err != nil {
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
	if persistence == nil || binding.SessionOnly {
		return remapSnapshotOf(state)
	}
	if err := s.saveDeviceConfig(binding, persistence, func(config *x6.DeviceConfig) { config.Remap = &retry }); err != nil {
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
	defaults := x6.DefaultRemapConfig()
	state.pending, state.applied, state.retry = cloneRemapConfig(defaults), cloneRemapConfig(defaults), nil
	state.revision++
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

func remapSnapshotLocked(state *remapState) RemapSnapshot {
	actions := []x6.RemapAction{
		x6.RemapOff, x6.RemapLeft, x6.RemapRight, x6.RemapMiddle, x6.RemapForward, x6.RemapBackward, x6.RemapDoubleClick, x6.RemapFire,
		x6.RemapMediaPlayer, x6.RemapPlayPause, x6.RemapStop, x6.RemapPreviousTrack, x6.RemapNextTrack, x6.RemapVolumeUp, x6.RemapVolumeDown, x6.RemapMute,
		x6.RemapScrollUp, x6.RemapScrollDown, x6.RemapDPICycle, x6.RemapDPIPlus, x6.RemapDPIMinus,
		x6.RemapBrowserCalculator, x6.RemapBrowserEmail, x6.RemapBrowserForward, x6.RemapBrowserBackward, x6.RemapBrowserStop,
		x6.RemapBrowserMyComputer, x6.RemapBrowserRefresh, x6.RemapBrowserHome, x6.RemapBrowserSearch,
	}
	return RemapSnapshot{Pending: cloneRemapConfig(state.pending), Applied: cloneRemapConfig(state.applied), Factory: cloneRemapConfig(state.factory), Actions: actions, Revision: state.revision, Firmware: state.firmware, Persistence: state.persistence, RetryAvailable: state.retry != nil, Error: state.err}
}

func cloneRemapConfig(config x6.RemapConfig) x6.RemapConfig {
	return x6.RemapConfig{Buttons: append([]x6.RemapButton(nil), config.Buttons...)}
}
func remapConfigsEqual(left, right x6.RemapConfig) bool {
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
