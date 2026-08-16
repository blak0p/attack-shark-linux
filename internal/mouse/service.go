package mouse

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/transport"
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
	// SessionOnly marks a serial-less binding which must never be persisted or migrated.
	SessionOnly bool
}

type Device struct {
	ID         DeviceID
	Profile    string
	Path       string
	Connection transport.Connection
	Eligible   bool
	Warning    string
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

// ProfileValidator reports whether a discovered candidate satisfies a profile's
// read-only HID facts. It is optional so legacy inventory sources remain valid.
type ProfileValidator interface {
	ProfileValid(context.Context, transport.Candidate, HIDFacts) bool
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
	validator, validatesProfiles := s.source.(ProfileValidator)
	counts := make(map[DeviceID]int)
	for index, candidate := range candidates {
		id := DeviceID{VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: candidate.Serial}
		profile, ok := s.registry.Lookup(id.VendorID, id.ProductID)
		profileValid := ok && (!validatesProfiles || validator.ProfileValid(ctx, candidate, profile.HIDFacts()))
		if ok && id.Validate() == nil && profileValid {
			counts[id]++
		}
		inventoryDiagnosticf("event=inventory_validation candidate_index=%d vid_pid=%04x:%04x interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=%t hidraw_basename=unknown profile_match=%t profile_validation=%t eligibility=pending warning=%s selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, candidate.Serial != "", ok, profileValid, inventoryWarning(ok, id, profileValid))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	s.devices = make(map[DeviceID]Device, len(candidates))
	eligible := make([]Device, 0, len(candidates))
	selectedIndex := -1
	result := make([]Device, 0, len(candidates))
	for index, candidate := range candidates {
		id := DeviceID{VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: candidate.Serial}
		profile, found := s.registry.Lookup(id.VendorID, id.ProductID)
		device := Device{ID: id, Path: candidate.Path, Connection: candidate.Connection}
		switch {
		case !found:
			device.Warning = "unsupported profile"
		case validatesProfiles && !validator.ProfileValid(ctx, candidate, profile.HIDFacts()):
			device.Profile = profile.ID()
			device.Warning = "profile/interface mismatch"
		case id.Validate() != nil && len(candidates) == 1 && validatesProfiles:
			id = sessionOnlyID(candidate, profile)
			device.ID, device.Profile, device.Eligible, device.Warning = id, profile.ID(), true, "session-only (no serial)"
			eligible = append(eligible, device)
			if _, exists := s.states[id]; !exists {
				s.states[id] = &deviceState{state: State{Applied: profile.Codec().Defaults(), Pending: profile.Codec().Defaults()}}
			}
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
		if device.Eligible && selectedIndex < 0 {
			selectedIndex = index
		}
		warning := device.Warning
		if warning == "" {
			warning = "none"
		}
		inventoryDiagnosticf("event=inventory_selection candidate_index=%d vid_pid=%04x:%04x interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=%t hidraw_basename=unknown profile_match=%t profile_validation=%t eligibility=%t warning=%s selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, candidate.Serial != "", found, found && device.Warning != "profile/interface mismatch", device.Eligible, warning)
	}
	if len(candidates) == 1 && len(eligible) == 1 {
		s.selection = &Binding{ID: eligible[0].ID, ProfileID: eligible[0].Profile, Path: eligible[0].Path, InventoryRevision: s.revision, SessionOnly: eligible[0].Warning == "session-only (no serial)"}
		inventoryDiagnosticf("event=selection_boundary candidate_index=%d vid_pid=%04x:%04x interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=%t hidraw_basename=unknown profile_match=true profile_validation=true eligibility=true warning=none selected_binding_present=true", selectedIndex, eligible[0].ID.VendorID, eligible[0].ID.ProductID, !s.selection.SessionOnly && eligible[0].ID.Serial != "")
	} else if len(candidates) > 1 || len(eligible) > 1 {
		s.selection = nil
		inventoryDiagnosticf("event=selection_boundary candidate_index=unknown vid_pid=multiple interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=unknown hidraw_basename=unknown profile_match=true profile_validation=true eligibility=multiple warning=selection_required selected_binding_present=false")
	} else {
		s.selection = nil
		inventoryDiagnosticf("event=selection_boundary candidate_index=unknown vid_pid=none interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=unknown hidraw_basename=unknown profile_match=false profile_validation=false eligibility=false warning=selection_required selected_binding_present=false")
	}
	return result, nil
}

func sessionOnlyID(candidate transport.Candidate, profile Profile) DeviceID {
	facts := profile.HIDFacts().StatusInput
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%04x:%04x:%d:%04x:%04x:%02x:%s", profile.ID(), candidate.VendorID, candidate.ProductID, facts.InterfaceNumber, facts.UsagePage, facts.Usage, facts.EndpointAddress, candidate.Connection)))
	return DeviceID{VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: fmt.Sprintf("session-%x", sum[:8])}
}

func inventoryWarning(profileMatch bool, id DeviceID, profileValid bool) string {
	switch {
	case !profileMatch:
		return "unsupported_profile"
	case !profileValid:
		return "profile_interface_mismatch"
	case id.Validate() != nil:
		return "missing_serial"
	default:
		return "none"
	}
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
	s.selection = &Binding{ID: device.ID, ProfileID: device.Profile, Path: device.Path, InventoryRevision: s.revision, SessionOnly: device.Warning == "session-only (no serial)"}
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

// ApplyBound validates and writes only the immutable binding captured by a caller.
func (s *TargetedService) ApplyBound(ctx context.Context, binding Binding, value any) error {
	selected, state, profile, err := s.selectedState()
	if err != nil || selected != binding || !s.bindingCurrent(ctx, binding) {
		return ErrStaleBinding
	}
	if err := profile.Codec().Validate(value); err != nil {
		return err
	}
	report, err := profile.Codec().Encode(value)
	if err != nil || s.command == nil {
		if err != nil {
			return err
		}
		return ErrStaleBinding
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	return s.command.SendAndAwaitBound(ctx, binding, report, func(report []byte) bool { return !profile.Codec().MatchesACK(report) })
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
		if candidate.Path == binding.Path && candidate.VendorID == binding.ID.VendorID && candidate.ProductID == binding.ID.ProductID && ((binding.SessionOnly && candidate.Serial == "") || (!binding.SessionOnly && candidate.Serial == binding.ID.Serial)) {
			return true
		}
	}
	return false
}
