package mouse

import (
	"context"
	"errors"
	"sync"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

var (
	ErrSelectionRequired = errors.New("selection required")
	ErrStaleBinding      = errors.New("stale binding")
	ErrRevisionChanged   = errors.New("configuration revision changed")
)

// Binding captures the exact transient connection for one operation.
type Binding struct {
	ID                DeviceID
	ProfileID         string
	Path              string
	InventoryRevision uint64
}

type Device struct {
	ID       DeviceID
	Profile  string
	Path     string
	Eligible bool
	Warning  string
}

type Event struct {
	ID    DeviceID
	Path  string
	Delta any
}

type State struct {
	Applied  any
	Pending  any
	Event    any
	Revision uint64
}

type InventorySource interface {
	Enumerate(context.Context) ([]transport.Candidate, error)
}

// TargetedCommand must revalidate and operate on the supplied binding, never
// discover a replacement device during an operation.
type TargetedCommand interface {
	SendAndAwaitBound(context.Context, Binding, []byte, func([]byte) bool) error
}

type deviceState struct {
	mu      sync.Mutex
	applyMu sync.Mutex
	state   State
}

// TargetedService owns inventory selection and state scoped to stable identities.
type TargetedService struct {
	mu        sync.Mutex
	registry  *ProfileRegistry
	source    InventorySource
	command   TargetedCommand
	revision  uint64
	devices   map[DeviceID]Device
	states    map[DeviceID]*deviceState
	selection *Binding
}

func NewTargetedService(registry *ProfileRegistry, source InventorySource, command TargetedCommand) *TargetedService {
	return &TargetedService{registry: registry, source: source, command: command, devices: make(map[DeviceID]Device), states: make(map[DeviceID]*deviceState)}
}

func (s *TargetedService) Refresh(ctx context.Context) ([]Device, error) {
	candidates, err := s.source.Enumerate(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[DeviceID]int)
	for _, candidate := range candidates {
		id := DeviceID{VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: candidate.Serial}
		if _, ok := s.registry.Lookup(id.VendorID, id.ProductID); ok && id.Validate() == nil {
			counts[id]++
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	s.devices = make(map[DeviceID]Device, len(candidates))
	eligible := make([]Device, 0, len(candidates))
	result := make([]Device, 0, len(candidates))
	for _, candidate := range candidates {
		id := DeviceID{VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: candidate.Serial}
		profile, found := s.registry.Lookup(id.VendorID, id.ProductID)
		device := Device{ID: id, Path: candidate.Path}
		switch {
		case !found:
			device.Warning = "unsupported profile"
		case id.Validate() != nil || counts[id] != 1:
			device.Profile = profile.ID()
			device.Warning = "ambiguous identity"
		default:
			device.Profile, device.Eligible = profile.ID(), true
			eligible = append(eligible, device)
			if _, exists := s.states[id]; !exists {
				s.states[id] = &deviceState{state: State{Applied: profile.Codec().Defaults(), Pending: profile.Codec().Defaults()}}
			}
		}
		s.devices[id] = device
		result = append(result, device)
	}
	if len(eligible) == 1 {
		s.selection = &Binding{ID: eligible[0].ID, ProfileID: eligible[0].Profile, Path: eligible[0].Path, InventoryRevision: s.revision}
	} else if len(eligible) > 1 {
		s.selection = nil
	}
	return result, nil
}

func (s *TargetedService) Selection() (Binding, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selection == nil {
		return Binding{}, false
	}
	return *s.selection, true
}

func (s *TargetedService) Select(id DeviceID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.devices[id]
	if !ok || !device.Eligible {
		return ErrSelectionRequired
	}
	s.selection = &Binding{ID: device.ID, ProfileID: device.Profile, Path: device.Path, InventoryRevision: s.revision}
	return nil
}

func (s *TargetedService) Stage(value any) error {
	binding, state, profile, err := s.selectedState()
	if err != nil {
		return err
	}
	if err := profile.Codec().Validate(value); err != nil {
		return err
	}
	_ = binding
	state.mu.Lock()
	state.state.Pending = value
	state.state.Revision++
	state.mu.Unlock()
	return nil
}

func (s *TargetedService) Apply(ctx context.Context) error {
	binding, state, profile, err := s.selectedState()
	if err != nil {
		return err
	}
	if !s.bindingCurrent(ctx, binding) {
		return ErrStaleBinding
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	state.mu.Lock()
	pending, revision := state.state.Pending, state.state.Revision
	state.mu.Unlock()
	report, err := profile.Codec().Encode(pending)
	if err != nil {
		return err
	}
	if s.command == nil {
		return ErrStaleBinding
	}
	if err := s.command.SendAndAwaitBound(ctx, binding, report, func(report []byte) bool { return !profile.Codec().MatchesACK(report) }); err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.state.Revision != revision {
		return ErrRevisionChanged
	}
	state.state.Applied = pending
	return nil
}

func (s *TargetedService) HandleEvent(event Event) bool {
	s.mu.Lock()
	device, ok := s.devices[event.ID]
	state := s.states[event.ID]
	s.mu.Unlock()
	if !ok || state == nil || !device.Eligible || device.Path != event.Path {
		return false
	}
	state.mu.Lock()
	state.state.Event = event.Delta
	state.mu.Unlock()
	return true
}

func (s *TargetedService) State(id DeviceID) (State, bool) {
	s.mu.Lock()
	state, ok := s.states[id]
	s.mu.Unlock()
	if !ok {
		return State{}, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.state, true
}

func (s *TargetedService) selectedState() (Binding, *deviceState, Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selection == nil {
		return Binding{}, nil, nil, ErrSelectionRequired
	}
	binding := *s.selection
	device, ok := s.devices[binding.ID]
	if !ok || !device.Eligible || device.Path != binding.Path || device.Profile != binding.ProfileID {
		return Binding{}, nil, nil, ErrStaleBinding
	}
	profile, ok := s.registry.Lookup(binding.ID.VendorID, binding.ID.ProductID)
	if !ok {
		return Binding{}, nil, nil, ErrStaleBinding
	}
	return binding, s.states[binding.ID], profile, nil
}

func (s *TargetedService) bindingCurrent(ctx context.Context, binding Binding) bool {
	candidates, err := s.source.Enumerate(ctx)
	if err != nil {
		return false
	}
	for _, candidate := range candidates {
		if candidate.Path == binding.Path && candidate.VendorID == binding.ID.VendorID && candidate.ProductID == binding.ID.ProductID && candidate.Serial == binding.ID.Serial {
			return true
		}
	}
	return false
}
