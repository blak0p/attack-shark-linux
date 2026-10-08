package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type macroCommandFake struct {
	normal, composite int
	progress          mouse.MacroProgress
	err               error
	during            func()
	assignment        protocol.MacroAssignment
	sequence          macros.X6Sequence
	multiConfig       protocol.RemapConfig
	items             []mouse.MacroSequenceItem
}

func (f *macroCommandFake) SendAndAwaitBound(_ context.Context, _ Binding, _ []byte, _ func([]byte) bool) error {
	f.normal++
	return nil
}
func (f *macroCommandFake) SendX6MacroSequenceAssignmentBound(_ context.Context, _ Binding, a protocol.MacroAssignment, c macros.X6Sequence) (mouse.MacroProgress, error) {
	f.composite++
	f.assignment = a
	f.sequence = macros.X6Sequence{Buttons: append([]macros.EventType(nil), c.Buttons...), Repeat: c.Repeat}
	if f.during != nil {
		f.during()
	}
	return f.progress, f.err
}
func (f *macroCommandFake) SendX6MultiMacroSequenceAssignmentBound(_ context.Context, _ Binding, cfg protocol.RemapConfig, items []mouse.MacroSequenceItem) (mouse.MacroProgress, error) {
	f.composite++
	f.multiConfig = cfg
	f.items = append([]mouse.MacroSequenceItem(nil), items...)
	if len(items) == 1 {
		f.assignment = protocol.MacroAssignment{Config: cfg, Button: items[0].Button}
		f.sequence = macros.X6Sequence{Buttons: append([]macros.EventType(nil), items[0].Sequence.Buttons...), Repeat: items[0].Sequence.Repeat}
	}
	if f.during != nil {
		f.during()
	}
	return f.progress, f.err
}
func macroFixture(t *testing.T) (*Service, *macroCommandFake, macros.Macro) {
	t.Helper()
	lib, err := macros.Open(filepath.Join(t.TempDir(), "macros.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	command := &macroCommandFake{progress: mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}}
	candidates := []transport.Candidate{{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/mock-a"}, {VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta", Path: "/dev/mock-b"}}
	s := Compose(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}, WithMacroLibrary(lib, nil)).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command))
	inventory := s.RefreshInventory(context.Background())
	// Multiple devices require explicit selection.
	s.SelectDevice(inventory.Devices[0].ID)
	m, err := s.CreateMacro("click", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	return s, command, m
}
func TestMacroStrictPositiveAdmissionStageWithoutIO(t *testing.T) {
	for _, button := range []uint8{1, 2, 3, 4, 5, 6, 7} {
		for _, kind := range []macros.EventType{macros.MouseLeft, macros.MouseRight, "mouse_middle", "mouse_back", "mouse_forward"} {
			for _, repeat := range []int{1, 255} {
				s, c, m := macroFixture(t)
				m, err := s.UpdateMacro(m.ID, m.Name, []macros.Event{{Type: kind, Action: macros.Down}, {Type: kind, Action: macros.Up}})
				if err != nil {
					t.Fatal(err)
				}
				got := s.StageMacroAssignment(m.ID, button, repeat)
				if got.Error.Code != "" || got.MacroPending == nil || got.MacroPending.Name != "click" || c.normal+c.composite != 0 {
					t.Fatalf("%+v calls=%d", got, c.composite)
				}
				// Returned drafts cannot mutate the internal event snapshot.
				got.MacroPending.Events[0].DelayMS = 99
				if s.GetMacroAssignmentSnapshot().MacroPending.Events[0].DelayMS != 0 {
					t.Fatal("snapshot aliases state")
				}
			}
		}
	}
}

func TestMacroOrderedSequenceApplyFrozenDraft(t *testing.T) {
	kinds := []macros.EventType{macros.MouseLeft, macros.MouseRight, macros.MouseMiddle, macros.MouseBack, macros.MouseForward}
	for _, first := range kinds {
		for _, second := range kinds {
			for _, destination := range []uint8{1, 2, 3, 4, 5, 6, 7} {
				for _, repeat := range []int{1, 255} {
					s, c, m := macroFixture(t)
					events := []macros.Event{{Type: first, Action: macros.Down}, {Type: first, Action: macros.Up}, {Type: second, Action: macros.Down}, {Type: second, Action: macros.Up}}
					m, err := s.UpdateMacro(m.ID, "ordered clicks", events)
					if err != nil {
						t.Fatal(err)
					}
					staged := s.StageMacroAssignment(m.ID, destination, repeat)
					if staged.Error.Code != "" || staged.MacroPending == nil {
						t.Fatalf("stage %s/%s destination=%d repeat=%d: %+v", first, second, destination, repeat, staged)
					}
					want := cloneMacroDraft(staged.MacroPending)
					staged.MacroPending.Name = "mutated snapshot"
					staged.MacroPending.Events[2].Type = macros.MouseLeft
					got := s.ApplyRemap(staged.Pending)
					if got.Error.Code != "" || got.Firmware != "success" || !reflect.DeepEqual(got.MacroApplied, want) || c.assignment.Button != destination || c.sequence.Repeat != repeat || !reflect.DeepEqual(c.sequence.Buttons, []macros.EventType{first, second}) || c.composite != 1 || c.normal != 0 {
						t.Fatalf("ordered frozen assignment lost: snapshot=%+v command=%+v", got, c)
					}
					got.MacroApplied.Events[0].DelayMS = 42
					if !reflect.DeepEqual(s.GetMacroAssignmentSnapshot().MacroApplied, want) {
						t.Fatal("applied events alias returned snapshot")
					}
				}
			}
		}
	}
}

func TestMacroStrictNegativeAdmissionPreservesDraftAndLibrary(t *testing.T) {
	valid := []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}}
	cases := []struct {
		name   string
		events []macros.Event
		repeat int
		button uint8
	}{
		{"empty", nil, 1, 1},
		{"single", valid[:1], 1, 1},
		{"incomplete second pair", append(append([]macros.Event{}, valid...), valid[0]), 1, 1},
		{"multi sequence (three clicks)", append(append(append([]macros.Event{}, valid...), valid...), valid...), 1, 1},
		{"second pair timing", append(append([]macros.Event{}, valid...), macros.Event{Type: macros.MouseRight, Action: macros.Down}, macros.Event{Type: macros.MouseRight, Action: macros.Up, DelayMS: 1}), 1, 1},
		{"second pair reversed", append(append([]macros.Event{}, valid...), valid[1], valid[0]), 1, 1},
		{"second pair unmatched", append(append([]macros.Event{}, valid...), valid[0], macros.Event{Type: macros.MouseRight, Action: macros.Up}), 1, 1},
		{"different buttons", []macros.Event{valid[0], {Type: macros.MouseRight, Action: macros.Up}}, 1, 1},
		{"reversed", []macros.Event{valid[1], valid[0]}, 1, 1},
		{"two downs", []macros.Event{valid[0], valid[0]}, 1, 1},
		{"down timing", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: 1}, valid[1]}, 1, 1},
		{"up timing", []macros.Event{valid[0], {Type: macros.MouseLeft, Action: macros.Up, DelayMS: 1}}, 1, 1},
		{"repeat zero", valid, 0, 1}, {"repeat negative", valid, -1, 1}, {"repeat 256", valid, 256, 1},
		{"button zero", valid, 1, 0}, {"button eight", valid, 1, 8},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s, c, m := macroFixture(t)
			original := s.StageMacroAssignment(m.ID, 6, 2).MacroPending
			other, err := s.CreateMacro(tt.name, tt.events)
			if err != nil {
				t.Fatal(err)
			}
			got := s.StageMacroAssignment(other.ID, tt.button, tt.repeat)
			after, _ := s.ReadMacro(other.ID)
			if got.Error.Code != InvalidConfiguration || !reflect.DeepEqual(original, got.MacroPending) || !reflect.DeepEqual(other, after) || c.composite+c.normal != 0 {
				t.Fatalf("lost draft/data or wrote hardware: %+v", got)
			}
		})
	}
	for _, events := range [][]macros.Event{
		{{Type: "keyboard", Action: macros.Down}, {Type: "keyboard", Action: macros.Up}},
		{{Type: macros.MouseLeft, Action: "unknown"}, {Type: macros.MouseLeft, Action: macros.Up}},
		{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: -1}, {Type: macros.MouseLeft, Action: macros.Up}},
	} {
		if _, ok := admittedSequence(events, 1); ok {
			t.Fatal("invalid data admitted")
		}
	}
}

func TestMacroPerDeviceIsolationClearingDiscardReplacement(t *testing.T) {
	s, c, m := macroFixture(t)
	a, _ := s.selectedBinding()
	inventory := s.RefreshInventory(context.Background())
	var b DeviceID
	for _, device := range inventory.Devices {
		if device.ID != a.ID {
			b = device.ID
		}
	}
	s.SelectDevice(a.ID)
	first := s.StageMacroAssignment(m.ID, 6, 1)
	moved := s.StageMacroAssignment(m.ID, 2, 3)
	if moved.MacroPending.Button != 2 || moved.Revision <= first.Revision {
		t.Fatal("replacement not explicit single draft")
	}
	s.SelectDevice(b)
	if s.GetRemapSnapshot().MacroPending != nil {
		t.Fatal("draft leaked across devices")
	}
	s.StageMacroAssignment(m.ID, 7, 255)
	s.SelectDevice(a.ID)
	if s.GetRemapSnapshot().MacroPending.Button != 2 {
		t.Fatal("device draft lost")
	}
	if s.ClearMacroAssignment().MacroPending != nil {
		t.Fatal("clear failed")
	}
	if s.DiscardRemap().MacroPending != nil {
		t.Fatal("discard resurrected macro")
	}
	s.SelectDevice(b)
	if s.DiscardRemap().MacroPending != nil {
		t.Fatal("discard failed")
	}
	if c.normal+c.composite != 0 {
		t.Fatal("draft lifecycle wrote hardware")
	}
}

func TestMacroApplySuccessOnlyStateAdvanceNormalRemapRegression(t *testing.T) {
	s, c, m := macroFixture(t)
	pending := s.GetRemapSnapshot().Pending
	pending.Buttons[3].Action = x6.RemapFire
	s.StageMacroAssignment(m.ID, 7, 255)
	got := s.ApplyRemap(pending)
	if got.Error.Code != "" || got.Firmware != "success" || got.MacroApplied == nil || got.MacroProgress != c.progress || got.Persistence != "not_supported" || c.composite != 1 || c.normal != 0 {
		t.Fatalf("%+v command=%+v", got, c)
	}
	if !reflect.DeepEqual(c.assignment.Config, pending) || c.assignment.Button != 7 || c.sequence.Repeat != 255 {
		t.Fatal("unrelated fields or assignment lost")
	}
	s.ClearMacroAssignment()
	// Discard does not resurrect an applied macro, even before the ordinary write.
	if s.DiscardRemap().MacroPending != nil {
		t.Fatal("resurrected applied macro")
	}
	got = s.ApplyRemap(pending)
	if c.normal != 1 || c.composite != 1 || got.MacroApplied != nil || got.Error.Code != "" {
		t.Fatalf("normal regression: %+v", got)
	}
	s.ApplyRemap(pending)
	if c.normal != 1 {
		t.Fatal("ordinary identical apply regression")
	}
}

func TestMacroStageOrdinaryFieldsPreservedThroughLifecycle(t *testing.T) {
	s, c, m := macroFixture(t)
	config := s.GetRemapSnapshot().Pending
	config.Buttons[3].Action = x6.RemapFire
	s.StageRemap(config)
	staged := s.StageMacroAssignment(m.ID, 7, 3)
	if !reflect.DeepEqual(staged.Pending, config) {
		t.Fatal("macro staging lost ordinary draft")
	}
	config.Buttons[4].Action = x6.RemapDoubleClick
	staged = s.StageRemap(config)
	if staged.MacroPending == nil || staged.MacroPending.Button != 7 {
		t.Fatal("ordinary staging lost macro overlay")
	}
	invalid := s.StageRemap(x6.RemapConfig{})
	if invalid.Error.Code != InvalidConfiguration || !reflect.DeepEqual(invalid.Pending, config) || invalid.MacroPending == nil {
		t.Fatal("invalid staging destroyed draft")
	}
	cleared := s.ClearMacroAssignment()
	if !reflect.DeepEqual(cleared.Pending, config) || cleared.MacroPending != nil {
		t.Fatal("clear lost unrelated fields")
	}
	discarded := s.DiscardRemap()
	if !reflect.DeepEqual(discarded.Pending, discarded.Applied) || c.normal+c.composite != 0 {
		t.Fatal("discard wrote or did not restore")
	}
}

func TestMacroConcurrentDraftMutationRetainsTransportEvidence(t *testing.T) {
	s, c, m := macroFixture(t)
	staged := s.StageMacroAssignment(m.ID, 6, 1)
	entered, release := make(chan struct{}), make(chan struct{})
	c.during = func() { close(entered); <-release }
	result := make(chan RemapSnapshot, 1)
	go func() { result <- s.ApplyRemap(staged.Pending) }()
	<-entered
	// Staging remains local and can invalidate an in-flight captured revision.
	replaced := s.StageMacroAssignment(m.ID, 7, 255)
	read := s.GetRemapSnapshot()
	if read.MacroPending.Button != 7 || replaced.MacroPending.Repeat != 255 {
		t.Fatal("concurrent draft replacement lost")
	}
	close(release)
	got := <-result
	if got.Error.Code != StaleBinding || got.MacroApplied != nil || got.MacroProgress != c.progress || got.MacroPending.Button != 7 {
		t.Fatalf("stale in-flight completion: %+v", got)
	}
}

func TestMacroSuccessDoesNotPersistOrdinaryConfigAsMacro(t *testing.T) {
	s, c, m := macroFixture(t)
	persistence := &remapPersistenceFake{}
	s.remapPersistence = persistence
	s.StageMacroAssignment(m.ID, 6, 1)
	got := s.ApplyRemap(x6.DefaultRemapConfig())
	if got.Firmware != "success" || got.Persistence != "not_supported" || persistence.saves != 0 || got.RetryAvailable {
		t.Fatalf("macro incorrectly persisted as ordinary config: %+v", got)
	}
	s.RetryRemapPersistence()
	if persistence.saves != 0 || c.composite != 1 {
		t.Fatal("unsupported macro persistence retried hardware or save")
	}
	// Factory reconciliation clears local overlay identity, not just ordinary fields.
	s.operationMu.Lock()
	s.remapComponent.reconcileFactoryReset()
	s.operationMu.Unlock()
	reset := s.GetRemapSnapshot()
	if reset.MacroPending != nil || reset.MacroApplied != nil || reset.MacroProgress != (mouse.MacroProgress{}) {
		t.Fatal("reset retained macro draft/evidence")
	}
}

func TestMacroMissingLibraryAndIDDoNotReplaceDraft(t *testing.T) {
	s, c, m := macroFixture(t)
	staged := s.StageMacroAssignment(m.ID, 1, 1)
	got := s.StageMacroAssignment("missing", 2, 1)
	if got.Error.Code != InvalidConfiguration || !reflect.DeepEqual(got.MacroPending, staged.MacroPending) {
		t.Fatal("missing ID replaced draft")
	}
	s.macroLibraryError = errors.New("unavailable library")
	got = s.StageMacroAssignment(m.ID, 2, 1)
	if got.Error.Code != InvalidConfiguration || !reflect.DeepEqual(got.MacroPending, staged.MacroPending) || c.composite+c.normal != 0 {
		t.Fatal("unavailable library replaced draft or wrote hardware")
	}
}

func TestMacroSelectedBindingStaleBeforeApply(t *testing.T) {
	s, c, m := macroFixture(t)
	staged := s.StageMacroAssignment(m.ID, 6, 1)
	// A new inventory epoch invalidates selection; no transport is permitted.
	s.RefreshInventory(context.Background())
	got := s.ApplyRemap(staged.Pending)
	if got.Error.Code != SelectionRequired || c.composite+c.normal != 0 {
		t.Fatalf("stale selected binding reached hardware: %+v", got)
	}
}

func TestMacroPartialACKUploadFailurePreservesDraftNoRetry(t *testing.T) {
	for _, progress := range []mouse.MacroProgress{
		{},
		{Assignment: mouse.MacroAssignmentUnknown},
		{Assignment: mouse.MacroAssignmentACKConfirmed},
		{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadPossiblyPartial},
	} {
		t.Run("partial", func(t *testing.T) {
			s, c, m := macroFixture(t)
			before := s.GetRemapSnapshot().Applied
			staged := s.StageMacroAssignment(m.ID, 6, 2)
			pending := staged.Pending
			pending.Buttons[4].Action = x6.RemapFire
			c.progress, c.err = progress, errors.New("transport failure")
			got := s.ApplyRemap(pending)
			if got.Firmware != "failed" || got.Error.Code == "" || !reflect.DeepEqual(got.Applied, before) || !reflect.DeepEqual(got.MacroPending, staged.MacroPending) || got.MacroProgress != progress || got.MacroApplied != nil || c.composite != 1 || got.RetryAvailable {
				t.Fatalf("false completion/lost evidence: %+v", got)
			}
			s.GetRemapSnapshot()
			s.RetryRemapPersistence()
			if c.composite != 1 {
				t.Fatal("automatic retry")
			}
		})
	}
	// A nil transport error without both confirmations is not success.
	s, c, m := macroFixture(t)
	c.progress = mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed}
	s.StageMacroAssignment(m.ID, 1, 1)
	if s.ApplyRemap(x6.DefaultRemapConfig()).Firmware != "failed" {
		t.Fatal("missing upload status accepted")
	}
}

func TestMacroLibraryEditDeleteRejectBeforeIO(t *testing.T) {
	for _, mutation := range []string{"edit", "rename", "delete"} {
		t.Run(mutation, func(t *testing.T) {
			s, c, m := macroFixture(t)
			staged := s.StageMacroAssignment(m.ID, 6, 1)
			switch mutation {
			case "edit":
				m.Events[0].DelayMS = 17
				_, _ = s.UpdateMacro(m.ID, m.Name, m.Events)
			case "rename":
				_, _ = s.UpdateMacro(m.ID, "renamed", m.Events)
			case "delete":
				_ = s.DeleteMacro(m.ID)
			}
			got := s.ApplyRemap(staged.Pending)
			if got.Error.Code != InvalidConfiguration || c.composite != 0 || !reflect.DeepEqual(got.MacroPending, staged.MacroPending) {
				t.Fatalf("stale library applied: %+v", got)
			}
		})
	}
}

func TestMacroSelectionRevisionAndLibraryRacesDoNotAdvance(t *testing.T) {
	for _, mutation := range []string{"selection", "replacement", "clear", "discard", "edit", "rename", "delete"} {
		t.Run(mutation, func(t *testing.T) {
			s, c, m := macroFixture(t)
			inventory := s.RefreshInventory(context.Background())
			s.SelectDevice(inventory.Devices[0].ID)
			binding, _ := s.selectedBinding()
			var other DeviceID
			for _, device := range inventory.Devices {
				if device.ID != binding.ID {
					other = device.ID
				}
			}
			m, err := s.UpdateMacro(m.ID, "mixed sequence", []macros.Event{
				{Type: macros.MouseForward, Action: macros.Down}, {Type: macros.MouseForward, Action: macros.Up},
				{Type: macros.MouseMiddle, Action: macros.Down}, {Type: macros.MouseMiddle, Action: macros.Up},
			})
			if err != nil {
				t.Fatal(err)
			}
			staged := s.StageMacroAssignment(m.ID, 6, 2)
			frozen := cloneMacroDraft(staged.MacroPending)
			before := staged.Applied
			c.during = func() {
				switch mutation {
				case "selection":
					s.SelectDevice(other)
				case "replacement":
					s.StageMacroAssignment(m.ID, 7, 3)
				case "clear":
					s.ClearMacroAssignment()
				case "discard":
					s.DiscardRemap()
				case "edit":
					m.Events[0].DelayMS = 31
					_, _ = s.UpdateMacro(m.ID, m.Name, m.Events)
				case "rename":
					_, _ = s.UpdateMacro(m.ID, "renamed during apply", m.Events)
				case "delete":
					_ = s.DeleteMacro(m.ID)
				}
			}
			got := s.ApplyRemap(staged.Pending)
			if got.Error.Code != StaleBinding || got.Firmware != "failed" || got.MacroApplied != nil || !reflect.DeepEqual(got.Applied, before) || got.MacroProgress != c.progress || c.composite != 1 {
				t.Fatalf("stale completion advanced: %+v", got)
			}
			if !reflect.DeepEqual(c.sequence.Buttons, []macros.EventType{macros.MouseForward, macros.MouseMiddle}) || c.sequence.Repeat != 2 {
				t.Fatal("transport snapshot changed")
			}
			if mutation == "edit" || mutation == "rename" || mutation == "delete" {
				if !reflect.DeepEqual(got.MacroPending, frozen) {
					t.Fatal("library mutation changed frozen identity/name/events")
				}
			}
			if mutation == "clear" || mutation == "discard" {
				if got.MacroPending != nil {
					t.Fatal("cleared draft resurrected")
				}
			}
		})
	}
}

func TestSimultaneousMultiMacroAssignment(t *testing.T) {
	s, c, m1 := macroFixture(t)
	m2, err := s.CreateMacro("right-pair", []macros.Event{{Type: macros.MouseRight, Action: macros.Down}, {Type: macros.MouseRight, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}

	// Stage on button 6 and button 7 simultaneously
	_ = s.StageMacroAssignment(m1.ID, 6, 1)
	staged7 := s.StageMacroAssignment(m2.ID, 7, 2)
	if len(staged7.MacroDrafts) != 2 {
		t.Fatalf("expected 2 staged drafts, got %d", len(staged7.MacroDrafts))
	}
	if staged7.MacroDrafts[6].ID != m1.ID || staged7.MacroDrafts[7].ID != m2.ID {
		t.Fatalf("staged drafts mismatch: %+v", staged7.MacroDrafts)
	}

	// Apply both macros
	applied := s.ApplyRemap(staged7.Pending)
	if applied.Error.Code != "" || applied.Firmware != "success" {
		t.Fatalf("apply failed: %+v", applied)
	}
	if len(applied.MacroAppliedDrafts) != 2 {
		t.Fatalf("expected 2 applied drafts, got %d", len(applied.MacroAppliedDrafts))
	}
	if applied.MacroAppliedDrafts[6].ID != m1.ID || applied.MacroAppliedDrafts[7].ID != m2.ID {
		t.Fatalf("applied drafts mismatch: %+v", applied.MacroAppliedDrafts)
	}
	if c.composite != 1 || len(c.items) != 2 {
		t.Fatalf("command mismatch: calls=%d items=%d", c.composite, len(c.items))
	}

	// Clear button 6 while keeping button 7
	cleared6 := s.ClearButtonMacroAssignment(6)
	if len(cleared6.MacroDrafts) != 1 || cleared6.MacroDrafts[7].ID != m2.ID {
		t.Fatalf("clear button 6 failed: %+v", cleared6.MacroDrafts)
	}
	if cleared6.MacroPending == nil || cleared6.MacroPending.Button != 7 {
		t.Fatalf("MacroPending mismatch: %+v", cleared6.MacroPending)
	}

	// Discard clears all staged macros
	discarded := s.DiscardRemap()
	if len(discarded.MacroDrafts) != 0 || discarded.MacroPending != nil {
		t.Fatalf("discard failed: %+v", discarded)
	}
}
