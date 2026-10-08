package mouse

import (
	"context"
	"errors"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
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
	// SessionOnly marks bindings that must never be persisted or migrated.
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

// MacroProgress records transport evidence only, never playback or persistence.
// Zero values explicitly mean neither phase was started.
type MacroProgress struct {
	Assignment MacroAssignmentProgress
	Upload     MacroUploadProgress
}
type MacroAssignmentProgress uint8
type MacroUploadProgress uint8

const (
	MacroAssignmentNotStarted MacroAssignmentProgress = iota
	MacroAssignmentUnknown
	MacroAssignmentACKConfirmed
)
const (
	MacroUploadNotStarted MacroUploadProgress = iota
	MacroUploadPossiblyPartial
	MacroUploadConfirmed
)

// TargetedMacroCommand is deliberately separate from generic report admission.
type TargetedMacroCommand interface {
	SendX6MacroAssignmentBound(context.Context, Binding, x6.MacroAssignment, macros.X6Click) (MacroProgress, error)
}

// TargetedMacroSequenceCommand is additive: legacy click-only commands remain valid.
type TargetedMacroSequenceCommand interface {
	SendX6MacroSequenceAssignmentBound(context.Context, Binding, x6.MacroAssignment, macros.X6Sequence) (MacroProgress, error)
}

func (s *TargetedService) ApplyMacroSequenceAssignmentBound(ctx context.Context, binding Binding, assignment x6.MacroAssignment, sequence macros.X6Sequence) (MacroProgress, error) {
	empty := MacroProgress{}
	if _, err := x6.EncodeMacroAssignmentReport(assignment); err != nil {
		return empty, err
	}
	destination, err := x6.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return empty, err
	}
	if _, err := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: destination, Sequence: sequence}); err != nil {
		return empty, err
	}
	command, ok := s.command.(TargetedMacroSequenceCommand)
	if !ok {
		return empty, errors.New("bound macro sequence command unavailable")
	}
	// Freeze the caller-owned slice before waiting for serialization.
	sequence.Buttons = append([]macros.EventType(nil), sequence.Buttons...)
	return s.applyMacroBound(ctx, binding, func() (MacroProgress, error) {
		return command.SendX6MacroSequenceAssignmentBound(ctx, binding, assignment, sequence)
	})
}

func (s *TargetedService) ApplyMacroAssignmentBound(ctx context.Context, binding Binding, assignment x6.MacroAssignment, click macros.X6Click) (MacroProgress, error) {
	empty := MacroProgress{}
	if _, err := x6.EncodeMacroAssignmentReport(assignment); err != nil {
		return empty, err
	}
	destination, err := x6.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return empty, err
	}
	if _, err := macros.EncodeX6Upload(macros.X6Upload{Destination: destination, Click: click}); err != nil {
		return empty, err
	}
	command, ok := s.command.(TargetedMacroCommand)
	if !ok {
		return empty, errors.New("bound macro command unavailable")
	}
	return s.applyMacroBound(ctx, binding, func() (MacroProgress, error) {
		return command.SendX6MacroAssignmentBound(ctx, binding, assignment, click)
	})
}

func (s *TargetedService) applyMacroBound(ctx context.Context, binding Binding, send func() (MacroProgress, error)) (MacroProgress, error) {
	empty := MacroProgress{}
	selected, state, _, err := s.selectedState()
	if err != nil || selected != binding {
		return empty, ErrStaleBinding
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	selected, _, _, err = s.selectedState()
	if err != nil || selected != binding || !s.bindingCurrent(ctx, binding) {
		return empty, ErrStaleBinding
	}
	return send()
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
		if ok && id.Validate() == nil && profileValid && (id.Serial != "" || profile.AllowsSeriallessIdentity()) {
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
	ambiguousSerialless := false
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
		case id.Validate() != nil || (id.Serial == "" && !profile.AllowsSeriallessIdentity()):
			device.Profile = profile.ID()
			device.Warning = "ambiguous identity"
		case counts[id] != 1:
			device.Profile = profile.ID()
			device.Warning = "ambiguous identity"
			ambiguousSerialless = ambiguousSerialless || id.Serial == ""
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
	if len(eligible) == 1 {
		binding := inventoryBinding(eligible[0], s.revision)
		s.selection = &binding
		inventoryDiagnosticf("event=selection_boundary candidate_index=%d vid_pid=%04x:%04x interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=%t hidraw_basename=unknown profile_match=true profile_validation=true eligibility=true warning=none selected_binding_present=true", selectedIndex, eligible[0].ID.VendorID, eligible[0].ID.ProductID, !s.selection.SessionOnly && eligible[0].ID.Serial != "")
	} else if len(candidates) > 1 || len(eligible) > 1 {
		s.selection = nil
		inventoryDiagnosticf("event=selection_boundary candidate_index=unknown vid_pid=multiple interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=unknown hidraw_basename=unknown profile_match=true profile_validation=true eligibility=multiple warning=selection_required selected_binding_present=false")
	} else {
		s.selection = nil
		inventoryDiagnosticf("event=selection_boundary candidate_index=unknown vid_pid=none interface_number=unknown endpoint=unknown hid_usage=unknown serial_present=unknown hidraw_basename=unknown profile_match=false profile_validation=false eligibility=false warning=selection_required selected_binding_present=false")
	}
	if ambiguousSerialless {
		return result, ErrAmbiguousIdentity
	}
	return result, nil
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

func inventoryBinding(device Device, revision uint64) Binding {
	return Binding{
		ID:                device.ID,
		ProfileID:         device.Profile,
		Path:              device.Path,
		InventoryRevision: revision,
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
	binding := inventoryBinding(device, s.revision)
	s.selection = &binding
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
	_, _, profile, err := s.selectedState()
	if err != nil {
		return ErrStaleBinding
	}
	return s.ApplyOperationBound(ctx, binding, codecOperation{profile.Codec()}, value)
}

// ApplyOperationBound validates and writes a typed operation only through the
// immutable binding captured by its caller.
func (s *TargetedService) ApplyOperationBound(ctx context.Context, binding Binding, operation CommandOperation, value any) error {
	selected, state, _, err := s.selectedState()
	if err != nil || selected != binding || !s.bindingCurrent(ctx, binding) || operation == nil {
		return ErrStaleBinding
	}
	if err := operation.Validate(value); err != nil {
		return err
	}
	report, err := operation.Encode(value)
	if err != nil {
		return err
	}
	if s.command == nil {
		return ErrStaleBinding
	}
	state.applyMu.Lock()
	defer state.applyMu.Unlock()
	return s.command.SendAndAwaitBound(ctx, binding, report, func(report []byte) bool { return !operation.MatchesACK(report) })
}

type codecOperation struct{ codec Codec }

func (o codecOperation) Validate(value any) error         { return o.codec.Validate(value) }
func (o codecOperation) Encode(value any) ([]byte, error) { return o.codec.Encode(value) }
func (o codecOperation) MatchesACK(report []byte) bool    { return o.codec.MatchesACK(report) }

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
