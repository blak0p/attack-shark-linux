package device

import (
	"errors"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// ErrorKind classifies a device service failure independently of the model.
type ErrorKind string

const (
	NoUsableDevice ErrorKind = "no usable X6 device"
	ReadFailure    ErrorKind = "input read failed"
	InvalidDPI     ErrorKind = "invalid DPI configuration"
	WriteFailure   ErrorKind = "configuration write failed"
	AckFailure     ErrorKind = "configuration acknowledgement failed"
	PersistFailure ErrorKind = "applied-state persistence failed"
)

// ServiceError carries an ErrorKind and the underlying cause.
type ServiceError struct {
	Kind ErrorKind
	Err  error
}

func (e *ServiceError) Error() string { return fmt.Sprintf("%s: %v", e.Kind, e.Err) }
func (e *ServiceError) Unwrap() error { return e.Err }

// IsErrorKind reports whether err wraps a ServiceError of the given kind.
func IsErrorKind(err error, kind ErrorKind) bool {
	var target *ServiceError
	return errors.As(err, &target) && target.Kind == kind
}

// Status is the passive battery/connection reading of one device.
type Status struct {
	Connection       transport.Connection
	BatteryPercent   int
	BatteryAvailable bool
}

// StatusEvent is one dongle-pushed status report delivered by Listen. Exactly
// one field is meaningful per event: a heartbeat carries Battery, and a
// physical DPI button press carries ActiveStage.
type StatusEvent struct {
	Connection       transport.Connection
	BatteryPercent   int
	BatteryAvailable bool
	ActiveStage      byte
	StageAvailable   bool
}

// AppliedDPIStore persists the DPI configuration after a confirmed apply.
type AppliedDPIStore interface{ SaveApplied(DPIConfig) error }
