package mouse

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

func TestTargetedServiceSelectsSoleDeviceAndRequiresSelectionForMany(t *testing.T) {
	profile := targetedProfile{}
	registry, err := NewProfileRegistry(profile)
	if err != nil {
		t.Fatal(err)
	}
	single := transport.Candidate{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{single}}, &commandFake{})

	devices, err := svc.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(devices) != 1 || !devices[0].Eligible {
		t.Fatalf("Refresh() = %#v; want one eligible device", devices)
	}
	if binding, ok := svc.Selection(); !ok || binding.ID.Serial != "A" || binding.Path != "hidraw-1" {
		t.Fatalf("Selection() = %#v, %t; want sole device binding", binding, ok)
	}

	multiple := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{single, {Path: "hidraw-2", Serial: "B", VendorID: 0x1D57, ProductID: 0xFA60}}}, &commandFake{})
	if _, err := multiple.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := multiple.Selection(); ok {
		t.Fatal("Selection() unexpectedly chose a device from multiple candidates")
	}
	if err := multiple.Stage("pending"); !errors.Is(err, ErrSelectionRequired) {
		t.Fatalf("Stage() error = %v, want ErrSelectionRequired", err)
	}
}

func TestTargetedServiceRefreshKeepsProfileMismatchesVisibleButIneligible(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name       string
		candidate  transport.Candidate
		profileOK  bool
		eligible   bool
		warning    string
		selectedOK bool
	}{
		{
			name:       "stable serial with interface mismatch",
			candidate:  transport.Candidate{Path: "hidraw-mismatch", Serial: "stable", VendorID: 0x1D57, ProductID: 0xFA60},
			profileOK:  false,
			eligible:   false,
			warning:    "profile/interface mismatch",
			selectedOK: false,
		},
		{
			name:       "validated interface remains selectable",
			candidate:  transport.Candidate{Path: "hidraw-valid", Serial: "stable", VendorID: 0x1D57, ProductID: 0xFA60},
			profileOK:  true,
			eligible:   true,
			selectedOK: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTargetedService(registry, profileValidatedInventory{candidates: []transport.Candidate{tt.candidate}, valid: tt.profileOK}, &commandFake{})

			devices, err := svc.Refresh(context.Background())
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
			if len(devices) != 1 || devices[0].Eligible != tt.eligible || devices[0].Warning != tt.warning {
				t.Fatalf("Refresh() = %#v; want visible device eligible=%t warning=%q", devices, tt.eligible, tt.warning)
			}
			if _, ok := svc.Selection(); ok != tt.selectedOK {
				t.Fatalf("Selection() present = %t; want %t", ok, tt.selectedOK)
			}
		})
	}
}

func TestTargetedServiceUsesSoleValidatedSeriallessX6AsSessionOnly(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	oldWriter := inventoryDiagnosticWriter
	inventoryDiagnosticWriter = &logs
	defer func() { inventoryDiagnosticWriter = oldWriter }()

	candidate := transport.Candidate{Path: "hidraw-missing-serial", VendorID: 0x1D57, ProductID: 0xFA60}
	service := NewTargetedService(registry, profileValidatedInventory{candidates: []transport.Candidate{candidate}, valid: true}, &commandFake{})
	devices, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || !devices[0].Eligible || devices[0].Warning != "session-only (no serial)" {
		t.Fatalf("Refresh() = %#v; want sole validated session-only device", devices)
	}
	binding, ok := service.Selection()
	if !ok || !binding.SessionOnly || binding.ID.Serial == "" || binding.ID.Serial == candidate.Path {
		t.Fatalf("Selection() = %#v, %t; want deterministic non-path session-only binding", binding, ok)
	}
	if second, ok := service.Selection(); !ok || second.ID != binding.ID {
		t.Fatalf("Selection() = %#v, %t; want deterministic session identity", second, ok)
	}
	output := logs.String()
	for _, want := range []string{"event=inventory_validation", "event=inventory_selection", "candidate_index=0", "vid_pid=1d57:fa60", "serial_present=false", "profile_match=true", "profile_validation=true", "eligibility=true", "warning=session-only (no serial)", "selected_binding_present=true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostic output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, candidate.Path) {
		t.Fatalf("diagnostic output contains candidate path:\n%s", output)
	}
}

func TestTargetedServiceRejectsSeriallessFallbackWhenNotSoleValidatedCandidate(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{})
	if err != nil {
		t.Fatal(err)
	}
	serialless := transport.Candidate{Path: "hidraw-serialless", VendorID: 0x1D57, ProductID: 0xFA60}
	stable := transport.Candidate{Path: "hidraw-stable", Serial: "stable", VendorID: 0x1D57, ProductID: 0xFA60}
	service := NewTargetedService(registry, profileValidatedInventory{candidates: []transport.Candidate{serialless, stable}, valid: true}, &commandFake{})

	devices, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].Eligible || devices[0].Warning != "ambiguous identity" {
		t.Fatalf("Refresh() = %#v; want serial-less fallback rejected among multiple candidates", devices)
	}
	if _, ok := service.Selection(); ok {
		t.Fatal("Selection() unexpectedly auto-selected with multiple candidates")
	}
}

func TestTargetedServiceRejectsSeriallessFallbackWhenProfileValidationFails(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{})
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{Path: "hidraw-invalid", VendorID: 0x1D57, ProductID: 0xFA60}
	service := NewTargetedService(registry, profileValidatedInventory{candidates: []transport.Candidate{candidate}, valid: false}, &commandFake{})

	devices, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Eligible || devices[0].Warning != "profile/interface mismatch" {
		t.Fatalf("Refresh() = %#v; want profile validation rejection", devices)
	}
	if _, ok := service.Selection(); ok {
		t.Fatal("Selection() unexpectedly auto-selected an invalid interface")
	}
}

func TestTargetedServiceRejectsStaleBindingAndIsolatesStateAndEvents(t *testing.T) {
	profile := targetedProfile{}
	registry, _ := NewProfileRegistry(profile)
	a := transport.Candidate{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}
	b := transport.Candidate{Path: "hidraw-2", Serial: "B", VendorID: 0x1D57, ProductID: 0xFA60}
	source := &mutableInventory{candidates: []transport.Candidate{a, b}}
	command := &commandFake{}
	svc := NewTargetedService(registry, source, command)
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Select(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stage("A-pending"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Select(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "B"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stage("B-pending"); err != nil {
		t.Fatal(err)
	}
	if !svc.HandleEvent(Event{ID: DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "B"}, Path: "hidraw-2", Delta: "B-event"}) {
		t.Fatal("HandleEvent() did not accept B event")
	}
	if svc.HandleEvent(Event{ID: DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}, Path: "hidraw-2", Delta: "foreign"}) {
		t.Fatal("HandleEvent() accepted an event with A identity on B path")
	}
	if state, ok := svc.State(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}); !ok || state.Pending != "A-pending" || state.Event != nil {
		t.Fatalf("A State() = %#v, %t; want isolated pending state", state, ok)
	}
	if state, ok := svc.State(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "B"}); !ok || state.Pending != "B-pending" || state.Event != "B-event" {
		t.Fatalf("B State() = %#v, %t; want B pending and event", state, ok)
	}

	if err := svc.Select(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"}); err != nil {
		t.Fatal(err)
	}
	source.candidates = []transport.Candidate{{Path: "hidraw-3", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}, b}
	if err := svc.Apply(context.Background()); !errors.Is(err, ErrStaleBinding) {
		t.Fatalf("Apply() error = %v, want ErrStaleBinding", err)
	}
	if command.calls != 0 {
		t.Fatalf("command calls = %d, want 0 for stale binding", command.calls)
	}
}

func TestTargetedServiceDoesNotCommitApplyWhenPendingRevisionChanges(t *testing.T) {
	profile := targetedProfile{}
	registry, _ := NewProfileRegistry(profile)
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}}}, nil)
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stage("first"); err != nil {
		t.Fatal(err)
	}
	svc.command = &commandFake{onApply: func() { _ = svc.Stage("second") }}
	if err := svc.Apply(context.Background()); !errors.Is(err, ErrRevisionChanged) {
		t.Fatalf("Apply() error = %v, want ErrRevisionChanged", err)
	}
	state, ok := svc.State(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "A"})
	if !ok || state.Applied != nil || state.Pending != "second" {
		t.Fatalf("State() = %#v, %t; want unapplied latest pending state", state, ok)
	}
}

type inventoryFake struct{ candidates []transport.Candidate }

func (f inventoryFake) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type profileValidatedInventory struct {
	candidates []transport.Candidate
	valid      bool
}

func (f profileValidatedInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

func (f profileValidatedInventory) ProfileValid(context.Context, transport.Candidate, HIDFacts) bool {
	return f.valid
}

type mutableInventory struct{ candidates []transport.Candidate }

func (f *mutableInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type commandFake struct {
	calls   int
	onApply func()
}

type targetedProfile struct{}

func (targetedProfile) ID() string             { return "x6" }
func (targetedProfile) Match() transport.Match { return transport.X6Match() }
func (targetedProfile) HIDFacts() HIDFacts     { return HIDFacts{} }
func (targetedProfile) Codec() Codec           { return targetedCodec{} }

type targetedCodec struct{}

func (targetedCodec) Validate(any) error              { return nil }
func (targetedCodec) Encode(any) ([]byte, error)      { return []byte{0x04}, nil }
func (targetedCodec) DecodeStatus([]byte) (any, bool) { return nil, false }
func (targetedCodec) MatchesACK([]byte) bool          { return true }
func (targetedCodec) Defaults() any                   { return nil }

func (f *commandFake) SendAndAwaitBound(context.Context, Binding, []byte, func([]byte) bool) error {
	f.calls++
	if f.onApply != nil {
		f.onApply()
	}
	return nil
}
