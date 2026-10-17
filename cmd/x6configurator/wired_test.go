package main

import (
	"context"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type combinedInventoryFake struct {
	profileValidBackendFake
	combined bool
}

func (f *combinedInventoryFake) EnumerateX6(context.Context) ([]transport.Candidate, error) {
	f.combined = true
	return []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa60, Connection: transport.Dongle}, {VendorID: 0x1d57, ProductID: 0xfa61, Connection: transport.Wired}}, nil
}
func TestProductionInventoryUsesCombinedUSBScan(t *testing.T) {
	backend := &combinedInventoryFake{}
	candidates, err := (x6InventorySource{backend: backend}).Enumerate(context.Background())
	if err != nil || !backend.combined || len(candidates) != 2 || candidates[1].ProductID != 0xfa61 || candidates[1].Connection != transport.Wired {
		t.Fatalf("inventory = %+v, %v combined=%t", candidates, err, backend.combined)
	}
}
