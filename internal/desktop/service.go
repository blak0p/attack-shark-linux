package desktop

import (
	"context"
	"errors"
	"fmt"
	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/hidlinux"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
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
	Desired, Applied device.PollingRate
	Persisted        *device.PollingRate
	Factory          device.PollingRate
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
	Pending  device.LightingSelection
	Applied  *device.LightingSelection
	Effects  []device.LightingEffect
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
	MacroPending, MacroApplied *MacroDraft
	MacroDrafts                map[uint8]MacroDraft
	MacroAppliedDrafts         map[uint8]MacroDraft
	MacroProgress              mouse.MacroProgress
	Pending, Applied, Factory  device.RemapConfig
	Actions                    []device.RemapAction
	Revision                   uint64
	Firmware, Persistence      string
	RetryAvailable             bool
	Error                      Error
}
type StatusReader interface {
	Status(context.Context) (device.Status, error)
}
type DPIWriter interface {
	ApplyAndPersist(context.Context, device.DPIConfig, device.AppliedDPIStore) error
}
type AppliedStore interface {
	device.AppliedDPIStore
	LoadApplied() (device.DPIConfig, error)
	LoadFactory() (device.DPIConfig, error)
}

type DevicePersistence interface {
	Load(Binding) (device.DPIConfig, error)
	Save(Binding, device.DPIConfig) error
}
type PollingPersistence interface {
	Load(Binding) (device.DeviceConfig, error)
	Save(Binding, device.DeviceConfig) error
}

type resetRunner interface {
	Run(context.Context) (ResetResult, error)
}
type RemapPersistence interface {
	Load(Binding) (device.DeviceConfig, error)
	Save(Binding, device.DeviceConfig) error
}

// RemapConfigurationEvent is an immutable completion snapshot for one binding.
type RemapConfigurationEvent struct {
	Binding  Binding
	Snapshot RemapSnapshot
}

// StatusListener runs the always-on status listener until its context is
// cancelled, forwarding every dongle-pushed report through onStatus.
type StatusListener interface {
	Listen(context.Context, func(device.StatusEvent)) error
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
	applied, pending, factory  device.DPIConfig
	connection                 transport.Connection
	battery                    *int
	revision                   uint64
	err                        Error
	firmware, persistence      string
	retry                      *device.DPIConfig
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
	effects  []device.LightingEffect
	mu       sync.Mutex
	applyMu  sync.Mutex
	pending  device.LightingSelection
	applied  *device.LightingSelection
	revision uint64
	firmware string
	err      Error
}

type Service struct {
	model             device.Model
	profiles          *profileComponent
	macroLibrary      *macros.Library
	macroLibraryError error
	status            StatusReader
	writer            DPIWriter
	store             AppliedStore
	*listenerComponent
	*inventoryComponent
	*dpiComponent
	pollingComponent    *pollingComponent
	mu                  sync.Mutex
	settingsPersistence PollingPersistence
	configMu            sync.Mutex
	lightingComponent   *lightingComponent
	remapComponent      *remapComponent
	remapPersistence    RemapPersistence
	operationMu         sync.Mutex
	reset               resetRunner
	update              *updateState
	watcherSettings     watcherComponent
	gameCatalog         gameComponent
	launcher            launcherComponent
	autostart           autostartComponent
}

func New(model device.Model, status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	applied, err := store.LoadApplied()
	if err != nil {
		applied = device.DefaultDPIConfig()
	}
	factory, err := store.LoadFactory()
	if err != nil {
		factory = device.DefaultDPIConfig()
	}
	s := &Service{model: model, status: status, writer: writer, store: store, inventoryComponent: newInventoryComponent(), listenerComponent: &listenerComponent{}}
	s.remapComponent = &remapComponent{service: s, states: make(map[DeviceID]*remapState), legacy: newRemapState(device.DefaultRemapConfig(), device.DefaultRemapConfig())}
	s.dpiComponent = &dpiComponent{service: s, legacy: newDeviceState(applied, factory), states: make(map[DeviceID]*deviceState)}
	s.pollingComponent = &pollingComponent{service: s, states: s.inventoryComponent.pollingStates}
	s.lightingComponent = &lightingComponent{service: s, states: make(map[DeviceID]*lightingState)}
	return s
}

// Option configures app-owned dependencies before the service is published.
type Option func(*Service)

func Compose(model device.Model, status StatusReader, writer DPIWriter, store AppliedStore, options ...Option) *Service {
	s := New(model, status, writer, store)
	for _, option := range options {
		option(s)
	}
	return s
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
func (s *Service) AttachDevicePersistence(load func(Binding) (device.DPIConfig, error), save func(Binding, device.DPIConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devicePersistence = devicePersistence{load: load, save: func(binding Binding, config device.DPIConfig) error {
		s.configMu.Lock()
		defer s.configMu.Unlock()
		return save(binding, config)
	}}
	return s
}

func (s *Service) AttachPollingPersistence(load func(Binding) (device.DeviceConfig, error), save func(Binding, device.DeviceConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pollingComponent.persistence = pollingPersistence{load: load, save: save}
	return s
}

// AttachNormalSleepPersistence and AttachDebouncePersistence share the durable
// per-device record so applying either field never discards the other.
func (s *Service) AttachNormalSleepPersistence(load func(Binding) (device.DeviceConfig, error), save func(Binding, device.DeviceConfig) error) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settingsPersistence = pollingPersistence{load: load, save: save}
	return s
}
func (s *Service) AttachDebouncePersistence(load func(Binding) (device.DeviceConfig, error), save func(Binding, device.DeviceConfig) error) *Service {
	return s.AttachNormalSleepPersistence(load, save)
}

type devicePersistence struct {
	load func(Binding) (device.DPIConfig, error)
	save func(Binding, device.DPIConfig) error
}
type pollingPersistence struct {
	load func(Binding) (device.DeviceConfig, error)
	save func(Binding, device.DeviceConfig) error
}

// saveDeviceConfig serializes read-modify-write across the shared device record.
func (s *Service) saveDeviceConfig(binding Binding, persistence PollingPersistence, update func(*device.DeviceConfig)) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	config, err := persistence.Load(binding)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		config = device.DefaultDeviceConfig()
	}
	update(&config)
	return persistence.Save(binding, config)
}

func (p pollingPersistence) Load(binding Binding) (device.DeviceConfig, error) {
	if p.load == nil {
		return device.DeviceConfig{}, os.ErrNotExist
	}
	return p.load(binding)
}
func (p pollingPersistence) Save(binding Binding, config device.DeviceConfig) error {
	if p.save == nil {
		return nil
	}
	return p.save(binding, config)
}

func (p devicePersistence) Load(binding Binding) (device.DPIConfig, error) {
	if p.load == nil {
		return device.DPIConfig{}, os.ErrNotExist
	}
	return p.load(binding)
}
func (p devicePersistence) Save(binding Binding, config device.DPIConfig) error {
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
func (s *Service) handleStatusEvent(event device.StatusEvent) {
	s.listenerComponent.handleStatusEvent(s, event)
}

// handleAttributedStatusEvent accepts listener data only for the currently
// selected immutable binding and its inventory revision.
func (s *Service) handleAttributedStatusEvent(event StatusEvent) {
	s.listenerComponent.handleAttributedStatusEvent(s, event)
}

func statusDelta(event device.StatusEvent) (*int, *int) {
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
	return s.dpiComponent.refreshStatus(ctx)
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
	return s.lightingComponent.snapshot()
}

// StageLighting updates only the selected device's pending state.
func (s *Service) StageLighting(selection device.LightingSelection) LightingSnapshot {
	return s.lightingComponent.stage(selection)
}

// ApplyLighting writes the staged catalog vector through one validated binding.
func (s *Service) ApplyLighting() LightingSnapshot { return s.lightingComponent.apply() }

// GetRemapSnapshot reports local remap truth; the device has no remap readback.
func (s *Service) GetRemapSnapshot() RemapSnapshot { return s.remapComponent.snapshot() }

// ApplyRemap validates and writes one complete draft. Applied state advances
// only after the targeted command receives its ACK.
func (s *Service) ApplyRemap(config device.RemapConfig) RemapSnapshot {
	return s.remapComponent.apply(config)
}

func (s *Service) RetryRemapPersistence() RemapSnapshot {
	return s.remapComponent.retryPersistence()
}

func (s *Service) emitRemapConfiguration(binding Binding, snapshot RemapSnapshot) {
	s.listenerComponent.emit(s, "mouse:remap-configuration", RemapConfigurationEvent{Binding: binding, Snapshot: snapshot})
}

func (s *Service) StagePollingRate(rate device.PollingRate) PollingSnapshot {
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
	s.dpiComponent.reconcileFactoryReset()

	s.pollingComponent.reconcileFactoryReset()

	s.remapComponent.reconcileFactoryReset()

	captured := device.DocumentedResetLightingSettings()
	settings := s.currentSettingsState()
	settings.mu.Lock()
	settings.normalSleep, settings.responseTimeMs = captured.NormalSleepMinutes, captured.ResponseTimeMs
	settings.persistedNormal, settings.persistedResponse = nil, nil
	settings.retryNormal, settings.retryResponse = nil, nil
	settings.normalRevision++
	settings.responseRevision++
	settings.normalFirmware, settings.responseFirmware = "success", "success"
	settings.normalPersistence, settings.responsePersistence = "", ""
	settings.normalError, settings.responseError = Error{}, Error{}
	settings.mu.Unlock()
	lighting := s.currentLightingState()
	lighting.mu.Lock()
	selection := captured.LightingSelection
	lighting.pending, lighting.applied = selection, &selection
	lighting.revision++
	lighting.firmware, lighting.err = "success", Error{}
	lighting.mu.Unlock()
}

func (s *Service) RetryPersistence() Snapshot            { return s.dpiComponent.retryPersistence() }
func (s *Service) ApplyDPI(ctx context.Context) Snapshot { return s.dpiComponent.apply(ctx) }

func (s *Service) applyFailure(err error) Snapshot { return s.dpiComponent.applyFailure(err) }

func newDeviceState(applied, factory device.DPIConfig) *deviceState {
	return &deviceState{applied: applied, pending: applied, factory: factory}
}

func (s *Service) currentState() *deviceState { return s.dpiComponent.currentState() }

func (s *Service) currentLightingState() *lightingState { return s.lightingComponent.currentState() }

func (s *Service) currentRemapState() *remapState { return s.remapComponent.currentState() }

func (s *Service) selectedBinding() (Binding, bool) {
	return (selectionResolver{s}).selected()
}

func (s *Service) bindingCurrent(binding Binding) bool {
	return (selectionResolver{s}).current(binding)
}

func (s *Service) cancelSync(binding Binding) { s.dpiComponent.cancelSync(binding) }

func (s *Service) cancelPollingSync(binding Binding) { s.pollingComponent.cancelSync(binding) }

func (s *Service) currentPollingState() *pollingState { return s.pollingComponent.currentState() }
func (s *Service) applyPollingBound(binding Binding, revision uint64, rate device.PollingRate) error {
	return s.pollingComponent.applyPollingBound(binding, revision, rate)
}

func (s *Service) applyBound(binding Binding, revision uint64, config device.DPIConfig) error {
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

func mappedDPI(config device.DPIConfig, stage int) *int {
	if stage < 1 || stage > len(config.DPI) || config.StageMask&(1<<uint(stage-1)) == 0 {
		return nil
	}
	dpi := config.DPI[stage-1]
	return &dpi
}

func validStage(stage *int) bool { return stage != nil && *stage >= 1 && *stage <= 8 }
func ToDTO(config device.DPIConfig) DPIConfig {
	return DPIConfig{config.AngleControl, config.RippleControl, config.StageMask, config.LiftDistance, config.DPI, config.ActiveStage, config.Colors}
}
func fromDTO(config DPIConfig) device.DPIConfig {
	return device.DPIConfig{AngleControl: config.AngleControl, RippleControl: config.RippleControl, StageMask: config.StageMask, LiftDistance: config.LiftDistance, DPI: config.DPI, ActiveStage: config.ActiveStage, Colors: config.Colors}
}
func errorCode(err error, status bool) ErrorCode {
	code := classifyErrorCode(err, status)
	if !status {
		logApplyError(code, "error", err)
	}
	return code
}

// logApplyError records why a settings write ended with a user-visible error
// code, including the caller so each settings domain can be told apart.
func logApplyError(code ErrorCode, reason string, err error) {
	caller := "unknown"
	if _, file, line, ok := runtime.Caller(2); ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	slog.Warn("x6_apply_error", "code", code, "reason", reason, "classification", applyErrorClassification(err), "caller", caller, "error", err)
}

func classifyErrorCode(err error, status bool) ErrorCode {
	if errors.Is(err, mouse.ErrSelectionRequired) {
		return SelectionRequired
	}
	if errors.Is(err, mouse.ErrStaleBinding) {
		return StaleBinding
	}
	if device.IsErrorKind(err, device.PersistFailure) {
		return PersistenceFailed
	}
	if device.IsErrorKind(err, device.NoUsableDevice) {
		return DeviceUnavailable
	}
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	if hidlinux.IsErrorKind(err, hidlinux.Disconnected) || errors.Is(err, os.ErrNotExist) {
		return DeviceDisconnected
	}
	if status && device.IsErrorKind(err, device.ReadFailure) {
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
	if device.IsErrorKind(err, device.AckFailure) {
		return "ack_failure"
	}
	return "unknown"
}
