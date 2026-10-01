package desktop

import (
	"context"
	"errors"
	"os"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

// inventoryComponent owns discovered bindings and the per-device state created
// at inventory boundaries. Service.mu protects these fields; never hold it
// while querying TargetedService or cancelling scheduled writes.
type inventoryComponent struct {
	inventory         *mouse.TargetedService
	inventoryDevices  []Device
	migrate           func(Binding) error
	devicePersistence DevicePersistence
	pollingStates     map[DeviceID]*pollingState // compatibility view; pollingComponent owns entries
	pollingSync       *PollingSyncCoordinator    // reset orchestration uses this coordinator
	settingsStates    map[DeviceID]*settingsState
}

func newInventoryComponent() *inventoryComponent {
	return &inventoryComponent{
		pollingStates:  make(map[DeviceID]*pollingState),
		settingsStates: make(map[DeviceID]*settingsState),
	}
}

// attach is called with Service.mu held.
func (c *inventoryComponent) attach(inventory *mouse.TargetedService, s *Service) {
	c.inventory = inventory
	c.inventoryDevices = nil
	s.dpiComponent.attachSync(realSyncScheduler{})
	s.pollingComponent.attachSync(realSyncScheduler{})
	c.pollingSync = s.pollingComponent.sync
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
			s.pollingComponent.initDevice(device.ID, result.Selected)
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
	s.pollingComponent.selectDevice(selected)
	if !selected.SessionOnly {
		s.mu.Lock()
		settingsPersistence, settings := s.settingsPersistence, s.settingsStates[selected.ID]
		s.mu.Unlock()
		if settingsPersistence != nil && settings != nil {
			settings.applyMu.Lock()
			if config, err := settingsPersistence.Load(selected); err == nil {
				settings.replace(config)
			} else if errors.Is(err, os.ErrNotExist) {
				settings.reset()
			}
			settings.applyMu.Unlock()
		}
	}
	if !selected.SessionOnly && migrate != nil && migrate(selected) != nil {
		return Inventory{Devices: devices, Selected: &selected, Error: Error{Code: MigrationFailed}}
	}
	return Inventory{Devices: devices, Selected: &selected}
}
