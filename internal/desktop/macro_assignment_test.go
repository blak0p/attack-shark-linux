package desktop

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"context"
	"errors"
	"fmt"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// stubSequence is the decoded first upload chunk: the pressed buttons in order.
type stubSequence struct {
	Buttons []macros.EventType
	Repeat  int
	Mode    uint8
}

type macroCommandFake struct {
	normal, composite int
	progress          mouse.MacroProgress
	err               error
	during            func()
	// batch is the last batch received; sequence/destination are decoded from its
	// chunks when they form a legacy sequence upload.
	batch       mouse.Batch
	sequence    stubSequence
	destination byte
}

func (f *macroCommandFake) SendAndAwaitBound(_ context.Context, _ Binding, _ []byte, _ func([]byte) bool) error {
	f.normal++
	return nil
}
func (f *macroCommandFake) SendBatchBound(_ context.Context, _ Binding, batch mouse.Batch) (mouse.MacroProgress, error) {
	f.composite++
	f.batch = batch
	if len(batch.Chunks) > 0 {
		var chunk stubMacroChunk
		if err := json.Unmarshal(batch.Chunks[0], &chunk); err == nil {
			f.sequence = stubSequence{Repeat: chunk.Repeat, Mode: chunk.Mode}
			for _, event := range chunk.Events {
				if event.Action == macros.Down {
					f.sequence.Buttons = append(f.sequence.Buttons, event.Type)
				}
			}
			f.destination = chunk.Destination
		}
	}
	if f.during != nil {
		f.during()
	}
	return f.progress, f.err
}

// multiSetup is the report08 a batch for these buttons must open with.
func multiSetup(t *testing.T, config device.RemapConfig, buttons ...uint8) []byte {
	t.Helper()
	report, err := stubEncodeRemap(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, button := range buttons {
		destination := destinationFor(t, button)
		report[3+int(destination-1)*3] = 0x12
	}
	return report
}

func destinationFor(t *testing.T, button uint8) byte {
	t.Helper()
	destination, err := stubDestination(button)
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func macroFixture(t *testing.T) (*Service, *macroCommandFake, macros.Macro) {
	t.Helper()
	lib, err := macros.Open(filepath.Join(t.TempDir(), "macros.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	command := &macroCommandFake{progress: mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}}
	candidates := []transport.Candidate{{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/mock-a"}, {VendorID: 0x1D57, ProductID: 0xFA60, Serial: "beta", Path: "/dev/mock-b"}}
	s := Compose(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()}, WithMacroLibrary(lib, nil)).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command))
	inventory := s.RefreshInventory(context.Background())
	// Multiple devices require explicit selection.
	s.SelectDevice(inventory.Devices[0].ID)
	m, err := s.CreateMacro("click", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	return s, command, m
}
func TestMacroApplyPrevalidationPreservesState(t *testing.T) {
	for _, mutation := range []string{"library", "unsupported", "destination", "duplicate", "capability"} {
		t.Run(mutation, func(t *testing.T) {
			s, c, m := macroFixture(t)
			staged := s.StageMacroAssignment(m.ID, 6, 1)
			state := s.currentRemapState()
			switch mutation {
			case "library":
				_, _ = s.UpdateMacro(m.ID, "renamed", m.Events)
			case "unsupported":
				m, _ = s.UpdateMacro(m.ID, m.Name, []macros.Event{{Type: macros.Keyboard, KeyCode: "F13", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "F13", Action: macros.Up}})
				state.mu.Lock()
				state.macroPendingMap[6].Events = append([]macros.Event(nil), m.Events...)
				state.mu.Unlock()
			case "destination":
				state.mu.Lock()
				state.macroPendingMap[6].Button = 8
				state.mu.Unlock()
			case "duplicate":
				state.mu.Lock()
				state.macroPendingMap[7] = cloneMacroDraft(state.macroPendingMap[6])
				state.mu.Unlock()
			case "capability":
				m, _ = s.UpdateMacro(m.ID, m.Name, []macros.Event{{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up}})
				staged = s.StageMacroAssignment(m.ID, 6, 1)
				attachPlainCommand(s, c)
			}
			before := s.GetRemapSnapshot()
			config := cloneRemapConfig(staged.Pending)
			config.Buttons[3].Action = device.RemapFire
			got := s.ApplyRemap(config)
			if got.Error.Code == "" || got.Revision != before.Revision || !reflect.DeepEqual(got.Pending, before.Pending) || !reflect.DeepEqual(got.MacroDrafts, before.MacroDrafts) || got.MacroProgress != before.MacroProgress || got.RetryAvailable != before.RetryAvailable || c.normal+c.composite != 0 {
				t.Fatalf("invalid Apply changed state: before=%+v after=%+v calls=%d", before, got, c.normal+c.composite)
			}
		})
	}
}

func TestMacroApplyFreezesConfigBeforeSerialization(t *testing.T) {
	for _, boundary := range []string{"state-load", "macro-transport"} {
		t.Run(boundary, func(t *testing.T) {
			s, c, m := macroFixture(t)
			config := cloneRemapConfig(s.GetRemapSnapshot().Pending)
			if boundary == "macro-transport" {
				config = cloneRemapConfig(s.StageMacroAssignment(m.ID, 6, 1).Pending)
			}
			config.Buttons[3].Action = device.RemapFire
			want := cloneRemapConfig(config)
			entered, resume := make(chan struct{}), make(chan struct{})
			var releaseOnce, gateOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(resume) }) }
			defer release()
			gate := func() {
				gateOnce.Do(func() {
					close(entered)
					<-resume
				})
			}
			if boundary == "state-load" {
				binding, ok := s.selectedBinding()
				if !ok || binding.SessionOnly {
					t.Fatal("fixture requires a persistent selected binding")
				}
				// Re-enter state loading through the existing persistence boundary.
				// Apply owns operationMu here but has not acquired state.applyMu.
				s.mu.Lock()
				delete(s.remapComponent.states, binding.ID)
				s.remapPersistence = remapSelectionPersistence{
					load: func(Binding) (device.DeviceConfig, error) {
						gate()
						return device.DeviceConfig{}, nil
					},
				}
				s.mu.Unlock()
			} else {
				c.during = gate
			}
			result := make(chan RemapSnapshot, 1)
			go func() { result <- s.ApplyRemap(config) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("Apply did not reach the synchronization boundary")
			}
			// Channel receipt orders this mutation after the initial copy; a failed
			// TryLock cannot establish that happens-before relationship.
			config.Buttons[3].Action = device.RemapOff
			release()
			var got RemapSnapshot
			select {
			case got = <-result:
			case <-time.After(time.Second):
				t.Fatal("Apply did not finish after releasing the boundary")
			}
			if got.Error.Code != "" || !reflect.DeepEqual(got.Applied, want) || !reflect.DeepEqual(got.Pending, want) {
				t.Fatalf("caller config leaked through boundary wait: %+v", got)
			}
			if boundary == "macro-transport" {
				if c.composite != 1 || c.normal != 0 || !reflect.DeepEqual(c.batch.Setup, multiSetup(t, want, 6)) {
					t.Fatalf("macro command changed: %x calls=%d/%d", c.batch.Setup, c.composite, c.normal)
				}
			} else if c.normal == 0 || c.composite != 0 {
				t.Fatalf("ordinary remap command changed: calls=%d/%d", c.composite, c.normal)
			}
		})
	}
}

// plainMacroCommand has no batch support, like a transport that only sends plain reports.
type plainMacroCommand struct{ fake *macroCommandFake }

func (p plainMacroCommand) SendAndAwaitBound(ctx context.Context, b Binding, report []byte, keepReading func([]byte) bool) error {
	return p.fake.SendAndAwaitBound(ctx, b, report, keepReading)
}

func attachPlainCommand(s *Service, c *macroCommandFake) {
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	candidates := []transport.Candidate{{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "alpha", Path: "/dev/mock-a"}}
	s.AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, plainMacroCommand{c}))
	s.SelectDevice(s.RefreshInventory(context.Background()).Devices[0].ID)
}

func TestMacroTimedStageApplyCapability(t *testing.T) {
	s, c, m := macroFixture(t)
	attachPlainCommand(s, c)
	events := []macros.Event{{Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up, DelayMS: 100}}
	m, err := s.UpdateMacro(m.ID, "captured chord", events)
	if err != nil {
		t.Fatal(err)
	}
	staged := s.StageMacroAssignmentWithMode(m.ID, 6, 2, 1)
	if staged.Error.Code != "" || staged.MacroPending == nil || c.composite+c.normal != 0 {
		t.Fatalf("stage rejected or wrote: %+v", staged)
	}
	// A command without batch support must never receive a macro payload.
	got := s.ApplyRemap(staged.Pending)
	if got.Error.Code == "" || c.composite+c.normal != 0 || got.MacroProgress != (mouse.MacroProgress{}) || !reflect.DeepEqual(got.MacroPending, staged.MacroPending) {
		t.Fatalf("missing capability mutated: %+v", got)
	}
}

// wantLocalBatch encodes the expected mixed local upload: item one is a legacy
// click macro (repeat 255), item two the captured chord (repeat 1, mode 2).
func wantLocalBatch(t *testing.T, config device.RemapConfig, first, second []macros.Event) mouse.Batch {
	t.Helper()
	batch, err := testModel().MacroAssignmentBatch(config, []device.MacroSpec{{Button: 6, Events: first, Repeat: 255}, {Button: 7, Events: second, Repeat: 1, Mode: 2}})
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

// sameBytes compares the encoded reports; the batch's callbacks are not comparable.
func sameBytes(got, want mouse.Batch) bool {
	return reflect.DeepEqual(got.Setup, want.Setup) && reflect.DeepEqual(got.Chunks, want.Chunks) && got.Spacing == want.Spacing
}

type localMacroCommandFake struct {
	*macroCommandFake
}

func localMacroFixture(t *testing.T) (*Service, *localMacroCommandFake, macros.Macro) {
	s, c, m := macroFixture(t)
	local := &localMacroCommandFake{macroCommandFake: c}
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	candidates := []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "alpha", Path: "/dev/mock-a"}, {VendorID: 0x1d57, ProductID: 0xfa60, Serial: "beta", Path: "/dev/mock-b"}}
	s.AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, local))
	inventory := s.RefreshInventory(context.Background())
	s.SelectDevice(inventory.Devices[0].ID)
	return s, local, m
}

// This command seam uses the real serializers; HID's private opener/wait seam
// is covered independently in hidlinux, without opening a real node here.
type officialSerializedCommand struct {
	*localMacroCommandFake
	reports [][]byte
}

func (f *officialSerializedCommand) SendBatchBound(ctx context.Context, binding Binding, batch mouse.Batch) (mouse.MacroProgress, error) {
	f.reports = append([][]byte{batch.Setup}, batch.Chunks...)
	return f.localMacroCommandFake.SendBatchBound(ctx, binding, batch)
}
func officialFDSTake() []macros.Event {
	var events []macros.Event
	for i, code := range []string{"KeyF", "KeyD", "KeyS", "KeyF", "KeyD"} {
		events = append(events, macros.Event{Type: macros.Keyboard, KeyCode: code, Action: macros.Down, DelayMS: int64(100 * (i + 1))}, macros.Event{Type: macros.Keyboard, KeyCode: code, Action: macros.Up, DelayMS: int64(20 + 10*i)})
	}
	events[9].DelayMS = 600 // retained locally; terminal writer must encode 10
	return events
}
func TestMacroOfficialFDSSavedSelectedStagedApplied(t *testing.T) {
	s, local, _ := localMacroFixture(t)
	command := &officialSerializedCommand{localMacroCommandFake: local}
	registry, _ := mouse.NewProfileRegistry(newStubProfile())
	candidates := []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "alpha", Path: "/dev/mock-a"}}
	s.AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command))
	inventory := s.RefreshInventory(context.Background())
	s.SelectDevice(inventory.Devices[0].ID)
	events := officialFDSTake()
	m, err := s.CreateMacro("F/D/S ten", events)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListMacros()
	if err != nil || len(listed) != 2 {
		t.Fatal(listed, err)
	}
	saved, err := s.ReadMacro(m.ID)
	if err != nil || !reflect.DeepEqual(saved.Events, events) {
		t.Fatal(saved, err)
	}
	staged := s.StageMacroAssignmentWithMode(m.ID, 7, 3, 2)
	if staged.Error.Code != "" || !reflect.DeepEqual(staged.MacroPending.Events, events) || len(command.reports) != 0 || command.normal+command.composite != 0 {
		t.Fatal(staged, command)
	}
	config := cloneRemapConfig(staged.Pending)
	config.Buttons[3].Action = device.RemapFire
	applied := s.ApplyRemap(config)
	if applied.Error.Code != "" || applied.Firmware != "success" || applied.MacroProgress != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}) || !reflect.DeepEqual(applied.Applied, config) || !reflect.DeepEqual(applied.MacroApplied.Events, events) || command.composite != 1 || len(command.reports) != 2 {
		t.Fatal(applied, command)
	}
	// The model owns the byte layout of the uploaded reports; the real X6
	// encoding of this take is pinned by TestOfficialHIDFDSTenEventReports.
	want, err := testModel().MacroAssignmentBatch(config, []device.MacroSpec{{Button: 7, Events: events, Repeat: 3, Mode: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(command.reports, append([][]byte{want.Setup}, want.Chunks...)) {
		t.Fatalf("reports sent to the command differ from the model batch: %x", command.reports)
	}
	saved, _ = s.ReadMacro(m.ID)
	if !reflect.DeepEqual(saved.Events, events) {
		t.Fatal("Apply rewrote source delays")
	}
}

func TestMacroOfficialInvalidStageAndApplyNoMutation(t *testing.T) {
	key := func(code string, action macros.Action) macros.Event {
		return macros.Event{Type: macros.Keyboard, KeyCode: code, Action: action, DelayMS: 10}
	}
	for _, tt := range []struct {
		name   string
		events []macros.Event
	}{
		{"simultaneous aliases", []macros.Event{key("Enter", macros.Down), key("NumpadEnter", macros.Down), key("Enter", macros.Up), key("NumpadEnter", macros.Up)}},
		{"alias release", []macros.Event{key("Enter", macros.Down), key("NumpadEnter", macros.Up)}},
		{"repeated down", []macros.Event{key("Enter", macros.Down), key("Enter", macros.Down), key("Enter", macros.Up)}},
		{"prototype identity", []macros.Event{key("constructor", macros.Down), key("constructor", macros.Up)}},
		{"oversize", repeatLocalClicks(24)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, c, valid := localMacroFixture(t)
			staged := s.StageMacroAssignment(valid.ID, 6, 1)
			invalid, err := s.CreateMacro(tt.name, tt.events)
			if err != nil {
				t.Fatal(err)
			}
			before := s.GetRemapSnapshot()
			rejected := s.StageMacroAssignment(invalid.ID, 7, 1)
			if rejected.Error.Code == "" || rejected.Revision != before.Revision || !reflect.DeepEqual(rejected.MacroDrafts, before.MacroDrafts) || rejected.MacroProgress != before.MacroProgress || !reflect.DeepEqual(rejected.Pending, before.Pending) {
				t.Fatal(before, rejected)
			}
			// Simulate a malformed staged snapshot to prove Apply independently gates it.
			state := s.currentRemapState()
			state.mu.Lock()
			state.macroPendingMap[6].ID = invalid.ID
			state.macroPendingMap[6].Name = invalid.Name
			state.macroPendingMap[6].Events = append([]macros.Event(nil), invalid.Events...)
			state.mu.Unlock()
			before = s.GetRemapSnapshot()
			rejected = s.ApplyRemap(staged.Pending)
			if rejected.Error.Code == "" || rejected.Revision != before.Revision || !reflect.DeepEqual(rejected.MacroDrafts, before.MacroDrafts) || rejected.MacroProgress != before.MacroProgress || !reflect.DeepEqual(rejected.Pending, before.Pending) || c.normal+c.composite != 0 {
				t.Fatal(before, rejected, c)
			}
			saved, _ := s.ReadMacro(invalid.ID)
			if !reflect.DeepEqual(saved.Events, tt.events) {
				t.Fatal("local source lost")
			}
		})
	}
}

func keyboardTake() []macros.Event {
	return []macros.Event{{Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up, DelayMS: 100}}
}

func TestMacroLocalMixedApplyAndStaleCompletion(t *testing.T) {
	for _, mutation := range []string{"none", "library", "selection", "draft", "partial", "missing ACK"} {
		t.Run(mutation, func(t *testing.T) {
			s, c, m := localMacroFixture(t)
			chord, err := s.CreateMacro("Captured chord", keyboardTake())
			if err != nil {
				t.Fatal(err)
			}
			s.StageMacroAssignment(m.ID, 6, 255)
			staged := s.StageMacroAssignmentWithMode(chord.ID, 7, 1, 2)
			config := cloneRemapConfig(staged.Pending)
			config.Buttons[3].Action = device.RemapFire
			before := staged.Applied
			c.during = func() {
				switch mutation {
				case "library":
					_, _ = s.UpdateMacro(chord.ID, "renamed", chord.Events)
				case "selection":
					inventory := s.RefreshInventory(context.Background())
					s.SelectDevice(inventory.Devices[1].ID)
				case "draft":
					s.ClearButtonMacroAssignment(7)
				}
			}
			if mutation == "partial" {
				c.err = errors.New("write failure")
				c.progress.Upload = mouse.MacroUploadPossiblyPartial
			}
			if mutation == "missing ACK" {
				c.progress.Upload = mouse.MacroUploadNotStarted
			}
			got := s.ApplyRemap(config)
			if c.composite != 1 || c.normal != 0 || !reflect.DeepEqual(c.batch.Setup, multiSetup(t, config, 6, 7)) || !sameBytes(c.batch, wantLocalBatch(t, config, m.Events, chord.Events)) || got.MacroProgress != c.progress {
				t.Fatalf("mixed transport contract: %+v command=%+v", got, c)
			}
			if mutation == "none" {
				if got.Error.Code != "" || got.Firmware != "success" || len(got.MacroAppliedDrafts) != 2 || got.Persistence != "not_supported" {
					t.Fatal(got)
				}
			} else {
				if got.Error.Code == "" || got.Firmware != "failed" || !reflect.DeepEqual(got.Applied, before) || len(got.MacroAppliedDrafts) != 0 {
					t.Fatal(got)
				}
				if mutation == "library" || mutation == "selection" || mutation == "draft" {
					if got.Error.Code != StaleBinding {
						t.Fatal(got.Error)
					}
				}
			}
			s.RetryRemapPersistence()
			if c.composite != 1 {
				t.Fatal("automatic transport retry")
			}
		})
	}
}

func TestMacroPlaybackModes(t *testing.T) {
	for _, mode := range []uint8{0, 1, 2} {
		s, c, m := macroFixture(t)
		got := s.StageMacroAssignmentWithMode(m.ID, 6, 1, mode)
		if got.Error.Code != "" || got.MacroPending.Mode != mode || c.composite+c.normal != 0 {
			t.Fatal(got)
		}
		want := cloneMacroDraft(got.MacroPending)
		got.MacroPending.Mode = 255
		for _, invalid := range []uint8{3, 255} {
			bad := s.StageMacroAssignmentWithMode(m.ID, 6, 1, invalid)
			if bad.Error.Code != InvalidConfiguration || !reflect.DeepEqual(bad.MacroPending, want) || c.composite+c.normal != 0 {
				t.Fatal(bad)
			}
		}
		applied := s.ApplyRemap(got.Pending)
		if applied.Error.Code != "" || applied.MacroApplied.Mode != mode || c.sequence.Mode != mode {
			t.Fatal(applied, c.sequence)
		}
		legacy := s.StageMacroAssignment(m.ID, 6, 255)
		if legacy.Error.Code != "" || legacy.MacroPending.Mode != 0 || legacy.MacroPending.Repeat != 255 {
			t.Fatal(legacy)
		}
	}
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
					if got.Error.Code != "" || got.Firmware != "success" || !reflect.DeepEqual(got.MacroApplied, want) || c.destination != destinationFor(t, destination) || c.sequence.Repeat != repeat || !reflect.DeepEqual(c.sequence.Buttons, []macros.EventType{first, second}) || c.composite != 1 || c.normal != 0 {
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

func repeatLocalClicks(count int) []macros.Event {
	var events []macros.Event
	for i := 0; i < count; i++ {
		events = append(events, macros.Event{Type: macros.MouseLeft, Action: macros.Down}, macros.Event{Type: macros.MouseLeft, Action: macros.Up})
	}
	return events
}

func TestMacroExpandedStageRetainsBalancedLayouts(t *testing.T) {
	cases := [][]macros.Event{
		repeatLocalClicks(3),
		{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: 10}, {Type: macros.MouseLeft, Action: macros.Up, DelayMS: 10}},
		{{Type: macros.Keyboard, KeyCode: "ControlLeft", Action: macros.Down, DelayMS: 1800}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down}, {Type: macros.MouseRight, Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up}, {Type: macros.MouseRight, Action: macros.Up}, {Type: macros.Keyboard, KeyCode: "ControlLeft", Action: macros.Up}},
	}
	for i, events := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			s, c, m := macroFixture(t)
			m, err := s.UpdateMacro(m.ID, m.Name, events)
			if err != nil {
				t.Fatal(err)
			}
			got := s.StageMacroAssignment(m.ID, 6, 255)
			if got.Error.Code != "" || !reflect.DeepEqual(got.MacroPending.Events, events) || c.normal+c.composite != 0 {
				t.Fatal(got)
			}
			after, _ := s.ReadMacro(m.ID)
			if !reflect.DeepEqual(after, m) {
				t.Fatal("staging edited library")
			}
		})
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
		// T4 admits longer balanced/timed takes; preserve overflow rejection here.
		{"source event overflow", repeatLocalClicks(24), 1, 1},
		{"second pair timing overflow", append(append([]macros.Event{}, valid...), macros.Event{Type: macros.MouseRight, Action: macros.Down}, macros.Event{Type: macros.MouseRight, Action: macros.Up, DelayMS: 51200}), 1, 1},
		{"second pair reversed", append(append([]macros.Event{}, valid...), valid[1], valid[0]), 1, 1},
		{"second pair unmatched", append(append([]macros.Event{}, valid...), valid[0], macros.Event{Type: macros.MouseRight, Action: macros.Up}), 1, 1},
		{"different buttons", []macros.Event{valid[0], {Type: macros.MouseRight, Action: macros.Up}}, 1, 1},
		{"reversed", []macros.Event{valid[1], valid[0]}, 1, 1},
		{"two downs", []macros.Event{valid[0], valid[0]}, 1, 1},
		{"down timing overflow", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: 51200}, valid[1]}, 1, 1},
		{"up timing overflow", []macros.Event{valid[0], {Type: macros.MouseLeft, Action: macros.Up, DelayMS: 51200}}, 1, 1},
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
	pending.Buttons[3].Action = device.RemapFire
	s.StageMacroAssignment(m.ID, 7, 255)
	got := s.ApplyRemap(pending)
	if got.Error.Code != "" || got.Firmware != "success" || got.MacroApplied == nil || got.MacroProgress != c.progress || got.Persistence != "not_supported" || c.composite != 1 || c.normal != 0 {
		t.Fatalf("%+v command=%+v", got, c)
	}
	if !reflect.DeepEqual(c.batch.Setup, multiSetup(t, pending, 7)) || c.sequence.Repeat != 255 {
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
	config.Buttons[3].Action = device.RemapFire
	s.StageRemap(config)
	staged := s.StageMacroAssignment(m.ID, 7, 3)
	if !reflect.DeepEqual(staged.Pending, config) {
		t.Fatal("macro staging lost ordinary draft")
	}
	config.Buttons[4].Action = device.RemapDoubleClick
	staged = s.StageRemap(config)
	if staged.MacroPending == nil || staged.MacroPending.Button != 7 {
		t.Fatal("ordinary staging lost macro overlay")
	}
	invalid := s.StageRemap(device.RemapConfig{})
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
	got := s.ApplyRemap(device.DefaultRemapConfig())
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
			pending.Buttons[4].Action = device.RemapFire
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
	if s.ApplyRemap(device.DefaultRemapConfig()).Firmware != "failed" {
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
	if c.composite != 1 || !reflect.DeepEqual(c.batch.Setup, multiSetup(t, staged7.Pending, 6, 7)) {
		t.Fatalf("command mismatch: calls=%d setup=%x", c.composite, c.batch.Setup)
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
