package hidlinux

import (
	"context"
	"fmt"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type commandDevice interface {
	SendFeatureReport([]byte) (int, error)
	ReadWithTimeout([]byte, time.Duration) (int, error)
	Close() error
}
type commandBackend interface {
	OpenPath(string) (commandDevice, error)
}
type commandAdapter struct {
	backend commandBackend
	path    string
}

func newCommandAdapter(backend commandBackend) x6.CommandTransport {
	return commandAdapter{backend: backend}
}
func newCommandAdapterAtPath(backend commandBackend, path string) x6.CommandTransport {
	return commandAdapter{backend: backend, path: path}
}

func (a commandAdapter) SendAndAwait(ctx context.Context, report []byte, accept func([]byte) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	device, err := a.backend.OpenPath(a.path)
	if err != nil {
		return err
	}
	defer device.Close()
	if n, err := device.SendFeatureReport(report); err != nil || n != len(report) {
		if err != nil {
			return err
		}
		return fmt.Errorf("feature report write was %d bytes, want %d", n, len(report))
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		input := make([]byte, 64)
		n, err := device.ReadWithTimeout(input, interruptReadTimeout)
		if err != nil {
			return err
		}
		if n > 0 && !accept(input[:n]) {
			return nil
		}
	}
}
