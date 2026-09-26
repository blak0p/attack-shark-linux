package desktop

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestNormalSleepAndDebounceClassifyDeviceAccessFailures(t *testing.T) {
	for _, tt := range []struct {
		name  string
		err   error
		apply func(*Service) ErrorCode
		want  ErrorCode
	}{
		{name: "normal sleep permission denied", err: os.ErrPermission, apply: func(s *Service) ErrorCode { return s.ApplyNormalSleep().Error.Code }, want: PermissionDenied},
		{name: "normal sleep wrapped disconnected", err: fmt.Errorf("write: %w", os.ErrNotExist), apply: func(s *Service) ErrorCode { return s.ApplyNormalSleep().Error.Code }, want: DeviceDisconnected},
		{name: "debounce permission denied", err: os.ErrPermission, apply: func(s *Service) ErrorCode { return s.ApplyDebounce().Error.Code }, want: PermissionDenied},
		{name: "debounce wrapped disconnected", err: fmt.Errorf("write: %w", os.ErrNotExist), apply: func(s *Service) ErrorCode { return s.ApplyDebounce().Error.Code }, want: DeviceDisconnected},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatal(err)
			}
			candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/hidraw0"}
			service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, &fakeHidrawCommand{err: tt.err}))
			service.RefreshInventory(context.Background())

			if got := tt.apply(service); got != tt.want {
				t.Fatalf("apply error code = %q, want %q", got, tt.want)
			}
		})
	}
}
