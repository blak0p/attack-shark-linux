package desktop

import (
	"context"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

// inventoryComponent owns discovered bindings and the per-device state created
// at inventory boundaries. Service.mu protects these fields; never hold it
// while querying TargetedService or cancelling scheduled writes.
type inventoryComponent struct {
	inventory          *mouse.TargetedService
	inventoryDevices   []Device
	migrate            func(Binding) error
	devicePersistence  DevicePersistence
	states             map[DeviceID]*deviceState
	sync               *SyncCoordinator
	pollingStates      map[DeviceID]*pollingState
	pollingSync        *PollingSyncCoordinator
	pollingPersistence PollingPersistence
	settingsStates     map[DeviceID]*settingsState
}

func newInventoryComponent() *inventoryComponent {
	return &inventoryComponent{
		states:         make(map[DeviceID]*deviceState),
		pollingStates:  make(map[DeviceID]*pollingState),
		settingsStates: make(map[DeviceID]*settingsState),
	}
}

// attach is called with Service.mu held.
func (c *inventoryComponent) attach(inventory *mouse.TargetedService, s *Service) {
	c.inventory = inventory
	c.inventoryDevices = nil
	c.sync = NewSyncCoordinator(realSyncScheduler{}, s.bindingCurrent, s.applyBound)
	c.pollingSync = NewPollingSyncCoordinator(realSyncScheduler{}, s.bindingCurrent, s.applyPollingBound)
}

func (c *inventoryComponent) refresh(ctx context.Context, s *Service) Inventory {
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return Inventory{Error: Error{Code: DeviceUnavailable}}
	}
	if selected, ok := inventory.Selection(); ok {
		s.cancelSync(selected)
		s.cancelPollingSync(selected)
	}
	devices, err := inventory.Refresh(ctx)
	if err != nil {
		if err == mouse.ErrAmbiguousIdentity {
			return Inventory{Devices: devices, Error: Error{Code: AmbiguousIdentity}}
		}
		return Inventory{Error: Error{Code: DeviceUnavailable}}
	}
	result := Inventory{Devices: devices}
	for _, device := range devices {
		if device.Warning == "ambiguous identity" {
			result.Error = Error{Code: AmbiguousIdentity}
			break
		}
	}
	if selected, ok := inventory.Selection(); ok {
		result.Selected = &selected
		s.mu.Lock()
		migrate := s.migrate
		s.mu.Unlock()
		if !selected.SessionOnly && migrate != nil && migrate(selected) != nil {
			result.Error = Error{Code: MigrationFailed}
		}
	}
	s.mu.Lock()
	s.inventoryDevices = devices
	for _, device := range devices {
		if _, ok := s.states[device.ID]; !ok {
			state := s.newStateFromLegacy()
			if result.Selected != nil && result.Selected.SessionOnly && result.Selected.ID == device.ID {
				state = newDeviceState(x6.DefaultDPIConfig(), x6.DefaultDPIConfig())
			}
			if result.Selected != nil && !result.Selected.SessionOnly && result.Selected.ID == device.ID && s.devicePersistence != nil {
				if applied, err := s.devicePersistence.Load(*result.Selected); err == nil {
					state = newDeviceState(applied, state.factory)
				}
			}
			polling := newPollingState()
			if result.Selected != nil && !result.Selected.SessionOnly && result.Selected.ID == device.ID && s.pollingPersistence != nil {
				if config, err := s.pollingPersistence.Load(*result.Selected); err == nil {
					polling = newPollingStateFromConfig(config)
				}
			}
			s.pollingStates[device.ID] = polling
			settings := newSettingsState()
			if result.Selected != nil && !result.Selected.SessionOnly && result.Selected.ID == device.ID && s.settingsPersistence != nil {
				if config, err := s.settingsPersistence.Load(*result.Selected); err == nil {
					settings = newSettingsStateFromConfig(config)
				}
			}
			s.settingsStates[device.ID] = settings
			s.states[device.ID] = state
		}
	}
	if result.Selected != nil {
		if state := s.states[result.Selected.ID]; state != nil {
			state.mu.Lock()
			state.observedStage, state.observedDPI = nil, nil
			state.mu.Unlock()
		}
	}
	s.mu.Unlock()
	return result
}

func (c *inventoryComponent) selectDevice(id DeviceID, s *Service) Inventory {
	s.mu.Lock()
	inventory := s.inventory
	devices := append([]Device(nil), s.inventoryDevices...)
	migrate := s.migrate
	s.mu.Unlock()
	if inventory != nil {
		if previous, ok := inventory.Selection(); ok {
			s.cancelSync(previous)
			s.cancelPollingSync(previous)
		}
	}
	if inventory == nil || inventory.Select(id) != nil {
		return Inventory{Devices: devices, Error: Error{Code: SelectionRequired}}
	}
	selected, _ := inventory.Selection()
	s.mu.Lock()
	persistence := s.devicePersistence
	state := s.states[selected.ID]
	s.mu.Unlock()
	if !selected.SessionOnly && persistence != nil && state != nil {
		if applied, err := persistence.Load(selected); err == nil {
			state.mu.Lock()
			state.applied, state.pending = applied, applied
			state.mu.Unlock()
		}
	}
	if !selected.SessionOnly {
		s.mu.Lock()
		pollingPersistence, polling := s.pollingPersistence, s.pollingStates[selected.ID]
		s.mu.Unlock()
		if pollingPersistence != nil && polling != nil {
			if config, err := pollingPersistence.Load(selected); err == nil {
				next := newPollingStateFromConfig(config)
				polling.mu.Lock()
				polling.desired, polling.applied, polling.factory = next.desired, next.applied, next.factory
				polling.persisted, polling.retry, polling.revision = next.persisted, next.retry, next.revision
				polling.firmware, polling.persistence = next.firmware, next.persistence
				polling.mu.Unlock()
			}
		}
	}
	if !selected.SessionOnly && migrate != nil && migrate(selected) != nil {
		return Inventory{Devices: devices, Selected: &selected, Error: Error{Code: MigrationFailed}}
	}
	return Inventory{Devices: devices, Selected: &selected}
}

func (c *inventoryComponent) cancelSync(binding Binding, s *Service) {
	s.mu.Lock()
	sync := s.sync
	s.mu.Unlock()
	if sync != nil {
		sync.Cancel(binding)
	}
}

func (c *inventoryComponent) cancelPollingSync(binding Binding, s *Service) {
	s.mu.Lock()
	sync := s.pollingSync
	s.mu.Unlock()
	if sync != nil {
		sync.Cancel(binding)
	}
}

func (c *inventoryComponent) newStateFromLegacy(s *Service) *deviceState {
	s.legacy.mu.Lock()
	defer s.legacy.mu.Unlock()
	return newDeviceState(s.legacy.applied, s.legacy.factory)
}
