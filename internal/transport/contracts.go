// Package transport owns generic transport identities without HID implementation.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidTopology = errors.New("invalid USB topology")

type Match struct{ VendorID, ProductID uint16 }

type Connection string

const (
	Dongle Connection = "dongle"
	Wired  Connection = "wired"
)

type Candidate struct {
	Path, Serial                          string
	VendorID, ProductID, UsagePage, Usage uint16
	InterfaceNumber                       int
	Connection                            Connection
	Topology                              TopologyEvidence
	DeviceKey                             DeviceKey
	EpochID                               EpochID
	Occurrence                            uint64
}

// TopologyEvidence is normalized physical USB location evidence, not a device identity.
type TopologyEvidence struct {
	Bus   uint16  `json:"bus"`
	Ports []uint8 `json:"ports"`
}

func (t TopologyEvidence) MarshalJSON() ([]byte, error) {
	ports := make([]uint16, len(t.Ports))
	for index, port := range t.Ports {
		ports[index] = uint16(port)
	}
	return json.Marshal(struct {
		Bus   uint16   `json:"bus"`
		Ports []uint16 `json:"ports"`
	}{Bus: t.Bus, Ports: ports})
}

func (t *TopologyEvidence) UnmarshalJSON(contents []byte) error {
	var decoded struct {
		Bus   uint16   `json:"bus"`
		Ports []uint16 `json:"ports"`
	}
	if err := json.Unmarshal(contents, &decoded); err != nil {
		return err
	}
	t.Bus = decoded.Bus
	t.Ports = make([]uint8, len(decoded.Ports))
	for index, port := range decoded.Ports {
		if port == 0 || port > 255 {
			return ErrInvalidTopology
		}
		t.Ports[index] = uint8(port)
	}
	return t.Validate()
}

func (t TopologyEvidence) Validate() error {
	if t.Bus == 0 || len(t.Ports) == 0 {
		return ErrInvalidTopology
	}
	for _, port := range t.Ports {
		if port == 0 {
			return ErrInvalidTopology
		}
	}
	return nil
}

func (t TopologyEvidence) String() string {
	if t.Validate() != nil {
		return ""
	}
	ports := make([]string, len(t.Ports))
	for index, port := range t.Ports {
		ports[index] = strconv.Itoa(int(port))
	}
	return fmt.Sprintf("%d-%s", t.Bus, strings.Join(ports, "."))
}

func ParseTopology(bus uint16, path string) (TopologyEvidence, error) {
	parts := strings.SplitN(path, "-", 2)
	if bus == 0 || len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return TopologyEvidence{}, ErrInvalidTopology
	}
	pathBus, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil || uint16(pathBus) != bus {
		return TopologyEvidence{}, ErrInvalidTopology
	}
	ports := strings.Split(parts[1], ".")
	topology := TopologyEvidence{Bus: bus, Ports: make([]uint8, len(ports))}
	for index, rawPort := range ports {
		port, err := strconv.ParseUint(rawPort, 10, 8)
		if err != nil || port == 0 {
			return TopologyEvidence{}, ErrInvalidTopology
		}
		topology.Ports[index] = uint8(port)
	}
	return topology, topology.Validate()
}

type DeviceKey string
type EpochID uint64

// EpochTracker issues a new epoch only after a topology occurrence disappears.
type EpochTracker struct {
	nextEpoch   EpochID
	epochs      map[DeviceKey]EpochID
	occurrences map[DeviceKey]uint64
	present     map[DeviceKey]bool
}

func NewEpochTracker() EpochTracker { return EpochTracker{} }

func (t *EpochTracker) Refresh(candidates []Candidate) []Candidate {
	if t.occurrences == nil {
		t.epochs = make(map[DeviceKey]EpochID)
		t.occurrences = make(map[DeviceKey]uint64)
		t.present = make(map[DeviceKey]bool)
	}
	nextPresent := make(map[DeviceKey]bool, len(candidates))
	for index := range candidates {
		key := DeviceKey(candidates[index].Topology.String())
		if key == "" {
			continue
		}
		if !t.present[key] {
			t.nextEpoch++
			t.epochs[key] = t.nextEpoch
			t.occurrences[key]++
		}
		candidates[index].DeviceKey = key
		candidates[index].EpochID = t.epochs[key]
		candidates[index].Occurrence = t.occurrences[key]
		nextPresent[key] = true
	}
	t.present = nextPresent
	return candidates
}

type InputDescriptor struct {
	InterfaceNumber  int
	UsagePage, Usage uint16
	EndpointAddress  uint8
}

type InputSource struct{ Path string }

type PassiveInputTransport interface {
	Enumerate(context.Context, Match) ([]Candidate, error)
	ValidateDescriptor(context.Context, Candidate, InputDescriptor) (InputSource, error)
	ReadInterruptIN(context.Context, InputSource, func([]byte) bool) error
}

type CommandTransport interface {
	SendAndAwait(context.Context, []byte, func([]byte) bool) error
}

func X6Match() Match { return Match{VendorID: 0x1D57, ProductID: 0xFA60} }
