//go:build gousb_prerequisite

package hidlinux

import (
	"testing"

	"github.com/google/gousb"
)

func TestGousbFoundationUsesPinnedModule(t *testing.T) {
	if got, want := uint16(gousb.ID(0x1D57)), uint16(0x1D57); got != want {
		t.Fatalf("gousb.ID() = %v, want %v", got, want)
	}
}
