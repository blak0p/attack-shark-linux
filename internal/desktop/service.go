package desktop

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
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
)

type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}
type Error struct{ Code ErrorCode }
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
	Connection  string
	Battery     *int
	ActiveStage *int
}

type Service struct {
	status           StatusReader
	writer           DPIWriter
	store            AppliedStore
	listener         StatusListener
	events           EventSink
	mu               sync.Mutex
	applyMu          sync.Mutex
	applied, pending x6.DPIConfig
	factory          x6.DPIConfig
	connection       x6.Connection
	battery          *int
	revision         uint64
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
	return &Service{status: status, writer: writer, store: store, applied: applied, pending: applied, factory: factory}
}
func Compose(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	return New(status, writer, store)
}

// AttachListener wires the always-on status listener and the frontend event
// sink. It does not start listening; call StartListener with a context.
func (s *Service) AttachListener(listener StatusListener, events EventSink) *Service {
	s.listener = listener
	s.events = events
	return s
}

// StartListener runs the status listener until ctx is cancelled, forwarding
// every dongle-pushed status report into the service state and the frontend.
// It is a no-op when no listener has been attached.
func (s *Service) StartListener(ctx context.Context) {
	if s.listener == nil {
		return
	}
	go func() {
		_ = s.listener.Listen(ctx, s.handleStatusEvent)
	}()
}

// handleStatusEvent folds one dongle-pushed report into the shared state and
// emits the delta to the frontend. The listener callback is serialized by
// Listen, so only the state lock is needed here.
func (s *Service) handleStatusEvent(event x6.StatusEvent) {
	s.mu.Lock()
	s.connection = event.Connection
	var battery, stage *int
	if event.BatteryAvailable {
		value := event.BatteryPercent
		s.battery = &value
		battery = &value
	}
	if event.StageAvailable {
		value := int(event.ActiveStage)
		s.applied.ActiveStage = event.ActiveStage
		s.pending.ActiveStage = event.ActiveStage
		s.revision++
		stage = &value
	}
	s.mu.Unlock()
	if s.events != nil {
		s.events.Emit("x6:status", StatusEvent{Connection: string(event.Connection), Battery: battery, ActiveStage: stage})
	}
}
func (s *Service) GetSnapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot(Error{})
}
func (s *Service) RefreshStatus(ctx context.Context) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.status.Status(ctx)
	if err != nil {
		return s.snapshot(Error{Code: errorCode(err, true)})
	}
	s.connection = status.Connection
	if status.BatteryAvailable {
		battery := status.BatteryPercent
		s.battery = &battery
	}
	return s.snapshot(Error{})
}
func (s *Service) StageDPI(config DPIConfig) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := fromDTO(config)
	if _, err := x6.EncodeDPIReport(next); err != nil {
		return s.snapshot(Error{Code: InvalidConfiguration})
	}
	s.pending = next
	s.revision++
	return s.snapshot(Error{})
}
func (s *Service) ApplyDPI(ctx context.Context) Snapshot {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	s.mu.Lock()
	pending := s.pending
	s.mu.Unlock()
	if err := s.writer.ApplyAndPersist(ctx, pending, s.store); err != nil {
		slog.Error("apply DPI failed", "error", err, "classification", applyErrorClassification(err))
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.snapshot(Error{Code: errorCode(err, false)})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applied = pending
	return s.snapshot(Error{})
}
func (s *Service) snapshot(err Error) Snapshot {
	return Snapshot{Connection: string(s.connection), Battery: s.battery, Applied: ToDTO(s.applied), Pending: ToDTO(s.pending), Factory: ToDTO(s.factory), Revision: s.revision, Error: err}
}
func ToDTO(config x6.DPIConfig) DPIConfig {
	return DPIConfig{config.AngleControl, config.RippleControl, config.StageMask, config.LiftDistance, config.DPI, config.ActiveStage, config.Colors}
}
func fromDTO(config DPIConfig) x6.DPIConfig {
	return x6.DPIConfig{AngleControl: config.AngleControl, RippleControl: config.RippleControl, StageMask: config.StageMask, LiftDistance: config.LiftDistance, DPI: config.DPI, ActiveStage: config.ActiveStage, Colors: config.Colors}
}
func errorCode(err error, status bool) ErrorCode {
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
