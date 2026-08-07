package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"github.com/alejandro/attack-shark-linux/internal/x6"
	"time"
)

const interruptReadTimeout = 2 * time.Second
const passiveReadDeadline = 15 * time.Second

type deviceInfo struct {
	path                                  string
	vendorID, productID, usagePage, usage uint16
	interfaceNumber                       int
}
type inputDevice interface {
	ReadWithTimeout([]byte, time.Duration) (int, error)
	Close() error
}
type passiveBackend interface {
	Enumerate(uint16, uint16, func(deviceInfo) error) error
	OpenPath(string) (inputDevice, error)
}
type passiveAdapter struct {
	passiveBackend
	now func() time.Time
}

func newPassiveAdapter(backend passiveBackend) x6.PassiveInputTransport {
	return passiveAdapter{passiveBackend: backend, now: time.Now}
}
func (a passiveAdapter) Enumerate(_ context.Context, match x6.Match) ([]x6.Candidate, error) {
	var candidates []x6.Candidate
	err := a.passiveBackend.Enumerate(match.VendorID, match.ProductID, func(info deviceInfo) error {
		candidates = append(candidates, x6.Candidate{Path: info.path, VendorID: info.vendorID, ProductID: info.productID, UsagePage: info.usagePage, Usage: info.usage, InterfaceNumber: info.interfaceNumber, Connection: x6.Dongle})
		return nil
	})
	return candidates, err
}
func (a passiveAdapter) ValidateDescriptor(_ context.Context, candidate x6.Candidate, descriptor x6.InputDescriptor) (x6.InputSource, error) {
	if candidate.Path == "" || candidate.InterfaceNumber != 2 || candidate.UsagePage != 1 || candidate.Usage != 0x80 || descriptor.InterfaceNumber != 2 || descriptor.UsagePage != 1 || descriptor.Usage != 0x80 || descriptor.EndpointAddress != 0x83 {
		return x6.InputSource{}, errors.New("unsupported HID input descriptor")
	}
	return x6.InputSource{Path: candidate.Path}, nil
}
func (a passiveAdapter) ReadInterruptIN(ctx context.Context, source x6.InputSource, accept func([]byte) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	device, err := a.passiveBackend.OpenPath(source.Path)
	if err != nil {
		return err
	}
	defer device.Close()
	report := make([]byte, 64)
	deadline := a.now().Add(passiveReadDeadline)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := deadline.Sub(a.now())
		if remaining <= 0 {
			return fmt.Errorf("passive input read deadline exceeded after %s", passiveReadDeadline)
		}
		readTimeout := interruptReadTimeout
		if remaining < readTimeout {
			readTimeout = remaining
		}
		n, err := device.ReadWithTimeout(report, readTimeout)
		if err != nil {
			if !isPassiveReadTimeout(err) {
				return err
			}
			if deadline.Sub(a.now()) <= 0 {
				return fmt.Errorf("passive input read deadline exceeded after %s: %w", passiveReadDeadline, err)
			}
			continue
		}
		if n > 0 && !accept(report[:n]) {
			return nil
		}
	}
}

func isPassiveReadTimeout(err error) bool {
	return err != nil && err.Error() == "timeout"
}
