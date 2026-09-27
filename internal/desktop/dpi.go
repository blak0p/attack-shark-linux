package desktop

import (
	"context"
	"log/slog"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

// dpiComponent owns the DPI state transitions; Service remains the Wails facade.
// Service.mu guards the state map and coordinator pointer. State.mu guards each
// device's DPI data; never hold Service.mu while querying inventory selection.
type dpiComponent struct {
	service *Service
	legacy  *deviceState
	states  map[DeviceID]*deviceState
	sync    *SyncCoordinator
}

func (c *dpiComponent) attachSync(scheduler SyncScheduler) {
	c.sync = NewSyncCoordinator(scheduler, c.service.bindingCurrent, c.applyBound)
}

func (c *dpiComponent) cancelSync(binding Binding) {
	c.service.mu.Lock()
	coordinator := c.sync
	c.service.mu.Unlock()
	if coordinator != nil {
		coordinator.Cancel(binding)
	}
}

func (c *dpiComponent) newStateFromLegacy() *deviceState {
	c.legacy.mu.Lock()
	defer c.legacy.mu.Unlock()
	return newDeviceState(c.legacy.applied, c.legacy.factory)
}

func (c *dpiComponent) currentState() *deviceState {
	s := c.service
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return c.legacy
	}
	selected, ok := (selectionResolver{s}).selected()
	if !ok {
		return c.legacy
	}
	s.mu.Lock()
	state := c.states[selected.ID]
	if state == nil {
		state = c.newStateFromLegacy()
		c.states[selected.ID] = state
	}
	s.mu.Unlock()
	return state
}

func (c *dpiComponent) stage(config DPIConfig) Snapshot {
	state := c.service.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	next := fromDTO(config)
	if _, err := x6.EncodeDPIReport(next); err != nil {
		state.err = Error{Code: InvalidConfiguration}
		return snapshotLocked(state)
	}
	state.pending = next
	state.revision++
	state.err = Error{}
	state.firmware, state.persistence, state.retry = "pending", "", nil
	return snapshotLocked(state)
}

func (c *dpiComponent) retryPersistence() Snapshot {
	s := c.service
	binding, ok := s.selectedBinding()
	if !ok || binding.SessionOnly {
		return s.GetSnapshot()
	}
	state := s.currentState()
	state.mu.Lock()
	retry := state.retry
	state.mu.Unlock()
	if retry == nil {
		return s.GetSnapshot()
	}
	s.mu.Lock()
	persistence := s.devicePersistence
	s.mu.Unlock()
	if persistence == nil || persistence.Save(binding, *retry) != nil {
		state.mu.Lock()
		state.persistence = "failed"
		state.err = Error{Code: PersistenceFailed}
		state.mu.Unlock()
		return s.GetSnapshot()
	}
	state.mu.Lock()
	state.persistence, state.retry, state.err = "success", nil, Error{}
	state.mu.Unlock()
	return s.GetSnapshot()
}

func (c *dpiComponent) apply(ctx context.Context) Snapshot {
	s := c.service
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	state := s.currentState()
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	state.mu.Lock()
	pending := state.pending
	state.mu.Unlock()
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory != nil {
		if err := inventory.Stage(pending); err != nil {
			return c.applyFailure(err)
		}
		if err := inventory.Apply(ctx); err != nil {
			return c.applyFailure(err)
		}
		if binding, ok := inventory.Selection(); ok {
			s.mu.Lock()
			persistence := s.devicePersistence
			s.mu.Unlock()
			if !binding.SessionOnly && persistence != nil {
				if err := persistence.Save(binding, pending); err != nil {
					state.mu.Lock()
					retry := pending
					state.applied, state.firmware, state.persistence, state.retry, state.err = pending, "success", "failed", &retry, Error{Code: PersistenceFailed}
					snapshot := snapshotLocked(state)
					state.mu.Unlock()
					return snapshot
				}
			}
			s.cancelSync(binding)
		}
	} else if err := s.writer.ApplyAndPersist(ctx, pending, s.store); err != nil {
		return c.applyFailure(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.applied = pending
	state.firmware, state.persistence, state.err = "success", "success", Error{}
	return snapshotLocked(state)
}

func (c *dpiComponent) applyFailure(err error) Snapshot {
	slog.Error("apply DPI failed", "error", err, "classification", applyErrorClassification(err))
	state := c.service.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.err = Error{Code: errorCode(err, false)}
	return snapshotLocked(state)
}

func (c *dpiComponent) applyBound(binding Binding, revision uint64, config x6.DPIConfig) error {
	s := c.service
	if !s.bindingCurrent(binding) {
		return mouse.ErrStaleBinding
	}
	state := s.currentState()
	defer s.emitConfiguration(binding, state)
	state.mu.Lock()
	if state.revision != revision {
		state.mu.Unlock()
		return mouse.ErrRevisionChanged
	}
	state.mu.Unlock()
	s.mu.Lock()
	inventory, persistence := s.inventory, s.devicePersistence
	s.mu.Unlock()
	if err := inventory.ApplyBound(context.Background(), binding, config); err != nil {
		state.mu.Lock()
		state.firmware, state.err = "failed", Error{Code: errorCode(err, false)}
		state.mu.Unlock()
		return err
	}
	state.mu.Lock()
	if state.revision != revision {
		state.mu.Unlock()
		return mouse.ErrRevisionChanged
	}
	state.applied, state.firmware, state.err = config, "success", Error{}
	state.mu.Unlock()
	if !pollingPersistenceAllowed(binding) || persistence == nil {
		return nil
	}
	if err := persistence.Save(binding, config); err != nil {
		state.mu.Lock()
		state.persistence, state.retry, state.err = "failed", &config, Error{Code: PersistenceFailed}
		state.mu.Unlock()
		return nil
	}
	state.mu.Lock()
	state.persistence = "success"
	state.mu.Unlock()
	return nil
}
