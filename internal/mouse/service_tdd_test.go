package mouse

import (
	"context"
	"errors"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/transport"
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
