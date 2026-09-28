package desktop

import (
	"context"
	"errors"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type lightingComponent struct {
	service *Service
	states  map[DeviceID]*lightingState
	// These test seams force selection changes around settings' lighting lookup.
	beforeSettingsLightingRead func()
	afterSettingsLightingRead  func()
}

func (c *lightingComponent) currentState() *lightingState {
	binding, ok := c.service.selectedBinding()
	if !ok {
		return newLightingState()
	}
	return c.stateForBinding(binding)
}

func (c *lightingComponent) stateForBinding(binding Binding) *lightingState {
	c.service.mu.Lock()
	defer c.service.mu.Unlock()
	state := c.states[binding.ID]
	if state == nil {
		state = newLightingState()
		c.states[binding.ID] = state
	}
	return state
}

func (c *lightingComponent) snapshot() LightingSnapshot {
	return lightingSnapshotOf(c.currentState())
}

func (c *lightingComponent) stage(selection x6.LightingSelection) LightingSnapshot {
	state := c.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := x6.NewLightingOperation().Validate(selection); err != nil {
		state.err = Error{Code: InvalidConfiguration}
		return lightingSnapshotLocked(state)
	}
	state.pending = selection
	state.revision++
	state.firmware = "pending"
	state.err = Error{}
	return lightingSnapshotLocked(state)
}

func (c *lightingComponent) apply() LightingSnapshot {
	s := c.service
	binding, ok := s.selectedBinding()
	if !ok {
		return failLighting(newLightingState(), SelectionRequired)
	}
	state := c.stateForBinding(binding)
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	state.mu.Lock()
	selection, revision := state.pending, state.revision
	state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		return failLighting(state, StaleBinding)
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return failLighting(state, StaleBinding)
	}
	settingsState := s.settingsStateForBinding(binding)
	settingsState.applyMu.Lock()
	defer settingsState.applyMu.Unlock()
	settingsState.mu.Lock()
	settings := x6.LightingSettings{LightingSelection: selection, NormalSleepMinutes: settingsState.normalSleep, ResponseTimeMs: settingsState.responseTimeMs}
	settingsState.mu.Unlock()
	if !s.bindingCurrent(binding) {
		return failLighting(state, StaleBinding)
	}
	if err := inventory.ApplyOperationBound(context.Background(), binding, x6.NewLightingSettingsOperation(), settings); err != nil {
		if errors.Is(err, mouse.ErrStaleBinding) {
			return failLighting(state, StaleBinding)
		}
		return failLighting(state, errorCode(err, false))
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.revision != revision {
		state.firmware = "failed"
		state.err = Error{Code: ApplyFailed}
		return lightingSnapshotLocked(state)
	}
	applied := selection
	state.applied = &applied
	state.firmware = "success"
	state.err = Error{}
	return lightingSnapshotLocked(state)
}

func failLighting(state *lightingState, code ErrorCode) LightingSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.firmware = "failed"
	state.err = Error{Code: code}
	return lightingSnapshotLocked(state)
}

func newLightingState() *lightingState {
	return &lightingState{pending: x6.LightingSelection{Mode: x6.LightingFixed, TemplateID: x6.LightingTemplateFixedGreen}}
}

func lightingSnapshotOf(state *lightingState) LightingSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return lightingSnapshotLocked(state)
}

func lightingSnapshotLocked(state *lightingState) LightingSnapshot {
	var applied *x6.LightingSelection
	if state.applied != nil {
		copy := *state.applied
		applied = &copy
	}
	return LightingSnapshot{Pending: state.pending, Applied: applied, Effects: x6.LightingEffects(), Revision: state.revision, Firmware: state.firmware, Error: state.err}
}
