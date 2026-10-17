package main

import (
	"context"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type combinedInventoryFake struct {
	profileValidBackendFake
	combined bool
}

func (f *combinedInventoryFake) EnumerateX6(context.Context) ([]transport.Candidate, error) {
	f.combined = true
	return []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa60, Connection: transport.Dongle}, {VendorID: 0x1d57, ProductID: 0xfa61, Connection: transport.Wired}}, nil
}
func TestPairedInventorySelectsAndAppliesCable(t *testing.T) {
	backend := &combinedInventoryFake{profileValidBackendFake: profileValidBackendFake{valid: true}}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile(), x6.NewWiredProfile())
	command := &targetedCommandFake{}
	inventory := mouse.NewTargetedService(registry, x6InventorySource{backend: backend}, command)
	devices, err := inventory.Refresh(context.Background())
	binding, selected := inventory.Selection()
	if err != nil || !selected || len(devices) != 1 || binding.ID.ProductID != 0xfa61 {
		t.Fatalf("devices=%+v selected=%t binding=%+v err=%v", devices, selected, binding, err)
	}
	if err := inventory.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls, bindings := command.observations()
	if calls != 1 || bindings[0].ID.ProductID != 0xfa61 {
		t.Fatalf("calls=%d bindings=%v", calls, bindings)
	}
}

type preferenceInventoryFake struct {
	profileValidBackendFake
	candidates []transport.Candidate
}

func (f *preferenceInventoryFake) EnumerateX6(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

func TestCableArrivalRejectsCapturedDongleBinding(t *testing.T) {
	d := transport.Candidate{VendorID: 0x1d57, ProductID: 0xfa60, Connection: transport.Dongle, Path: "dongle"}
	w := transport.Candidate{VendorID: 0x1d57, ProductID: 0xfa61, Connection: transport.Wired, Path: "cable"}
	backend := &preferenceInventoryFake{profileValidBackendFake: profileValidBackendFake{valid: true}, candidates: []transport.Candidate{d}}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile(), x6.NewWiredProfile())
	command := &targetedCommandFake{}
	inventory := mouse.NewTargetedService(registry, x6InventorySource{backend: backend}, command)
	if _, err := inventory.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, ok := inventory.Selection()
	if !ok || binding.ID.ProductID != 0xfa60 {
		t.Fatalf("dongle selection=%v %t", binding, ok)
	}
	before, _ := inventory.State(binding.ID)
	backend.candidates = append(backend.candidates, w)
	if err := inventory.ApplyBound(context.Background(), binding, x6.DefaultDPIConfig()); err != mouse.ErrStaleBinding {
		t.Fatalf("stale apply=%v", err)
	}
	after, _ := inventory.State(binding.ID)
	calls, _ := command.observations()
	if calls != 0 || before.Revision != after.Revision {
		t.Fatalf("rejected apply calls=%d before=%v after=%v", calls, before, after)
	}
	backend.candidates = []transport.Candidate{w, w}
	if _, err := inventory.Refresh(context.Background()); err != mouse.ErrAmbiguousIdentity {
		t.Fatalf("duplicate cables=%v", err)
	}
	if _, ok := inventory.Selection(); ok {
		t.Fatal("multiple cables selected")
	}
	if err := inventory.Apply(context.Background()); err != mouse.ErrSelectionRequired {
		t.Fatalf("ambiguous apply=%v", err)
	}
	calls, _ = command.observations()
	if calls != 0 {
		t.Fatalf("ambiguous command calls=%d", calls)
	}
}

func TestProductionInventoryUsesCombinedUSBScan(t *testing.T) {
	backend := &combinedInventoryFake{}
	candidates, err := (x6InventorySource{backend: backend}).Enumerate(context.Background())
	if err != nil || !backend.combined || len(candidates) != 2 || candidates[1].ProductID != 0xfa61 || candidates[1].Connection != transport.Wired {
		t.Fatalf("inventory = %+v, %v combined=%t", candidates, err, backend.combined)
	}
}
