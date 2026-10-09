package desktop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func remapMultiMacroFixture(t *testing.T) (*Service, *macroCommandFake, macros.Macro, macros.Macro) {
	t.Helper()
	lib, err := macros.Open(filepath.Join(t.TempDir(), "macros.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	command := &macroCommandFake{progress: mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}}
	candidates := []transport.Candidate{{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/mock-a"}}
	s := Compose(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}, WithMacroLibrary(lib, nil)).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command))
	s.RefreshInventory(context.Background())
	s.SelectDevice(DeviceID{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha"})

	m1, err := s.CreateMacro("left-click", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	m2, err := s.CreateMacro("right-click", []macros.Event{{Type: macros.MouseRight, Action: macros.Down}, {Type: macros.MouseRight, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	return s, command, m1, m2
}

func TestSimultaneousMultiMacroStagingAndApply(t *testing.T) {
	s, cmd, m1, m2 := remapMultiMacroFixture(t)

	// 1. Stage macro m1 on button 6 and macro m2 on button 7 simultaneously
	s6 := s.StageMacroAssignment(m1.ID, 6, 1)
	if s6.Error.Code != "" || len(s6.MacroDrafts) != 1 || s6.MacroDrafts[6].ID != m1.ID {
		t.Fatalf("stage button 6 failed: %+v", s6)
	}
	s7 := s.StageMacroAssignment(m2.ID, 7, 2)
	if s7.Error.Code != "" || len(s7.MacroDrafts) != 2 {
		t.Fatalf("stage button 7 failed: %+v", s7)
	}
	if s7.MacroDrafts[6].ID != m1.ID || s7.MacroDrafts[6].Button != 6 || s7.MacroDrafts[6].Repeat != 1 {
		t.Fatalf("button 6 draft corrupted: %+v", s7.MacroDrafts[6])
	}
	if s7.MacroDrafts[7].ID != m2.ID || s7.MacroDrafts[7].Button != 7 || s7.MacroDrafts[7].Repeat != 2 {
		t.Fatalf("button 7 draft corrupted: %+v", s7.MacroDrafts[7])
	}
	// Backwards compatibility: MacroPending points to the most recent draft
	if s7.MacroPending == nil || s7.MacroPending.Button != 7 {
		t.Fatalf("MacroPending backwards compatibility broken: %+v", s7.MacroPending)
	}

	// 2. Apply both macros and verify both are in applied state
	applied := s.ApplyRemap(s7.Pending)
	if applied.Error.Code != "" || applied.Firmware != "success" {
		t.Fatalf("ApplyRemap failed: %+v", applied)
	}
	if len(applied.MacroAppliedDrafts) != 2 {
		t.Fatalf("expected 2 applied drafts, got %d", len(applied.MacroAppliedDrafts))
	}
	if applied.MacroAppliedDrafts[6].ID != m1.ID || applied.MacroAppliedDrafts[7].ID != m2.ID {
		t.Fatalf("applied drafts mismatch: %+v", applied.MacroAppliedDrafts)
	}
	if applied.MacroApplied == nil {
		t.Fatalf("MacroApplied backwards compatibility broken")
	}
	if cmd.composite != 1 || len(cmd.items) != 2 {
		t.Fatalf("expected 1 composite call with 2 items, got calls=%d items=%d", cmd.composite, len(cmd.items))
	}
	if cmd.items[0].Button != 6 || cmd.items[1].Button != 7 {
		t.Fatalf("command items buttons mismatch: %+v", cmd.items)
	}

	// 3. Clear one button's macro while keeping the other
	cleared6 := s.ClearButtonMacroAssignment(6)
	if cleared6.Error.Code != "" {
		t.Fatalf("ClearButtonMacroAssignment(6) failed: %+v", cleared6)
	}
	if len(cleared6.MacroDrafts) != 1 {
		t.Fatalf("expected 1 remaining draft, got %d", len(cleared6.MacroDrafts))
	}
	if _, exists := cleared6.MacroDrafts[6]; exists {
		t.Fatal("button 6 was not cleared")
	}
	if draft7, exists := cleared6.MacroDrafts[7]; !exists || draft7.ID != m2.ID {
		t.Fatalf("button 7 was lost: %+v", cleared6.MacroDrafts)
	}
	if cleared6.MacroPending == nil || cleared6.MacroPending.Button != 7 {
		t.Fatalf("MacroPending should point to button 7: %+v", cleared6.MacroPending)
	}

	// 4. Clear all macros
	clearedAll := s.ClearMacroAssignment()
	if clearedAll.Error.Code != "" || len(clearedAll.MacroDrafts) != 0 || clearedAll.MacroPending != nil {
		t.Fatalf("ClearMacroAssignment failed: %+v", clearedAll)
	}
}
