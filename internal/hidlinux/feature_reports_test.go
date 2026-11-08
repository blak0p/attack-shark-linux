//go:build linux

package hidlinux

import (
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

func TestSupportedFeatureReportComesFromBoundProfileFacts(t *testing.T) {
	backend := &HidrawBackend{registry: fixtureRegistry(t)}
	bound := mouse.Binding{ID: mouse.DeviceID{VendorID: fixtureVendorID, ProductID: fixtureDongleProduct}}
	unknown := mouse.Binding{ID: mouse.DeviceID{VendorID: fixtureVendorID, ProductID: 0xfa62}}
	for _, tt := range []struct {
		name    string
		binding mouse.Binding
		payload []byte
		want    bool
	}{
		{"declared config report", bound, make([]byte, fixtureConfigLength), true},
		{"declared ID wrong length", bound, make([]byte, fixtureConfigLength-1), false},
		{"unregistered binding", unknown, make([]byte, fixtureConfigLength), false},
	} {
		if tt.want {
			tt.payload[0] = fixtureConfigID
		}
		if got := backend.supportedFeatureReport(tt.binding, tt.payload); got != tt.want {
			t.Fatalf("%s: got %t, want %t", tt.name, got, tt.want)
		}
	}
}
