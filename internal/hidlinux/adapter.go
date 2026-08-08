// Package hidlinux provides a validated Linux HID adapter with injectable seams.
package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

const (
	x6VendorID     = 0x1D57
	x6ProductID    = 0xFA60
	hidClass       = 0x03
	statusEndpoint = 0x83
	// statusReadDeadline bounds each passive read. The X6 dongle emits its
	// battery report periodically roughly every 1.5 s, so a deadline of 3 s
	// (two report periods) guarantees the read catches the next report even
	// when it starts immediately after one was consumed. A shorter deadline
	// races the report cadence and fails with status_read_failed.
	statusReadDeadline   = 3 * time.Second
	dpiReportLength      = 52
	setReportRequestType = 0x21
	setReportRequest     = 0x09
	dpiReportValue       = 0x0304
	hidInterface         = 2
)

var (
	ErrDeviceDisconnected = errors.New("X6 USB device disconnected")
	ErrCandidateMismatch  = &Error{Kind: Mismatch, Err: errors.New("X6 USB candidate does not match the validated path")}
)

type ErrorKind string

const (
	NotFound     ErrorKind = "not found"
	Mismatch     ErrorKind = "mismatch"
	Permission   ErrorKind = "permission denied"
	Timeout      ErrorKind = "timeout"
	Cancelled    ErrorKind = "cancelled"
	Disconnected ErrorKind = "disconnected"
	IO           ErrorKind = "I/O"
)

type Error struct {
	Kind ErrorKind
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("X6 HID %s: %v", e.Kind, e.Err) }
func (e *Error) Unwrap() error { return e.Err }
func (e *Error) Is(target error) bool {
	var typed *Error
	return errors.As(target, &typed) && e.Kind == typed.Kind
}

func IsErrorKind(err error, kind ErrorKind) bool {
	var target *Error
	return errors.As(err, &target) && target.Kind == kind
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	var typed *Error
	if errors.As(err, &typed) {
		return err
	}
	switch {
	case errors.Is(err, os.ErrPermission):
		return &Error{Kind: Permission, Err: err}
	case errors.Is(err, context.DeadlineExceeded):
		return &Error{Kind: Timeout, Err: err}
	case errors.Is(err, context.Canceled):
		return &Error{Kind: Cancelled, Err: err}
	case errors.Is(err, ErrDeviceDisconnected):
		return &Error{Kind: Disconnected, Err: err}
	default:
		return &Error{Kind: IO, Err: err}
	}
}

// Candidate is descriptor metadata supplied by an injected discovery implementation.
type Candidate struct {
	VendorID   uint16
	ProductID  uint16
	Bus        uint8
	PortPath   string
	Interfaces []InterfaceDescriptor
}

// InterfaceDescriptor is the descriptor-only information needed before a claim.
type InterfaceDescriptor struct {
	Number, AlternateSetting int
	Class                    uint8
	Endpoints                []uint8
}

// Discovery lists metadata only; it must not open, claim, control, or read a device.
type Discovery interface {
	Enumerate(context.Context) ([]Candidate, error)
}

// USB is the narrow seam for opening a previously validated candidate.
type USB interface {
	Open(context.Context, Candidate) (Device, error)
}

// Device owns the configuration selected for one operation.
type Device interface {
	OpenConfiguration(context.Context) (Configuration, error)
	Close() error
}

// Configuration owns the claimed HID interface.
type Configuration interface {
	Claim(context.Context, int, int) (Claim, error)
	Close() error
}

// Claim releases the claimed interface.
type Claim interface {
	ControlTransfer(context.Context, uint8, uint8, uint16, uint16, []byte) (int, error)
	ReadInterruptIN(context.Context, uint8, func([]byte) bool) error
	Close() error
}

// Sysfs reads the read-only HID report descriptor associated with a candidate.
type Sysfs interface {
	ReportDescriptor(context.Context, Candidate) ([]byte, error)
}

// Adapter serializes passive USB status operations and validates before opening USB.
type Adapter struct {
	mu        sync.Mutex
	discovery Discovery
	usb       USB
	sysfs     Sysfs
	sources   map[string]Candidate
}

func NewAdapter(usb USB, sysfs Sysfs) *Adapter {
	return &Adapter{usb: usb, sysfs: sysfs, sources: make(map[string]Candidate)}
}

// NewStatusAdapter is passive: construction is inert and I/O begins only through Status.
func NewStatusAdapter(discovery Discovery, usb USB, sysfs Sysfs) *Adapter {
	adapter := NewAdapter(usb, sysfs)
	adapter.discovery = discovery
	return adapter
}

// WithValidatedCandidate claims interface 2/alternate setting 0 for a validated candidate.
// Production and test backends share the same validated seam contract.
func (a *Adapter) WithValidatedCandidate(ctx context.Context, candidate Candidate, use func(Claim) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.validateCandidate(ctx, candidate); err != nil {
		return err
	}

	device, err := a.usb.Open(ctx, candidate)
	if err != nil {
		return classify(err)
	}
	defer device.Close()

	configuration, err := device.OpenConfiguration(ctx)
	if err != nil {
		return classify(err)
	}
	defer configuration.Close()

	claim, err := configuration.Claim(ctx, 2, 0)
	if err != nil {
		return classify(err)
	}
	defer claim.Close()

	return classify(use(claim))
}

// Enumerate implements transport.PassiveInputTransport without accessing USB.
func (a *Adapter) Enumerate(ctx context.Context, match transport.Match) ([]transport.Candidate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.discovery == nil {
		return nil, &Error{Kind: NotFound, Err: errors.New("USB discovery is unavailable")}
	}
	candidates, err := a.discovery.Enumerate(ctx)
	if err != nil {
		return nil, classify(err)
	}
	result := make([]transport.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.VendorID != match.VendorID || candidate.ProductID != match.ProductID || !matchesX6Identity(candidate) {
			continue
		}
		key := candidateKey(candidate)
		if key == "" {
			key = "invalid physical path"
			a.sources[key] = candidate
			result = append(result, transport.Candidate{Path: key, VendorID: candidate.VendorID, ProductID: candidate.ProductID, Connection: transport.Dongle})
			continue
		}
		a.sources[key] = candidate
		result = append(result, transport.Candidate{Path: key, VendorID: candidate.VendorID, ProductID: candidate.ProductID, Connection: transport.Dongle})
	}
	if len(result) == 0 {
		return nil, &Error{Kind: NotFound, Err: errors.New("validated X6 VID:PID was not found")}
	}
	return result, nil
}

// ValidateDescriptor checks every identity field before any USB open or claim.
func (a *Adapter) ValidateDescriptor(ctx context.Context, source transport.Candidate, wanted transport.InputDescriptor) (transport.InputSource, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	candidate, ok := a.sources[source.Path]
	if !ok {
		return transport.InputSource{}, &Error{Kind: NotFound, Err: errors.New("discovered device is no longer present")}
	}
	if wanted.InterfaceNumber != 2 || wanted.UsagePage != 1 || wanted.Usage != 0x80 || wanted.EndpointAddress != statusEndpoint {
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("unsupported status descriptor request")}
	}
	if err := a.validateCandidate(ctx, candidate); err != nil {
		return transport.InputSource{}, err
	}
	return transport.InputSource{Path: source.Path}, nil
}

// ReadInterruptIN performs one bounded, passive status read on a validated source.
func (a *Adapter) ReadInterruptIN(ctx context.Context, source transport.InputSource, use func([]byte) bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	candidate, ok := a.sources[source.Path]
	if !ok {
		return &Error{Kind: NotFound, Err: errors.New("validated device is no longer present")}
	}
	bounded, cancel := context.WithTimeout(ctx, statusReadDeadline)
	defer cancel()
	return a.withValidatedCandidate(bounded, candidate, func(claim Claim) error {
		return claim.ReadInterruptIN(bounded, statusEndpoint, use)
	})
}

// SendAndAwait implements transport.CommandTransport for an explicit DPI Apply.
// Claim.ReadInterruptIN continues while its callback returns true.
func (a *Adapter) SendAndAwait(ctx context.Context, payload []byte, continueReading func([]byte) bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(payload) != dpiReportLength {
		return &Error{Kind: Mismatch, Err: fmt.Errorf("DPI SET_REPORT length = %d, want %d", len(payload), dpiReportLength)}
	}
	candidate, err := a.commandCandidate()
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, statusReadDeadline)
	defer cancel()
	return a.withValidatedCandidate(bounded, candidate, func(claim Claim) error {
		count, err := claim.ControlTransfer(bounded, setReportRequestType, setReportRequest, dpiReportValue, hidInterface, payload)
		if err != nil {
			return err
		}
		if count != len(payload) {
			return fmt.Errorf("DPI SET_REPORT wrote %d bytes, want %d", count, len(payload))
		}
		return claim.ReadInterruptIN(bounded, statusEndpoint, continueReading)
	})
}

func (a *Adapter) commandCandidate() (Candidate, error) {
	if len(a.sources) == 0 {
		return Candidate{}, &Error{Kind: NotFound, Err: errors.New("no validated device is available for DPI Apply")}
	}
	if len(a.sources) != 1 {
		return Candidate{}, &Error{Kind: Mismatch, Err: errors.New("multiple validated devices are available for DPI Apply")}
	}
	for _, candidate := range a.sources {
		return candidate, nil
	}
	panic("unreachable")
}

func (a *Adapter) withValidatedCandidate(ctx context.Context, candidate Candidate, use func(Claim) error) error {
	if err := a.validateCandidate(ctx, candidate); err != nil {
		return err
	}
	device, err := a.usb.Open(ctx, candidate)
	if err != nil {
		return classify(err)
	}
	defer device.Close()
	configuration, err := device.OpenConfiguration(ctx)
	if err != nil {
		return classify(err)
	}
	defer configuration.Close()
	claim, err := configuration.Claim(ctx, 2, 0)
	if err != nil {
		return classify(err)
	}
	defer claim.Close()
	return classify(use(claim))
}

func (a *Adapter) validateCandidate(ctx context.Context, candidate Candidate) error {
	if !matchesX6Identity(candidate) || !hasPhysicalPath(candidate) || !hasValidatedInterface(candidate.Interfaces) {
		return &Error{Kind: Mismatch, Err: errors.New("candidate does not match the validated X6 path")}
	}
	if a.sysfs == nil {
		return &Error{Kind: IO, Err: errors.New("sysfs descriptor reader is unavailable")}
	}
	descriptor, err := a.sysfs.ReportDescriptor(ctx, candidate)
	if err != nil {
		return classify(err)
	}
	if !hasX6TopLevelUsage(descriptor) {
		return &Error{Kind: Mismatch, Err: errors.New("HID report descriptor does not expose usage 1/0x80")}
	}
	return nil
}

func matchesX6Identity(candidate Candidate) bool {
	return candidate.VendorID == x6VendorID && candidate.ProductID == x6ProductID
}

func hasPhysicalPath(candidate Candidate) bool {
	return candidateKey(candidate) != ""
}

func candidateKey(candidate Candidate) string {
	if candidate.Bus == 0 || candidate.PortPath == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", candidate.Bus, candidate.PortPath)
}

func hasValidatedInterface(interfaces []InterfaceDescriptor) bool {
	for _, descriptor := range interfaces {
		if descriptor.Number != 2 || descriptor.AlternateSetting != 0 || descriptor.Class != hidClass {
			continue
		}
		for _, endpoint := range descriptor.Endpoints {
			if endpoint == statusEndpoint {
				return true
			}
		}
	}
	return false
}

func hasX6TopLevelUsage(descriptor []byte) bool {
	return bytes.Contains(descriptor, []byte{0x05, 0x01, 0x09, 0x80})
}
