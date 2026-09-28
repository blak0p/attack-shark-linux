package desktop

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/hidlinux"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type ErrorCode string

const (
	DeviceUnavailable    ErrorCode = "device_unavailable"
	DeviceDisconnected   ErrorCode = "device_disconnected"
	PermissionDenied     ErrorCode = "permission_denied"
	StatusReadFailed     ErrorCode = "status_read_failed"
	InvalidConfiguration ErrorCode = "invalid_configuration"
	ApplyFailed          ErrorCode = "apply_failed"
	PersistenceFailed    ErrorCode = "persistence_failed"
	SelectionRequired    ErrorCode = "selection_required"
	StaleBinding         ErrorCode = "stale_binding"
	AmbiguousIdentity    ErrorCode = "ambiguous_identity"
	MigrationFailed      ErrorCode = "migration_failed"
)

type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}
type Error struct{ Code ErrorCode }
type DeviceID = mouse.DeviceID
type Device = mouse.Device
type Binding = mouse.Binding
type Inventory struct {
	Devices  []Device
	Selected *Binding
	Error    Error
}
type Snapshot struct {
	Connection     string
	Battery        *int
	Applied        DPIConfig
	Pending        DPIConfig
	Factory        DPIConfig
	Revision       uint64
	Error          Error
	Firmware       string
	Persistence    string
	RetryAvailable bool
	ObservedStage  *int
	ObservedDPI    *int
}
type PollingSnapshot struct {
	Desired, Applied x6.PollingRate
	Persisted        *x6.PollingRate
	Factory          x6.PollingRate
	Revision         uint64
	Error            Error
	Firmware         string
	Persistence      string
	RetryAvailable   bool
}
type DebounceSnapshot struct {
	Desired, Applied      int
	Persisted             *int
	Factory               int
	Revision              uint64
	Error                 Error
	Firmware, Persistence string
	RetryAvailable        bool
}
type LightingSnapshot struct {
	Pending  x6.LightingSelection
	Applied  *x6.LightingSelection
	Effects  []x6.LightingEffect
	Revision uint64
	Firmware string
	Error    Error
}
type NormalSleepSnapshot struct {
	Pending, Applied      float64
	Persisted             *float64
	Revision              uint64
	Firmware, Persistence string
	RetryAvailable        bool
	Error                 Error
}
type RemapSnapshot struct {
	Pending, Applied, Factory x6.RemapConfig
	Actions                   []x6.RemapAction
	Revision                  uint64
	Firmware, Persistence     string
	RetryAvailable            bool
	Error                     Error
}
type StatusReader interface {
	Status(context.Context) (x6.Status, error)
}
type DPIWriter interface {
	ApplyAndPersist(context.Context, x6.DPIConfig, x6.AppliedDPIStore) error
}
type AppliedStore interface {
	x6.AppliedDPIStore
	LoadApplied() (x6.DPIConfig, error)
	LoadFactory() (x6.DPIConfig, error)
}

type DevicePersistence interface {
	Load(Binding) (x6.DPIConfig, error)
	Save(Binding, x6.DPIConfig) error
}
type PollingPersistence interface {
	Load(Binding) (x6.DeviceConfig, error)
	Save(Binding, x6.DeviceConfig) error
}

type resetRunner interface {
	Run(context.Context) (ResetResult, error)
}
type RemapPersistence interface {
	Load(Binding) (x6.DeviceConfig, error)
	Save(Binding, x6.DeviceConfig) error
}

// RemapConfigurationEvent is an immutable completion snapshot for one binding.
type RemapConfigurationEvent struct {
	Binding  Binding
	Snapshot RemapSnapshot
}

// StatusListener runs the always-on status listener until its context is
// cancelled, forwarding every dongle-pushed report through onStatus.
type StatusListener interface {
	Listen(context.Context, func(x6.StatusEvent)) error
}

// EventSink pushes live status updates to the frontend. The desktop service
// never imports Wails: the application wiring supplies the real emitter.
type EventSink interface {
	Emit(event string, payload any)
}

// StatusEvent is the payload forwarded through EventSink. Fields are nil unless
// the dongle report carried them.
type StatusEvent struct {
	ID                DeviceID
	Path              string
	InventoryRevision uint64
	Connection        string
	Battery           *int
	ActiveStage       *int
}

// ConfigurationEvent carries an immutable binding with its latest sync snapshot.
type ConfigurationEvent struct {
	Binding  Binding
	Snapshot Snapshot
}

// PollingConfigurationEvent reports a completed polling apply or persistence retry.
// It is separate from ConfigurationEvent so existing DPI event consumers remain compatible.
type PollingConfigurationEvent struct {
	Binding  Binding
	Snapshot PollingSnapshot
}

type deviceState struct {
	mu                         sync.Mutex
	applyMu                    sync.Mutex
	applied, pending, factory  x6.DPIConfig
	connection                 x6.Connection
	battery                    *int
	revision                   uint64
	err                        Error
	firmware, persistence      string
	retry                      *x6.DPIConfig
	observedStage, observedDPI *int
}
type settingsState struct {
	mu                                     sync.Mutex
	applyMu                                sync.Mutex
	normalSleep, responseTime              float64
	responseTimeMs                         int
	persistedNormal                        *float64
	persistedResponse                      *int
	retryNormal                            *float64
	retryResponse                          *int
	normalRevision, responseRevision       uint64
	normalFirmware, responseFirmware       string
	normalPersistence, responsePersistence string
	normalError, responseError             Error
}
type lightingState struct {
	mu       sync.Mutex
	applyMu  sync.Mutex
	pending  x6.LightingSelection
	applied  *x6.LightingSelection
	revision uint64
	firmware string
	err      Error
}
type remapState struct {
	mu                        sync.Mutex
	applyMu                   sync.Mutex
	pending, applied, factory x6.RemapConfig
	retry                     *x6.RemapConfig
	revision                  uint64
	firmware, persistence     string
	err                       Error
}

type Service struct {
	status StatusReader
	writer DPIWriter
	store  AppliedStore
	*listenerComponent
	*inventoryComponent
	*dpiComponent
	pollingComponent    *pollingComponent
	mu                  sync.Mutex
	settingsPersistence PollingPersistence
	configMu            sync.Mutex
	lightingStates      map[DeviceID]*lightingState
	remapStates         map[DeviceID]*remapState
	legacyRemap         *remapState
	remapPersistence    RemapPersistence
	operationMu         sync.Mutex
	reset               resetRunner
	update              *updateState
}

func New(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	applied, err := store.LoadApplied()
	if err != nil {
		applied = x6.DefaultDPIConfig()
	}
	factory, err := store.LoadFactory()
	if err != nil {
		factory = x6.DefaultDPIConfig()
	}
	s := &Service{status: status, writer: writer, store: store, inventoryComponent: newInventoryComponent(), listenerComponent: &listenerComponent{}, lightingStates: make(map[DeviceID]*lightingState), remapStates: make(map[DeviceID]*remapState), legacyRemap: newRemapState(x6.DefaultRemapConfig(), x6.DefaultRemapConfig())}
	s.dpiComponent = &dpiComponent{service: s, legacy: newDeviceState(applied, factory), states: make(map[DeviceID]*deviceState)}
	s.pollingComponent = &pollingComponent{service: s, states: s.inventoryComponent.pollingStates}
	return s
}
func Compose(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	return New(status, writer, store)
}

// AttachListener wires the always-on status listener and the frontend event
// sink. It does not start listening; call StartListener with a context.
func (s *Service) AttachListener(listener StatusListener, events EventSink) *Service {
	s.listenerComponent.attach(s, listener, events)
	return s
}

// AttachInventory wires the targeted device service used for explicit desktop selection.
func (s *Service) AttachInventory(inventory *mouse.TargetedService) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inventoryComponent.attach(inventory, s)
	return s
}

// AttachResetRunner wires the reset orchestration shared by desktop and CLI entrypoints.
func (s *Service) AttachResetRunner(runner resetRunner) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reset = runner
	return s
}

func (s *Service) attachPollingAutomaticSave(scheduler SyncScheduler) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pollingComponent.attachSync(scheduler)
	s.inventoryComponent.pollingSync = s.pollingComponent.sync
	return s
}

type realSyncScheduler struct{}

func (realSyncScheduler) After(delay time.Duration, f func()) SyncCancel {
	timer := time.AfterFunc(delay, f)
	return func() { timer.Stop() }
}

// attachAutomaticSave replaces the production timer for deterministic tests.
func (s *Service) attachAutomaticSave(scheduler SyncScheduler) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dpiComponent.attachSync(scheduler)
	return s
}

// AttachMigrator wires backup-first legacy migration to a selected device.
func (s *Service) AttachMigrator(migrate func(Binding) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.migrate = migrate
	return s
}

// AttachDevicePersistence wires keyed selected-device applied-state storage.
func (s *Service) AttachDevicePersistence(load func(Binding) (x6.DPIConfig, error), save func(Binding, x6.DPIConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devicePersistence = devicePersistence{load: load, save: func(binding Binding, config x6.DPIConfig) error {
		s.configMu.Lock()
		defer s.configMu.Unlock()
		return save(binding, config)
	}}
	return s
}

func (s *Service) AttachPollingPersistence(load func(Binding) (x6.DeviceConfig, error), save func(Binding, x6.DeviceConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pollingComponent.persistence = pollingPersistence{load: load, save: save}
	return s
}

// AttachNormalSleepPersistence and AttachDebouncePersistence share the durable
// per-device record so applying either field never discards the other.
func (s *Service) AttachNormalSleepPersistence(load func(Binding) (x6.DeviceConfig, error), save func(Binding, x6.DeviceConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settingsPersistence = pollingPersistence{load: load, save: save}
	return s
}
func (s *Service) AttachDebouncePersistence(load func(Binding) (x6.DeviceConfig, error), save func(Binding, x6.DeviceConfig) error) *Service {
	return s.AttachNormalSleepPersistence(load, save)
}

type devicePersistence struct {
	load func(Binding) (x6.DPIConfig, error)
	save func(Binding, x6.DPIConfig) error
}
type pollingPersistence struct {
	load func(Binding) (x6.DeviceConfig, error)
	save func(Binding, x6.DeviceConfig) error
}

// saveDeviceConfig serializes read-modify-write across the shared device record.
func (s *Service) saveDeviceConfig(binding Binding, persistence PollingPersistence, update func(*x6.DeviceConfig)) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	config, err := persistence.Load(binding)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		config = x6.DefaultDeviceConfig()
	}
	update(&config)
	return persistence.Save(binding, config)
}

func (p pollingPersistence) Load(binding Binding) (x6.DeviceConfig, error) {
	if p.load == nil {
		return x6.DeviceConfig{}, os.ErrNotExist
	}
	return p.load(binding)
}
func (p pollingPersistence) Save(binding Binding, config x6.DeviceConfig) error {
	if p.save == nil {
		return nil
	}
	return p.save(binding, config)
}

func (p devicePersistence) Load(binding Binding) (x6.DPIConfig, error) {
	if p.load == nil {
		return x6.DPIConfig{}, os.ErrNotExist
	}
	return p.load(binding)
}
func (p devicePersistence) Save(binding Binding, config x6.DPIConfig) error {
	if p.save == nil {
		return nil
	}
	return p.save(binding, config)
}

// RefreshInventory exposes all discovered devices and the explicit selection.
func (s *Service) RefreshInventory(ctx context.Context) Inventory {
	return s.inventoryComponent.refresh(ctx, s)
}

// SelectDevice establishes an explicit binding for a previously inventoried device.
func (s *Service) SelectDevice(id DeviceID) Inventory {
	return s.inventoryComponent.selectDevice(id, s)
}

// StartListener runs the status listener until ctx is cancelled, forwarding
// every dongle-pushed status report into the service state and the frontend.
// It is a no-op when no listener has been attached.
func (s *Service) StartListener(ctx context.Context) {
	s.listenerComponent.start(ctx, s)
}

// handleStatusEvent folds one dongle-pushed report into the shared state and
// emits the delta to the frontend. The listener callback is serialized by
// Listen, so only the state lock is needed here.
func (s *Service) handleStatusEvent(event x6.StatusEvent) {
	s.listenerComponent.handleStatusEvent(s, event)
}

// handleAttributedStatusEvent accepts listener data only for the currently
// selected immutable binding and its inventory revision.
func (s *Service) handleAttributedStatusEvent(event StatusEvent) {
	s.listenerComponent.handleAttributedStatusEvent(s, event)
}

func statusDelta(event x6.StatusEvent) (*int, *int) {
	var battery, stage *int
	if event.BatteryAvailable {
		value := event.BatteryPercent
		battery = &value
	}
	if event.StageAvailable && event.ActiveStage >= 1 && event.ActiveStage <= 8 {
		value := int(event.ActiveStage)
		stage = &value
	}
	return battery, stage
}

func (s *Service) foldStatusEvent(state *deviceState, event StatusEvent, eventName string) {
	s.listenerComponent.foldStatusEvent(s, state, event, eventName)
}
func (s *Service) GetSnapshot() Snapshot {
	return snapshotOf(s.currentState())
}
func (s *Service) RefreshStatus(ctx context.Context) Snapshot {
	status, err := s.status.Status(ctx)
	state := s.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if err != nil {
		state.err = Error{Code: errorCode(err, true)}
		return snapshotLocked(state)
	}
	state.connection = status.Connection
	if status.BatteryAvailable {
		battery := status.BatteryPercent
		state.battery = &battery
	}
	state.err = Error{}
	return snapshotLocked(state)
}
func (s *Service) StageDPI(config DPIConfig) Snapshot { return s.dpiComponent.stage(config) }

// GetPollingSnapshot reports desired, acknowledged, and persistence state; it
// deliberately does not claim a live hardware observation.
func (s *Service) GetPollingSnapshot() PollingSnapshot {
	return s.pollingComponent.snapshot()
}

// GetLightingSnapshot reports staged and acknowledged lighting state without
// claiming a live hardware read.
func (s *Service) GetLightingSnapshot() LightingSnapshot {
	return lightingSnapshotOf(s.currentLightingState())
}

// StageLighting updates only the selected device's pending state.
func (s *Service) StageLighting(selection x6.LightingSelection) LightingSnapshot {
	state := s.currentLightingState()
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

// ApplyLighting writes the staged catalog vector through one validated binding.
func (s *Service) ApplyLighting() LightingSnapshot {
	binding, ok := s.selectedBinding()
	state := s.currentLightingState()
	if !ok {
		return s.failLighting(state, SelectionRequired)
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	state.mu.Lock()
	selection, revision := state.pending, state.revision
	state.mu.Unlock()
	if !s.bindingCurrent(binding) {
		return s.failLighting(state, StaleBinding)
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return s.failLighting(state, StaleBinding)
	}
	settingsState := s.currentSettingsState()
	settingsState.mu.Lock()
	settings := x6.LightingSettings{LightingSelection: selection, NormalSleepMinutes: settingsState.normalSleep, ResponseTimeMs: settingsState.responseTimeMs}
	settingsState.mu.Unlock()
	if err := inventory.ApplyOperationBound(context.Background(), binding, x6.NewLightingSettingsOperation(), settings); err != nil {
		if errors.Is(err, mouse.ErrStaleBinding) {
			return s.failLighting(state, StaleBinding)
		}
		return s.failLighting(state, errorCode(err, false))
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

// GetRemapSnapshot reports local remap truth; the device has no remap readback.
func (s *Service) GetRemapSnapshot() RemapSnapshot { return remapSnapshotOf(s.currentRemapState()) }

// ApplyRemap validates and writes one complete draft. Applied state advances
// only after the targeted command receives its ACK.
func (s *Service) ApplyRemap(config x6.RemapConfig) RemapSnapshot {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, ok := s.selectedBinding()
	state := s.currentRemapState()
	if !ok {
		return s.failRemap(state, SelectionRequired)
	}
	if err := x6.NewRemapOperation().Validate(config); err != nil {
		return s.failRemap(state, InvalidConfiguration)
	}
	state.mu.Lock()
	if remapConfigsEqual(state.applied, config) && state.firmware == "success" {
		retry := state.retry != nil
		snapshot := remapSnapshotLocked(state)
		state.mu.Unlock()
		if retry {
			return s.RetryRemapPersistence()
		}
		return snapshot
	}
	state.pending = cloneRemapConfig(config)
	state.revision++
	revision := state.revision
	state.firmware, state.persistence, state.retry, state.err = "pending", "", nil, Error{}
	state.mu.Unlock()
	if err := s.applyRemapBound(binding, revision, config); err != nil {
		return remapSnapshotOf(state)
	}
	return remapSnapshotOf(state)
}

func (s *Service) applyRemapBound(binding Binding, revision uint64, pending x6.RemapConfig) error {
	state := s.currentRemapState()
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	if !s.bindingCurrent(binding) {
		s.failRemap(state, StaleBinding)
		return mouse.ErrStaleBinding
	}
	s.mu.Lock()
	inventory, persistence := s.inventory, s.remapPersistence
	s.mu.Unlock()
	if inventory == nil {
		s.failRemap(state, StaleBinding)
		return mouse.ErrStaleBinding
	}
	if err := inventory.ApplyOperationBound(context.Background(), binding, x6.NewRemapOperation(), pending); err != nil {
		if errors.Is(err, mouse.ErrStaleBinding) || errors.Is(err, mouse.ErrRevisionChanged) {
			s.failRemap(state, StaleBinding)
			return err
		}
		s.failRemap(state, errorCode(err, false))
		return err
	}
	state.mu.Lock()
	if state.revision != revision || !s.bindingCurrent(binding) {
		state.mu.Unlock()
		s.failRemap(state, StaleBinding)
		return mouse.ErrRevisionChanged
	}
	state.applied, state.firmware, state.err = cloneRemapConfig(pending), "success", Error{}
	state.mu.Unlock()
	if binding.SessionOnly || persistence == nil {
		s.emitRemapConfiguration(binding, remapSnapshotOf(state))
		return nil
	}
	if err := s.saveDeviceConfig(binding, persistence, func(config *x6.DeviceConfig) { config.Remap = &pending }); err != nil {
		state.mu.Lock()
		retry := cloneRemapConfig(pending)
		state.retry, state.persistence, state.err = &retry, "failed", Error{Code: PersistenceFailed}
		state.mu.Unlock()
		s.emitRemapConfiguration(binding, remapSnapshotOf(state))
		return nil
	}
	state.mu.Lock()
	state.persistence = "success"
	state.mu.Unlock()
	s.emitRemapConfiguration(binding, remapSnapshotOf(state))
	return nil
}

func (s *Service) RetryRemapPersistence() RemapSnapshot {
	binding, ok := s.selectedBinding()
	state := s.currentRemapState()
	if !ok {
		return s.failRemap(state, SelectionRequired)
	}
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
		return s.failRemap(state, PersistenceFailed)
	}
	state.mu.Lock()
	state.retry, state.persistence, state.err = nil, "success", Error{}
	state.mu.Unlock()
	return remapSnapshotOf(state)
}

func (s *Service) emitRemapConfiguration(binding Binding, snapshot RemapSnapshot) {
	s.listenerComponent.emit(s, "mouse:remap-configuration", RemapConfigurationEvent{Binding: binding, Snapshot: snapshot})
}

func (s *Service) failRemap(state *remapState, code ErrorCode) RemapSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.firmware, state.err = "failed", Error{Code: code}
	return remapSnapshotLocked(state)
}

func (s *Service) failLighting(state *lightingState, code ErrorCode) LightingSnapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.firmware = "failed"
	state.err = Error{Code: code}
	return lightingSnapshotLocked(state)
}

func (s *Service) StagePollingRate(rate x6.PollingRate) PollingSnapshot {
	return s.pollingComponent.stage(rate)
}

func (s *Service) RetryPollingPersistence() PollingSnapshot {
	return s.pollingComponent.retryPersistence()
}

// ApplyPollingRate writes the selected pending polling rate after explicit user confirmation.
func (s *Service) ApplyPollingRate(ctx context.Context) PollingSnapshot {
	return s.pollingComponent.apply(ctx)
}

// ResetToFactory delegates one explicitly confirmed reset to the shared runner.
func (s *Service) ResetToFactory(ctx context.Context) ResetResult {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	s.mu.Lock()
	runner := s.reset
	s.mu.Unlock()
	if runner == nil {
		return ResetResult{Error: Error{Code: SelectionRequired}, RetryAvailable: true}
	}
	result, err := runner.Run(ctx)
	if err == nil && result.Cleanup.State == "success" {
		s.reconcileFactoryReset()
	}
	return result
}

func (s *Service) reconcileFactoryReset() {
	state := s.currentState()
	state.mu.Lock()
	factory := x6.DocumentedResetDPIConfig()
	state.applied, state.pending = factory, factory
	state.revision++
	state.firmware, state.persistence, state.retry, state.err = "success", "success", nil, Error{}
	state.mu.Unlock()

	s.pollingComponent.reconcileFactoryReset()

	remap := s.currentRemapState()
	remap.mu.Lock()
	defaults := x6.DefaultRemapConfig()
	remap.pending, remap.applied, remap.retry = cloneRemapConfig(defaults), cloneRemapConfig(defaults), nil
	remap.revision++
	remap.firmware, remap.persistence, remap.err = "success", "success", Error{}
	remap.mu.Unlock()
}

func (s *Service) RetryPersistence() Snapshot            { return s.dpiComponent.retryPersistence() }
func (s *Service) ApplyDPI(ctx context.Context) Snapshot { return s.dpiComponent.apply(ctx) }

func (s *Service) applyFailure(err error) Snapshot { return s.dpiComponent.applyFailure(err) }

func newDeviceState(applied, factory x6.DPIConfig) *deviceState {
	return &deviceState{applied: applied, pending: applied, factory: factory}
}

func (s *Service) currentState() *deviceState { return s.dpiComponent.currentState() }

func (s *Service) currentLightingState() *lightingState {
	binding, ok := s.selectedBinding()
	if !ok {
		return newLightingState()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.lightingStates[binding.ID]
	if state == nil {
		state = newLightingState()
		s.lightingStates[binding.ID] = state
	}
	return state
}

func newRemapState(applied, factory x6.RemapConfig) *remapState {
	return &remapState{applied: cloneRemapConfig(applied), pending: cloneRemapConfig(applied), factory: cloneRemapConfig(factory)}
}

func (s *Service) currentRemapState() *remapState {
	binding, ok := s.selectedBinding()
	if !ok {
		return s.legacyRemap
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.remapStates[binding.ID]
	if state == nil {
		factory := x6.DefaultRemapConfig()
		applied := factory
		if s.remapPersistence != nil && !binding.SessionOnly {
			if persisted, err := s.remapPersistence.Load(binding); err == nil && persisted.Remap != nil {
				applied = cloneRemapConfig(*persisted.Remap)
			}
		}
		state = newRemapState(applied, factory)
		s.remapStates[binding.ID] = state
	}
	return state
}

func (s *Service) selectedBinding() (Binding, bool) {
	return (selectionResolver{s}).selected()
}

func (s *Service) bindingCurrent(binding Binding) bool {
	return (selectionResolver{s}).current(binding)
}

func (s *Service) cancelSync(binding Binding) { s.dpiComponent.cancelSync(binding) }

func (s *Service) cancelPollingSync(binding Binding) { s.pollingComponent.cancelSync(binding) }

func (s *Service) currentPollingState() *pollingState { return s.pollingComponent.currentState() }
func (s *Service) applyPollingBound(binding Binding, revision uint64, rate x6.PollingRate) error {
	return s.pollingComponent.applyPollingBound(binding, revision, rate)
}

func (s *Service) applyBound(binding Binding, revision uint64, config x6.DPIConfig) error {
	return s.dpiComponent.applyBound(binding, revision, config)
}

func (s *Service) emitConfiguration(binding Binding, state *deviceState) {
	s.listenerComponent.emitConfiguration(s, binding, state)
}

func (s *Service) newStateFromLegacy() *deviceState { return s.dpiComponent.newStateFromLegacy() }

func snapshotOf(state *deviceState) Snapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return snapshotLocked(state)
}

func snapshotLocked(state *deviceState) Snapshot {
	return Snapshot{Connection: string(state.connection), Battery: state.battery, Applied: ToDTO(state.applied), Pending: ToDTO(state.pending), Factory: ToDTO(state.factory), Revision: state.revision, Error: state.err, Firmware: state.firmware, Persistence: state.persistence, RetryAvailable: state.retry != nil, ObservedStage: state.observedStage, ObservedDPI: state.observedDPI}
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

func mappedDPI(config x6.DPIConfig, stage int) *int {
	if stage < 1 || stage > len(config.DPI) || config.StageMask&(1<<uint(stage-1)) == 0 {
		return nil
	}
	dpi := config.DPI[stage-1]
	return &dpi
}

func validStage(stage *int) bool { return stage != nil && *stage >= 1 && *stage <= 8 }
func ToDTO(config x6.DPIConfig) DPIConfig {
	return DPIConfig{config.AngleControl, config.RippleControl, config.StageMask, config.LiftDistance, config.DPI, config.ActiveStage, config.Colors}
}
func fromDTO(config DPIConfig) x6.DPIConfig {
	return x6.DPIConfig{AngleControl: config.AngleControl, RippleControl: config.RippleControl, StageMask: config.StageMask, LiftDistance: config.LiftDistance, DPI: config.DPI, ActiveStage: config.ActiveStage, Colors: config.Colors}
}
func errorCode(err error, status bool) ErrorCode {
	if errors.Is(err, mouse.ErrSelectionRequired) {
		return SelectionRequired
	}
	if errors.Is(err, mouse.ErrStaleBinding) {
		return StaleBinding
	}
	if x6.IsErrorKind(err, x6.PersistFailure) {
		return PersistenceFailed
	}
	if x6.IsErrorKind(err, x6.NoUsableDevice) {
		return DeviceUnavailable
	}
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	if hidlinux.IsErrorKind(err, hidlinux.Disconnected) || errors.Is(err, os.ErrNotExist) {
		return DeviceDisconnected
	}
	if status && x6.IsErrorKind(err, x6.ReadFailure) {
		return StatusReadFailed
	}
	if status {
		return DeviceUnavailable
	}
	return ApplyFailed
}

func applyErrorClassification(err error) string {
	if hidlinux.IsErrorKind(err, hidlinux.Timeout) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if operation := hidlinux.DiagnosticOperation(err); operation != "" {
		return operation
	}
	if x6.IsErrorKind(err, x6.AckFailure) {
		return "ack_failure"
	}
	return "unknown"
}
