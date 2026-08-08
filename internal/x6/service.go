package x6

import (
	"context"
	"errors"
	"fmt"
	"sync"

	protocol "github.com/alejandro/attack-shark-linux/internal/protocol/x6"
	"github.com/alejandro/attack-shark-linux/internal/transport"
)

type ErrorKind string

const (
	NoUsableDevice ErrorKind = "no usable X6 device"
	ReadFailure    ErrorKind = "input read failed"
	InvalidDPI     ErrorKind = "invalid DPI configuration"
	WriteFailure   ErrorKind = "configuration write failed"
	AckFailure     ErrorKind = "configuration acknowledgement failed"
	PersistFailure ErrorKind = "applied-state persistence failed"
)

type ServiceError struct {
	Kind ErrorKind
	Err  error
}

func (e *ServiceError) Error() string { return fmt.Sprintf("%s: %v", e.Kind, e.Err) }
func (e *ServiceError) Unwrap() error { return e.Err }
func IsErrorKind(err error, kind ErrorKind) bool {
	var target *ServiceError
	return errors.As(err, &target) && target.Kind == kind
}

type Status struct {
	Connection       Connection
	BatteryPercent   int
	BatteryAvailable bool
}
type Service struct {
	transport PassiveInputTransport
	command   CommandTransport
	commandMu sync.Mutex
}

func NewService(transport PassiveInputTransport) Service  { return Service{transport: transport} }
func NewCommandService(command CommandTransport) *Service { return &Service{command: command} }

// NewDesktopServices keeps passive status and explicit command operations on one adapter.
func NewDesktopServices(adapter interface {
	PassiveInputTransport
	CommandTransport
}) (Service, *Service) {
	return NewService(adapter), NewCommandService(adapter)
}

func (s *Service) ApplyDPI(ctx context.Context, config DPIConfig) error {
	return s.applyDPI(ctx, config, nil)
}
func (s *Service) ApplyAndPersist(ctx context.Context, config DPIConfig, store AppliedDPIStore) error {
	return s.applyDPI(ctx, config, store)
}
func (s *Service) applyDPI(ctx context.Context, config DPIConfig, store AppliedDPIStore) error {
	report, err := EncodeDPIReport(config)
	if err != nil {
		return err
	}
	if s.command == nil {
		return &ServiceError{WriteFailure, errors.New("command transport is unavailable")}
	}
	s.commandMu.Lock()
	defer s.commandMu.Unlock()
	matched := false
	err = s.command.SendAndAwait(ctx, report, func(input []byte) bool { matched = matchesDPIACK(input); return !matched })
	if err != nil {
		return &ServiceError{AckFailure, err}
	}
	if !matched {
		return &ServiceError{AckFailure, errors.New("matching DPI acknowledgement was not received")}
	}
	if store != nil {
		if err := store.SaveApplied(config); err != nil {
			return &ServiceError{PersistFailure, err}
		}
	}
	return nil
}
func (s Service) Status(ctx context.Context) (Status, error) {
	if s.transport == nil {
		return Status{}, &ServiceError{NoUsableDevice, errors.New("passive input transport is unavailable")}
	}
	candidates, err := s.transport.Enumerate(ctx, transport.X6Match())
	if err != nil {
		return Status{}, &ServiceError{NoUsableDevice, err}
	}
	var readErr error
	for _, connection := range []Connection{Dongle, Wired} {
		for _, candidate := range candidates {
			if candidate.Connection != connection {
				continue
			}
			source, err := s.transport.ValidateDescriptor(ctx, candidate, InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83})
			if err != nil {
				continue
			}
			status := Status{Connection: connection}
			err = s.transport.ReadInterruptIN(ctx, source, func(report []byte) bool {
				if battery, available := protocol.DecodeBatteryStatus(report); available {
					status.BatteryPercent, status.BatteryAvailable = battery, true
					return false
				}
				return true
			})
			if err == nil {
				return status, nil
			}
			readErr = err
		}
	}
	if readErr != nil {
		return Status{}, &ServiceError{ReadFailure, readErr}
	}
	return Status{}, &ServiceError{NoUsableDevice, errors.New("no validated passive input path")}
}
