package desktop

import (
	"context"
	"errors"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type pollingState struct {
	mu, applyMu               sync.Mutex
	desired, applied, factory x6.PollingRate
	persisted, retry          *x6.PollingRate
	revision                  uint64
	err                       Error
	firmware, persistence     string
}

// pollingComponent owns polling state and its serialized selected-device writes.
// Service.mu protects its map, persistence adapter and coordinator.
type pollingComponent struct {
	service     *Service
	states      map[DeviceID]*pollingState
	persistence PollingPersistence
	sync        *PollingSyncCoordinator
}

func (c *pollingComponent) attachSync(scheduler SyncScheduler) {
	c.sync = NewPollingSyncCoordinator(scheduler, c.service.bindingCurrent, c.applyPollingBound)
}
func (c *pollingComponent) cancelSync(binding Binding) {
	c.service.mu.Lock()
	coordinator := c.sync
	c.service.mu.Unlock()
	if coordinator != nil {
		coordinator.Cancel(binding)
	}
}
func (c *pollingComponent) initDevice(id DeviceID, selected *Binding) {
	state := newPollingState()
	if selected != nil && !selected.SessionOnly && selected.ID == id && c.persistence != nil {
		if config, err := c.persistence.Load(*selected); err == nil {
			state = newPollingStateFromConfig(config)
		}
	}
	c.states[id] = state // caller holds Service.mu
}
func (c *pollingComponent) selectDevice(selected Binding) {
	if selected.SessionOnly {
		return
	}
	s := c.service
	s.mu.Lock()
	persistence, state := c.persistence, c.states[selected.ID]
	s.mu.Unlock()
	if persistence == nil || state == nil {
		return
	}
	if config, err := persistence.Load(selected); err == nil {
		next := newPollingStateFromConfig(config)
		state.mu.Lock()
		state.desired, state.applied, state.factory = next.desired, next.applied, next.factory
		state.persisted, state.retry, state.revision = next.persisted, next.retry, next.revision
		state.firmware, state.persistence = next.firmware, next.persistence
		state.mu.Unlock()
	}
}
func (c *pollingComponent) currentState() *pollingState {
	s := c.service
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return newPollingState()
	}
	selected, ok := (selectionResolver{s}).selected()
	if !ok {
		return newPollingState()
	}
	s.mu.Lock()
	state := c.states[selected.ID]
	if state == nil {
		state = newPollingState()
		c.states[selected.ID] = state
	}
	s.mu.Unlock()
	return state
}
func (c *pollingComponent) snapshot() PollingSnapshot { return pollingSnapshotOf(c.currentState()) }
func (c *pollingComponent) stage(rate x6.PollingRate) PollingSnapshot {
	state := c.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if err := x6.NewPollingOperation().Validate(rate); err != nil {
		return pollingSnapshotLocked(state)
	}
	state.desired = rate
	state.revision++
	state.firmware, state.persistence, state.retry = "pending", "", nil
	return pollingSnapshotLocked(state)
}
func (c *pollingComponent) retryPersistence() PollingSnapshot {
	s := c.service
	binding, ok := s.selectedBinding()
	if !ok || binding.SessionOnly {
		return c.snapshot()
	}
	state := c.currentState()
	state.mu.Lock()
	retry := state.retry
	state.mu.Unlock()
	if retry == nil {
		return c.snapshot()
	}
	s.mu.Lock()
	persistence := c.persistence
	s.mu.Unlock()
	if persistence == nil || persistence.Save(binding, x6.DeviceConfig{PollingRate: *retry}) != nil {
		state.mu.Lock()
		state.persistence = "failed"
		state.mu.Unlock()
		snapshot := pollingSnapshotOf(state)
		c.emitConfiguration(binding, snapshot)
		return snapshot
	}
	state.mu.Lock()
	state.persisted, state.retry, state.persistence = retry, nil, "success"
	state.mu.Unlock()
	snapshot := pollingSnapshotOf(state)
	c.emitConfiguration(binding, snapshot)
	return snapshot
}
func (c *pollingComponent) apply(ctx context.Context) PollingSnapshot {
	s := c.service
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, ok := s.selectedBinding()
	state := c.currentState()
	if !ok {
		return failPolling(state, SelectionRequired)
	}
	state.mu.Lock()
	revision, rate := state.revision, state.desired
	state.mu.Unlock()
	if err := c.applyBound(ctx, binding, revision, rate); err != nil {
		if errors.Is(err, mouse.ErrStaleBinding) {
			return failPolling(state, SelectionRequired)
		}
	}
	return pollingSnapshotOf(state)
}
func (c *pollingComponent) applyPollingBound(binding Binding, revision uint64, rate x6.PollingRate) error {
	return c.applyBound(context.Background(), binding, revision, rate)
}
func (c *pollingComponent) applyBound(ctx context.Context, binding Binding, revision uint64, rate x6.PollingRate) error {
	s := c.service
	if !s.bindingCurrent(binding) {
		return mouse.ErrStaleBinding
	}
	state := c.currentState()
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	completed := false
	defer func() {
		if completed {
			c.emitConfiguration(binding, pollingSnapshotOf(state))
		}
	}()
	state.mu.Lock()
	if state.revision != revision {
		state.mu.Unlock()
		return mouse.ErrRevisionChanged
	}
	state.mu.Unlock()
	s.mu.Lock()
	inventory, persistence := s.inventory, c.persistence
	s.mu.Unlock()
	if inventory == nil {
		return mouse.ErrStaleBinding
	}
	if err := inventory.ApplyOperationBound(ctx, binding, x6.NewPollingOperation(), rate); err != nil {
		state.mu.Lock()
		state.firmware, state.err = "failed", Error{Code: errorCode(err, false)}
		state.mu.Unlock()
		completed = true
		return err
	}
	state.mu.Lock()
	if state.revision != revision {
		state.mu.Unlock()
		return mouse.ErrRevisionChanged
	}
	state.applied, state.firmware = rate, "success"
	state.mu.Unlock()
	if binding.SessionOnly || persistence == nil {
		completed = true
		return nil
	}
	if err := persistence.Save(binding, x6.DeviceConfig{PollingRate: rate}); err != nil {
		state.mu.Lock()
		state.retry, state.persistence = &rate, "failed"
		state.mu.Unlock()
		completed = true
		return nil
	}
	state.mu.Lock()
	state.persisted, state.persistence = &rate, "success"
	state.mu.Unlock()
	completed = true
	return nil
}
func (c *pollingComponent) emitConfiguration(binding Binding, snapshot PollingSnapshot) {
	c.service.listenerComponent.emit(c.service, "mouse:polling-configuration", PollingConfigurationEvent{Binding: binding, Snapshot: snapshot})
}
func (c *pollingComponent) reconcileFactoryReset() {
	state := c.currentState()
	state.mu.Lock()
	state.desired, state.applied, state.persisted, state.retry = x6.PollingRate1000, x6.PollingRate1000, nil, nil
	state.revision++
	state.firmware, state.persistence = "success", "success"
	state.mu.Unlock()
}
func pollingPersistenceAllowed(binding Binding) bool { return !binding.SessionOnly }
func newPollingState() *pollingState {
	return &pollingState{desired: x6.PollingRate1000, applied: x6.PollingRate1000, factory: x6.PollingRate1000}
}
func newPollingStateFromConfig(config x6.DeviceConfig) *pollingState {
	return &pollingState{desired: config.PollingRate, applied: config.PollingRate, persisted: &config.PollingRate, factory: x6.PollingRate1000}
}
func pollingSnapshotOf(state *pollingState) PollingSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return pollingSnapshotLocked(state)
}
func pollingSnapshotLocked(state *pollingState) PollingSnapshot {
	return PollingSnapshot{Desired: state.desired, Applied: state.applied, Persisted: state.persisted, Factory: state.factory, Revision: state.revision, Error: state.err, Firmware: state.firmware, Persistence: state.persistence, RetryAvailable: state.retry != nil}
}
func failPolling(state *pollingState, code ErrorCode) PollingSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.err = Error{Code: code}
	return pollingSnapshotLocked(state)
}
