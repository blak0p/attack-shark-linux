package x6

import (
	"bytes"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestProfilePreservesX6ValidationAndCodecParity(t *testing.T) {
	profile := NewProfile()
	config := DefaultDPIConfig()
	config.DPI[0] = 1600

	if err := profile.Codec().Validate(config); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	got, err := profile.Codec().Encode(config)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	want, err := EncodeDPIReport(config)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("Encode() = % x, %v; want X6 wrapper output % x, %v", got, err, want, err)
	}

	if !profile.Codec().MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x04}) {
		t.Fatal("MatchesACK() rejected the X6 configuration acknowledgement")
	}
	if profile.Codec().MatchesACK([]byte{0x03, 0x10, 0x50, 0x00, 0x05}) {
		t.Fatal("MatchesACK() accepted a non-matching acknowledgement")
	}
}

func TestProfilePreservesX6HIDFactsDefaultsAndStatusDecoding(t *testing.T) {
	profile := NewProfile()
	facts := profile.HIDFacts()
	if facts.StatusInput.InterfaceNumber != 2 || facts.StatusInput.UsagePage != 1 || facts.StatusInput.Usage != 0x80 || facts.StatusInput.EndpointAddress != 0x83 || facts.ConfigurationReportSize != protocol.DPIReportLength {
		t.Fatalf("HIDFacts() = %#v; want validated X6 HID facts", facts)
	}
	if got, ok := profile.Codec().Defaults().(DPIConfig); !ok || got != DefaultDPIConfig() {
		t.Fatalf("Defaults() = %#v, %t; want X6 defaults", got, ok)
	}
	got, ok := profile.Codec().DecodeStatus([]byte{0x03, 0x10, 0x40, 0x00, 0x07})
	want, wantOK := protocol.DecodeStatusReport([]byte{0x03, 0x10, 0x40, 0x00, 0x07})
	if !ok || !wantOK || got != want {
		t.Fatalf("DecodeStatus() = %#v, %t; want %#v, %t", got, ok, want, wantOK)
	}
}

func TestProfileRejectsInvalidOrForeignConfiguration(t *testing.T) {
	profile := NewProfile()
	invalid := DefaultDPIConfig()
	invalid.DPI[0] = 55

	if err := profile.Codec().Validate(invalid); err == nil {
		t.Fatal("Validate() accepted an invalid X6 DPI configuration")
	}
	if err := profile.Codec().Validate(struct{}{}); err == nil {
		t.Fatal("Validate() accepted a configuration from another profile")
	}
	if _, ok := profile.(mouse.Profile); !ok {
		t.Fatal("NewProfile() does not expose the generic mouse profile contract")
	}
}

func TestProfilesOwnX6USBIdentityAndPairing(t *testing.T) {
	registry, err := mouse.NewProfileRegistry(NewProfile(), NewWiredProfile())
	if err != nil {
		t.Fatal(err)
	}
	dongle, wired := NewProfile(), NewWiredProfile()
	if dongle.Match() != (transport.Match{VendorID: 0x1D57, ProductID: 0xFA60}) || dongle.HIDFacts().Connection != transport.Dongle {
		t.Fatalf("dongle profile = %v / %v", dongle.Match(), dongle.HIDFacts().Connection)
	}
	if wired.Match() != (transport.Match{VendorID: 0x1D57, ProductID: 0xFA61}) || wired.HIDFacts().Connection != transport.Wired {
		t.Fatalf("wired profile = %v / %v", wired.Match(), wired.HIDFacts().Connection)
	}
	if wired.HIDFacts().StatusInput != dongle.HIDFacts().StatusInput {
		t.Fatalf("wired and dongle must share status input facts")
	}
	pairs := registry.WiredPairs()
	if len(pairs) != 1 || pairs[0].Wired != wired.Match() || pairs[0].Dongle != dongle.Match() {
		t.Fatalf("WiredPairs() = %v", pairs)
	}
}
