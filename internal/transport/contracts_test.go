package transport

import (
	"context"
	"errors"
	"testing"
)

func TestX6MatchUsesCapturedDeviceIdentity(t *testing.T) {
	match := X6Match()
	if match.VendorID != 0x1D57 || match.ProductID != 0xFA60 {
		t.Fatalf("X6Match() = %#v, want Attack Shark X6 identity", match)
	}
}

func TestGenericContractsRepresentPassiveAndCommandTransport(t *testing.T) {
	var _ PassiveInputTransport = passiveTransportStub{}
	var _ CommandTransport = commandTransportStub{}

	candidate := Candidate{Path: "/dev/hidraw0", Connection: Dongle}
	descriptor := InputDescriptor{InterfaceNumber: 2, UsagePage: 1, Usage: 0x80, EndpointAddress: 0x83}
	if candidate.Connection != Dongle || descriptor.EndpointAddress != 0x83 {
		t.Fatalf("generic transport values = %#v, %#v", candidate, descriptor)
	}
}

func TestCandidateCarriesStableSerialAlongsideTransientPath(t *testing.T) {
	candidate := Candidate{Path: "/dev/hidraw0", VendorID: 0x1D57, ProductID: 0xFA60, Serial: "X6-001"}
	if candidate.Serial != "X6-001" {
		t.Fatalf("Candidate.Serial = %q, want stable device serial", candidate.Serial)
	}
}

func TestTopologyEvidenceRejectsNonCanonicalUSBPaths(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
		want error
	}{
		{name: "canonical root port", path: "1-4"},
		{name: "canonical downstream port", path: "1-3.7"},
		{name: "missing bus", path: "-4", want: ErrInvalidTopology},
		{name: "zero port", path: "1-0", want: ErrInvalidTopology},
		{name: "malformed separator", path: "1-4..2", want: ErrInvalidTopology},
		{name: "bus mismatch", path: "2-4", want: ErrInvalidTopology},
	} {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := ParseTopology(1, tt.path)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ParseTopology(1, %q) error = %v, want %v", tt.path, err, tt.want)
			}
			if tt.want == nil && (topology.Bus == 0 || len(topology.Ports) == 0 || topology.String() != tt.path) {
				t.Fatalf("ParseTopology(1, %q) = %#v, want canonical topology", tt.path, topology)
			}
		})
	}
}

func TestEpochTrackerRotatesOnlyWhenOccurrenceChanges(t *testing.T) {
	tracker := NewEpochTracker()
	candidate := Candidate{Path: "1:1-4", Topology: TopologyEvidence{Bus: 1, Ports: []uint8{4}}}

	first := tracker.Refresh([]Candidate{candidate})[0]
	stable := tracker.Refresh([]Candidate{candidate})[0]
	tracker.Refresh(nil)
	reconnected := tracker.Refresh([]Candidate{candidate})[0]

	if first.DeviceKey == "" || first.EpochID == 0 || first.Occurrence == 0 {
		t.Fatalf("first occurrence = %#v, want runtime key and epoch", first)
	}
	if stable.DeviceKey != first.DeviceKey || stable.EpochID != first.EpochID || stable.Occurrence != first.Occurrence {
		t.Fatalf("stable refresh = %#v, want %#v", stable, first)
	}
	if reconnected.DeviceKey != first.DeviceKey || reconnected.EpochID == first.EpochID || reconnected.Occurrence == first.Occurrence {
		t.Fatalf("reconnected occurrence = %#v, want rotated epoch from %#v", reconnected, first)
	}
}

type passiveTransportStub struct{}

func (passiveTransportStub) Enumerate(context.Context, Match) ([]Candidate, error) { return nil, nil }
func (passiveTransportStub) ValidateDescriptor(context.Context, Candidate, InputDescriptor) (InputSource, error) {
	return InputSource{}, nil
}
func (passiveTransportStub) ReadInterruptIN(context.Context, InputSource, func([]byte) bool) error {
	return nil
}

type commandTransportStub struct{}

func (commandTransportStub) SendAndAwait(context.Context, []byte, func([]byte) bool) error {
	return nil
}
