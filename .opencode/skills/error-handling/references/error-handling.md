# error-handling — taxonomy references (real repo excerpts)

## 1. The closed `ErrorCode` enum and `Error` payload

`internal/desktop/service.go:16` defines the only error vocabulary the UI sees:

```go
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

type Error struct{ Code ErrorCode }
```

## 2. Single funnel: `errorCode`

Every failure routes through one mapper so classification is consistent
(`internal/desktop/service.go:1058`):

```go
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
```

## 3. Domain errors stay typed for `IsErrorKind`

`internal/x6/dpi.go:16` wraps while preserving the cause, and the domain exposes
a kind-based classifier instead of exported sentinels:

```go
func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	report, err := protocol.EncodeDPIReport(config)
	if err != nil {
		return nil, &ServiceError{InvalidDPI, fmt.Errorf("%w", err)}
	}
	return report, nil
}
```

## 4. Transport flavor for logs, not for the UI

`internal/desktop/service.go:1080` classifies the *why* for logging only:

```go
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
```

## 5. State stores the code, UI reads only the code

`internal/desktop/service.go:767` shows the failure path: the internal error is
logged with classification, but `state.err` only carries `Error{Code: ...}`:

```go
func (s *Service) applyFailure(err error) Snapshot {
	slog.Error("apply DPI failed", "error", err, "classification", applyErrorClassification(err))
	state := s.currentState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.err = Error{Code: errorCode(err, false)}
	return snapshotLocked(state)
}
```
