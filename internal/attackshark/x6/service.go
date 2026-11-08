package x6

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"context"
	"errors"
	"sync"
	"time"

	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type ErrorKind = device.ErrorKind

const (
	NoUsableDevice = device.NoUsableDevice
	ReadFailure    = device.ReadFailure
	InvalidDPI     = device.InvalidDPI
	WriteFailure   = device.WriteFailure
	AckFailure     = device.AckFailure
	PersistFailure = device.PersistFailure
)

type ServiceError = device.ServiceError

func IsErrorKind(err error, kind ErrorKind) bool { return device.IsErrorKind(err, kind) }

type Status = device.Status

// StatusEvent is one dongle-pushed status report delivered by Listen.
type StatusEvent = device.StatusEvent

type Service struct {
	transport PassiveInputTransport
	command   CommandTransport
	commandMu sync.Mutex
}

func NewService(transport PassiveInputTransport) *Service { return &Service{transport: transport} }
func NewCommandService(command CommandTransport) *Service { return &Service{command: command} }

// NewDesktopServices keeps passive status and explicit command operations on one adapter.
func NewDesktopServices(adapter interface {
	PassiveInputTransport
	CommandTransport
}) (*Service, *Service) {
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
		return &ServiceError{Kind: WriteFailure, Err: errors.New("command transport is unavailable")}
	}
	s.commandMu.Lock()
	defer s.commandMu.Unlock()
	matched := false
	err = s.command.SendAndAwait(ctx, report, func(input []byte) bool { matched = matchesDPIACK(input); return !matched })
	if err != nil {
		return &ServiceError{Kind: AckFailure, Err: err}
	}
	if !matched {
		return &ServiceError{Kind: AckFailure, Err: errors.New("matching DPI acknowledgement was not received")}
	}
	if store != nil {
		if err := store.SaveApplied(config); err != nil {
			return &ServiceError{Kind: PersistFailure, Err: err}
		}
	}
	return nil
}
func (s *Service) Status(ctx context.Context) (Status, error) {
	if s.transport == nil {
		return Status{}, &ServiceError{Kind: NoUsableDevice, Err: errors.New("passive input transport is unavailable")}
	}
	candidates, err := transport.EnumerateAll(ctx, s.transport, dongleMatch, wiredMatch)
	if err != nil {
		return Status{}, &ServiceError{Kind: NoUsableDevice, Err: err}
	}
	candidates = s.preferredCandidates(ctx, candidates)
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
		return Status{}, &ServiceError{Kind: ReadFailure, Err: readErr}
	}
	return Status{}, &ServiceError{Kind: NoUsableDevice, Err: errors.New("no validated passive input path")}
}

// listenRetryDelay bounds the retry cadence when no device answers. The dongle
// pushes its heartbeat every ~2.1 s, so 500 ms is short enough to react the
// moment a device appears without spinning on an absent dongle.
const listenRetryDelay = 500 * time.Millisecond

// Listen runs until ctx is cancelled. It keeps an interrupt 0x83 read armed on
// the validated device and forwards every dongle-pushed status report through
// onStatus: heartbeats (battery) and physical DPI button events (active stage).
// The callback never stops the read, so a bounded timeout mid-idle is treated
// as "keep listening" and only a missing/unusable device backs off before
// re-enumerating.
func (s *Service) Listen(ctx context.Context, onStatus func(StatusEvent)) error {
	if s.transport == nil {
		return &ServiceError{Kind: NoUsableDevice, Err: errors.New("passive input transport is unavailable")}
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		candidates, err := transport.EnumerateAll(ctx, s.transport, dongleMatch, wiredMatch)
		if err != nil {
			if !waitOrDone(ctx, listenRetryDelay) {
				return nil
			}
			continue
		}
		candidates = s.preferredCandidates(ctx, candidates)
		if len(candidates) > 1 {
			if !waitOrDone(ctx, listenRetryDelay) {
				return nil
			}
			continue
		}
		listened := false
		for _, connection := range []Connection{Dongle, Wired} {
			for _, candidate := range candidates {
				if candidate.Connection != connection {
					continue
				}
				source, err := s.transport.ValidateDescriptor(ctx, candidate, InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83})
				if err != nil {
					continue
				}
				listened = true
				err = s.transport.ReadInterruptIN(ctx, source, func(report []byte) bool {
					decoded, ok := protocol.DecodeStatusReport(report)
					if ok {
						onStatus(StatusEvent{
							Connection:       connection,
							BatteryPercent:   decoded.Battery,
							BatteryAvailable: decoded.BatteryAvailable,
							ActiveStage:      decoded.ActiveStage,
							StageAvailable:   decoded.StageAvailable,
						})
					}
					return true
				})
				if err != nil {
					if !waitOrDone(ctx, listenRetryDelay) {
						return nil
					}
					break
				}
			}
		}
		if !listened && !waitOrDone(ctx, listenRetryDelay) {
			return nil
		}
	}
}

// preferredCandidates chooses a connection only after descriptor admission.
func (s *Service) preferredCandidates(ctx context.Context, candidates []transport.Candidate) []transport.Candidate {
	eligible := make([]transport.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if _, err := s.transport.ValidateDescriptor(ctx, c, InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}); err == nil {
			eligible = append(eligible, c)
		}
	}
	return transport.PreferWired(eligible, transport.WiredPair{Wired: wiredMatch, Dongle: dongleMatch})
}

// waitOrDone sleeps for the given delay unless ctx is cancelled first. It
// returns false when ctx was cancelled, which signals the caller to stop.
func waitOrDone(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
