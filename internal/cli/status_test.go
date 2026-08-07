package cli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

type statusFunc func(context.Context) (x6.Status, error)

func (f statusFunc) Status(ctx context.Context) (x6.Status, error) { return f(ctx) }

func TestRun(t *testing.T) {
	tests := []struct {
		name, args, want string
		service          statusFunc
		code             int
	}{
		{"available dongle", "status", "status: connected\nconnection: dongle\nbattery: 70%\ndpi: unavailable\nrgb: unavailable\nfirmware: unavailable\n", func(context.Context) (x6.Status, error) {
			return x6.Status{Connection: x6.Dongle, BatteryPercent: 70, BatteryAvailable: true}, nil
		}, 0},
		{"unavailable fields", "status", "status: connected\nconnection: wired\nbattery: unavailable\ndpi: unavailable\nrgb: unavailable\nfirmware: unavailable\n", func(context.Context) (x6.Status, error) { return x6.Status{Connection: x6.Wired}, nil }, 0},
		{"not found", "status", "no usable X6 device\n", func(context.Context) (x6.Status, error) {
			return x6.Status{}, &x6.ServiceError{Kind: x6.NoUsableDevice, Err: errors.New("absent")}
		}, 3},
		{"permission", "status", "permission denied\n", func(context.Context) (x6.Status, error) { return x6.Status{}, os.ErrPermission }, 4},
		{"read failure", "status", "input read failed\n", func(context.Context) (x6.Status, error) {
			return x6.Status{}, &x6.ServiceError{Kind: x6.ReadFailure, Err: errors.New("timeout")}
		}, 5},
		{"usage", "", "usage: attack-shark-linux status\n", nil, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			args := strings.Fields(tt.args)
			want := tt.want
			if tt.name == "available dongle" {
				data, err := os.ReadFile("testdata/status.golden")
				if err != nil {
					t.Fatal(err)
				}
				want = string(data)
			}
			if got := Run(context.Background(), args, tt.service, &output); got != tt.code || output.String() != want {
				t.Fatalf("Run() = (%d, %q), want (%d, %q)", got, output.String(), tt.code, want)
			}
		})
	}
}
