package hidlinux

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCommandAdapterWritesOnceAndFiltersUntilMatchingACK(t *testing.T) {
	device := &commandDeviceFake{reports: [][]byte{{0x03, 0x10, 0x40, 1, 10}, {0x03, 0x10, 0x50, 0, 4}}}
	accepted := []byte(nil)
	err := newCommandAdapter(&commandBackendFake{device: device}).SendAndAwait(context.Background(), make([]byte, 52), func(report []byte) bool {
		accepted = append([]byte(nil), report...)
		return report[2] != 0x50
	})
	if err != nil {
		t.Fatalf("SendAndAwait() error = %v", err)
	}
	if device.writes != 1 || !reflect.DeepEqual(accepted, []byte{0x03, 0x10, 0x50, 0, 4}) || !reflect.DeepEqual(device.calls, []string{"open", "write", "read", "read", "close"}) {
		t.Fatalf("writes=%d accepted=%x calls=%v", device.writes, accepted, device.calls)
	}
}

type commandDeviceFake struct {
	reports [][]byte
	calls   []string
	writes  int
}

func (d *commandDeviceFake) SendFeatureReport([]byte) (int, error) {
	d.calls = append(d.calls, "write")
	d.writes++
	return 52, nil
}
func (d *commandDeviceFake) ReadWithTimeout(p []byte, _ time.Duration) (int, error) {
	d.calls = append(d.calls, "read")
	report := d.reports[0]
	d.reports = d.reports[1:]
	copy(p, report)
	return len(report), nil
}
func (d *commandDeviceFake) Close() error { d.calls = append(d.calls, "close"); return nil }

type commandBackendFake struct{ device *commandDeviceFake }

func (b *commandBackendFake) OpenPath(string) (commandDevice, error) {
	b.device.calls = append(b.device.calls, "open")
	return b.device, nil
}
