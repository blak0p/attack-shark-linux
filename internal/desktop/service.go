package desktop

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/alejandro/attack-shark-linux/internal/mouse"
	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type ErrorCode string

const (
	DeviceUnavailable    ErrorCode = "device_unavailable"
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
	Connection string
	Battery    *int
	Applied    DPIConfig
	Pending    DPIConfig
	Factory    DPIConfig
	Revision   uint64
	Error      Error
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

type deviceState struct {
	mu                        sync.Mutex
	applyMu                   sync.Mutex
	applied, pending, factory x6.DPIConfig
	connection                x6.Connection
	battery                   *int
	revision                  uint64
	err                       Error
}

type Service struct {
	status            StatusReader
	writer            DPIWriter
	store             AppliedStore
	listener          StatusListener
	events            EventSink
	inventory         *mouse.TargetedService
	migrate           func(Binding) error
	devicePersistence DevicePersistence
	inventoryDevices  []Device
	mu                sync.Mutex
	legacy            *deviceState
	states            map[DeviceID]*deviceState
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
	return &Service{status: status, writer: writer, store: store, legacy: newDeviceState(applied, factory), states: make(map[DeviceID]*deviceState)}
}
func Compose(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	return New(status, writer, store)
}

// AttachListener wires the always-on status listener and the frontend event
// sink. It does not start listening; call StartListener with a context.
func (s *Service) AttachListener(listener StatusListener, events EventSink) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listener = listener
	s.events = events
	return s
}

// AttachInventory wires the targeted device service used for explicit desktop selection.
func (s *Service) AttachInventory(inventory *mouse.TargetedService) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inventory = inventory
	s.inventoryDevices = nil
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
	s.devicePersistence = devicePersistence{load: load, save: save}
	return s
}

type devicePersistence struct {
	load func(Binding) (x6.DPIConfig, error)
	save func(Binding, x6.DPIConfig) error
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
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return Inventory{Error: Error{Code: DeviceUnavailable}}
	}
	devices, err := inventory.Refresh(ctx)
	if err != nil {
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
			s.states[device.ID] = state
		}
	}
	s.mu.Unlock()
	return result
}

// SelectDevice establishes an explicit binding for a previously inventoried device.
func (s *Service) SelectDevice(id DeviceID) Inventory {
	s.mu.Lock()
	inventory := s.inventory
	devices := append([]Device(nil), s.inventoryDevices...)
	migrate := s.migrate
	s.mu.Unlock()
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
	if !selected.SessionOnly && migrate != nil && migrate(selected) != nil {
		return Inventory{Devices: devices, Selected: &selected, Error: Error{Code: MigrationFailed}}
	}
	return Inventory{Devices: devices, Selected: &selected}
}

// StartListener runs the status listener until ctx is cancelled, forwarding
// every dongle-pushed status report into the service state and the frontend.
// It is a no-op when no listener has been attached.
func (s *Service) StartListener(ctx context.Context) {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	if listener == nil {
		return
	}
	go func() {
		_ = listener.Listen(ctx, s.handleStatusEvent)
	}()
}

// handleStatusEvent folds one dongle-pushed report into the shared state and
// emits the delta to the frontend. The listener callback is serialized by
// Listen, so only the state lock is needed here.
func (s *Service) handleStatusEvent(event x6.StatusEvent) {
	battery, stage := statusDelta(event)
	s.mu.Lock()
	inventory := s.inventory
	devices := len(s.inventoryDevices)
	s.mu.Unlock()
	if inventory != nil {
		selected, ok := inventory.Selection()
		if !ok || devices != 1 {
			return
		}
		s.handleAttributedStatusEvent(StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, Connection: string(event.Connection), Battery: battery, ActiveStage: stage})
		return
	}
	s.foldStatusEvent(s.legacy, StatusEvent{Connection: string(event.Connection), Battery: battery, ActiveStage: stage}, "x6:status")
}

// handleAttributedStatusEvent accepts listener data only for the currently
// selected immutable binding and its inventory revision.
func (s *Service) handleAttributedStatusEvent(event StatusEvent) {
	s.mu.Lock()
	inventory := s.inventory
	sink := s.events
	s.mu.Unlock()
	if inventory == nil {
		return
	}
	selected, ok := inventory.Selection()
	if !ok || selected.ID != event.ID || selected.Path != event.Path || selected.InventoryRevision != event.InventoryRevision {
		return
	}
	s.mu.Lock()
	state := s.states[event.ID]
	s.mu.Unlock()
	if state == nil {
		return
	}
	s.foldStatusEvent(state, event, "mouse:status")
	_ = sink
}

func statusDelta(event x6.StatusEvent) (*int, *int) {
	var battery, stage *int
	if event.BatteryAvailable {
		value := event.BatteryPercent
		battery = &value
	}
	if event.StageAvailable {
		value := int(event.ActiveStage)
		stage = &value
	}
	return battery, stage
}

func (s *Service) foldStatusEvent(state *deviceState, event StatusEvent, eventName string) {
	state.mu.Lock()
	state.connection = x6.Connection(event.Connection)
	if event.Battery != nil {
		value := *event.Battery
		state.battery = &value
	}
	if event.ActiveStage != nil {
		state.applied.ActiveStage = byte(*event.ActiveStage)
		state.pending.ActiveStage = byte(*event.ActiveStage)
		state.revision++
	}
	state.mu.Unlock()
	s.mu.Lock()
	sink := s.events
	s.mu.Unlock()
	if sink != nil {
		sink.Emit(eventName, event)
	}
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
func (s *Service) StageDPI(config DPIConfig) Snapshot {
	state := s.currentState()
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
	return snapshotLocked(state)
}
func (s *Service) ApplyDPI(ctx context.Context) Snapshot {
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
			return s.applyFailure(err)
		}
		if err := inventory.Apply(ctx); err != nil {
			return s.applyFailure(err)
		}
		if binding, ok := inventory.Selection(); ok {
			s.mu.Lock()
			persistence := s.devicePersistence
			s.mu.Unlock()
			if !binding.SessionOnly && persistence != nil {
				if err := persistence.Save(binding, pending); err != nil {
					return s.applyFailure(&x6.ServiceError{Kind: x6.PersistFailure, Err: err})
				}
			}
		}
	} else if err := s.writer.ApplyAndPersist(ctx, pending, s.store); err != nil {
		return s.applyFailure(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.applied = pending
	state.err = Error{}
	return snapshotLocked(state)
}

func (s *Service) applyFailure(err error) Snapshot {
	slog.Error("apply DPI failed", "error", err, "classification", applyErrorClassification(err))
	state := s.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.err = Error{Code: errorCode(err, false)}
	return snapshotLocked(state)
}

func newDeviceState(applied, factory x6.DPIConfig) *deviceState {
	return &deviceState{applied: applied, pending: applied, factory: factory}
}

func (s *Service) currentState() *deviceState {
	s.mu.Lock()
	inventory := s.inventory
	legacy := s.legacy
	states := s.states
	s.mu.Unlock()
	if inventory == nil {
		return legacy
	}
	selected, ok := inventory.Selection()
	if !ok {
		return legacy
	}
	s.mu.Lock()
	state := states[selected.ID]
	if state == nil {
		state = s.newStateFromLegacy()
		states[selected.ID] = state
	}
	s.mu.Unlock()
	return state
}

func (s *Service) newStateFromLegacy() *deviceState {
	s.legacy.mu.Lock()
	defer s.legacy.mu.Unlock()
	return newDeviceState(s.legacy.applied, s.legacy.factory)
}

func snapshotOf(state *deviceState) Snapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return snapshotLocked(state)
}

func snapshotLocked(state *deviceState) Snapshot {
	return Snapshot{Connection: string(state.connection), Battery: state.battery, Applied: ToDTO(state.applied), Pending: ToDTO(state.pending), Factory: ToDTO(state.factory), Revision: state.revision, Error: state.err}
}
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
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	if x6.IsErrorKind(err, x6.PersistFailure) {
		return PersistenceFailed
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
