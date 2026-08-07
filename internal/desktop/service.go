package desktop

import (
	"context"
	"errors"
	"os"
	"sync"

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

// DPIConfig is the desktop-facing complete DPI configuration DTO.
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
}

type Service struct {
	status   StatusReader
	writer   DPIWriter
	store    AppliedStore
	mu       sync.Mutex
	applyMu  sync.Mutex
	applied  x6.DPIConfig
	pending  x6.DPIConfig
	revision uint64
}

func New(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	applied, err := store.LoadApplied()
	if err != nil {
		applied = x6.DefaultDPIConfig()
	}
	return &Service{status: status, writer: writer, store: store, applied: applied, pending: applied}
}

// Compose creates the Wails-facing service from the x6 status, command, and state adapters.
func Compose(status StatusReader, writer DPIWriter, store AppliedStore) *Service {
	return New(status, writer, store)
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
	snapshot := s.snapshot(Error{})
	snapshot.Connection = string(status.Connection)
	if status.BatteryAvailable {
		battery := status.BatteryPercent
		snapshot.Battery = &battery
	}
	return snapshot
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
	return Snapshot{Applied: ToDTO(s.applied), Pending: ToDTO(s.pending), Revision: s.revision, Error: err}
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
