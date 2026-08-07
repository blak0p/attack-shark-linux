//go:build hardware

package x6_test

import (
	"context"
	"os"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func hardwareEnabled(short bool, gate string) bool { return !short && gate == "1" }

func TestHardwarePassiveStatus(t *testing.T) {
	if !hardwareEnabled(testing.Short(), os.Getenv("ATTACK_SHARK_X6_HARDWARE")) {
		t.Skip("passive hardware test not enabled")
	}
	transport, err := hidlinux.NewPassiveTransport()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x6.NewService(transport).Status(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHardwareGate(t *testing.T) {
	for _, tt := range []struct {
		short bool
		gate  string
		want  bool
	}{{true, "1", false}, {false, "", false}, {false, "1", true}} {
		if got := hardwareEnabled(tt.short, tt.gate); got != tt.want {
			t.Fatalf("hardwareEnabled(%t, %q) = %t", tt.short, tt.gate, got)
		}
	}
}
