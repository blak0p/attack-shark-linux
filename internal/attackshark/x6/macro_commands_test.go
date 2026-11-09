package x6

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/attackshark/x6/macrocodec"
	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

var macroClick = macrocodec.X6Click{Button: macros.MouseLeft, Repeat: 1}

// batchCommand records the batches the X6 helpers hand to the generic seam.
type batchCommand struct {
	batches  []mouse.Batch
	progress mouse.MacroProgress
	err      error
}

func (c *batchCommand) SendAndAwaitBound(context.Context, mouse.Binding, []byte, func([]byte) bool) error {
	return nil
}
func (c *batchCommand) SendBatchBound(_ context.Context, _ mouse.Binding, batch mouse.Batch) (mouse.MacroProgress, error) {
	c.batches = append(c.batches, batch)
	return c.progress, c.err
}

type fixedInventory struct{}

func (fixedInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}, nil
}

func macroService(t *testing.T, command mouse.TargetedCommand) (*mouse.TargetedService, mouse.Binding) {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	service := mouse.NewTargetedService(registry, fixedInventory{}, command)
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, ok := service.Selection()
	if !ok {
		t.Fatal("no selected binding")
	}
	return service, binding
}

func keyEvent(code string, action macros.Action, delay int64) macros.Event {
	return macros.Event{Type: macros.Keyboard, KeyCode: code, Action: action, DelayMS: delay}
}

func TestMacroBatchPolicy(t *testing.T) {
	batch, err := MacroAssignmentBatch(protocol.MacroAssignment{Config: protocol.DefaultRemapConfig(), Button: 6}, macroClick)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Spacing != 1200*time.Millisecond || batch.Status == nil || batch.SetupKeepReading == nil {
		t.Fatalf("batch policy = %+v", batch)
	}
	// Only the report08 ACK ends the setup wait; the DPI ACK is skipped.
	if batch.SetupKeepReading([]byte{3, 0x10, 0x50, 0, 8}) || !batch.SetupKeepReading([]byte{3, 0x10, 0x50, 0, 4}) || !batch.SetupKeepReading(nil) {
		t.Fatal("setup ACK matcher is not the remap ACK")
	}
}

func TestMacroStatusClassification(t *testing.T) {
	for _, tt := range []struct {
		report []byte
		want   mouse.BatchStatus
	}{
		{[]byte{3, 0x10, 0x50, 0, 9}, mouse.StatusDone},
		// Captured idle heartbeat: docs/protocol-x6.md, status report.
		{[]byte{3, 0x10, 0x40, 1, 0x0a}, mouse.StatusIgnore},
		{[]byte{3, 0x10, 0x50, 0, 4}, mouse.StatusIgnore},
		{[]byte{3, 0x10, 0x50, 0, 5}, mouse.StatusIgnore},
		{[]byte{3, 0x10, 0x50, 0, 6}, mouse.StatusIgnore},
		{[]byte{3, 0x10, 0x50, 0, 8}, mouse.StatusIgnore},
		{[]byte{3, 0x10, 0x1e, 0, 0xaa}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x40, 1}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x40, 1, 0x0a, 0}, mouse.StatusUnexpected},
		{[]byte{2, 0x10, 0x40, 1, 0x0a}, mouse.StatusUnexpected},
		{[]byte{3, 0x11, 0x40, 1, 0x0a}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x50, 1, 4}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x50, 1, 9}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x50, 0}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x50, 0, 9, 0}, mouse.StatusUnexpected},
		{[]byte{3, 0x10, 0x50, 0, 7}, mouse.StatusUnexpected},
		{nil, mouse.StatusUnexpected},
	} {
		if got := macroStatus(tt.report); got != tt.want {
			t.Fatalf("macroStatus(%x) = %d, want %d", tt.report, got, tt.want)
		}
	}
}

func TestProfileDeclaresTheFourAcceptedFeatureReports(t *testing.T) {
	for _, profile := range []mouse.Profile{NewProfile(), NewWiredProfile()} {
		facts := profile.HIDFacts()
		for _, ok := range [][2]int{{0x04, 56}, {0x06, 9}, {0x05, 13}, {0x08, 59}} {
			payload := make([]byte, ok[1])
			payload[0] = byte(ok[0])
			if !facts.AcceptsFeatureReport(payload) || facts.AcceptsFeatureReport(payload[:len(payload)-1]) {
				t.Fatalf("report %#x/%d admission wrong", ok[0], ok[1])
			}
		}
		// report09 macro chunks are never admitted as plain bound commands.
		if facts.AcceptsFeatureReport(append([]byte{9}, make([]byte, 63)...)) || facts.AcceptsFeatureReport(nil) {
			t.Fatal("undeclared report admitted")
		}
	}
}

func TestMacroClickBatchMatchesStrictTemplate(t *testing.T) {
	batch, err := MacroClickBatch(macroClick)
	want, _ := macrocodec.EncodeX6Reports(macroClick)
	if err != nil || batch.Setup != nil || !reflect.DeepEqual(batch.Chunks, want) || batch.Spacing != MacroSpacing {
		t.Fatal(err, batch)
	}
	for _, click := range []macrocodec.X6Click{{}, {Button: macros.MouseLeft, Repeat: 256}, {Button: macros.MouseRight, Repeat: 0}} {
		command := &chunkCommand{}
		if err := SendMacroClickBound(context.Background(), command, mouse.Binding{}, click); err == nil || command.calls != 0 {
			t.Fatal("invalid template accepted")
		}
	}
	command := &chunkCommand{}
	if err := SendMacroClickBound(context.Background(), command, mouse.Binding{}, macroClick); err != nil || command.calls != 1 || !reflect.DeepEqual(command.batch.Chunks, want) {
		t.Fatal(err, command)
	}
}

type chunkCommand struct {
	calls int
	batch mouse.Batch
}

func (c *chunkCommand) SendChunksBound(_ context.Context, _ mouse.Binding, batch mouse.Batch) error {
	c.calls++
	c.batch = batch
	return nil
}

func TestMacroAssignmentBatchOrderPerButton(t *testing.T) {
	for button := uint8(1); button <= 7; button++ {
		assignment := protocol.MacroAssignment{Config: protocol.DefaultRemapConfig(), Button: button}
		batch, err := MacroAssignmentBatch(assignment, macroClick)
		report, _ := protocol.EncodeMacroAssignmentReport(assignment)
		destination, _ := protocol.MacroDestinationForButton(button)
		chunks, _ := macrocodec.EncodeX6Upload(macrocodec.X6Upload{Destination: destination, Click: macroClick})
		if err != nil || !reflect.DeepEqual(batch.Setup, report) || !reflect.DeepEqual(batch.Chunks, chunks) {
			t.Fatalf("button %d: %v", button, err)
		}
	}
	for _, a := range []protocol.MacroAssignment{{}, {Config: protocol.DefaultRemapConfig(), Button: 8}} {
		if _, err := MacroAssignmentBatch(a, macroClick); err == nil {
			t.Fatal("invalid assignment")
		}
	}
	if _, err := MacroAssignmentBatch(protocol.MacroAssignment{Config: protocol.DefaultRemapConfig(), Button: 7}, macrocodec.X6Click{}); err == nil {
		t.Fatal("invalid click")
	}
}

func TestSequenceAssignmentBatchOrderModesAndGuards(t *testing.T) {
	a := protocol.MacroAssignment{Config: protocol.DefaultRemapConfig(), Button: 7}
	s := macrocodec.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 2}
	batch, err := MacroSequenceAssignmentBatch(a, s)
	r, _ := protocol.EncodeMacroAssignmentReport(a)
	chunks, _ := macrocodec.EncodeX6SequenceUpload(macrocodec.X6SequenceUpload{Destination: 6, Sequence: s})
	if err != nil || !reflect.DeepEqual(batch.Setup, r) || !reflect.DeepEqual(batch.Chunks, chunks) {
		t.Fatal(err, batch)
	}
	for _, mode := range []uint8{1, 2, 3, 255} {
		seq := macrocodec.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft}, Repeat: 1, Mode: mode}
		batch, err := MacroSequenceAssignmentBatch(a, seq)
		if mode > 2 {
			if err == nil {
				t.Fatalf("mode %d accepted", mode)
			}
			continue
		}
		got, derr := macrocodec.DecodeX6SequenceUpload(batch.Chunks)
		if err != nil || derr != nil || len(batch.Chunks) != 3 || got.Sequence.Mode != mode || got.Sequence.Repeat != 1 || batch.Chunks[2][1] != 12 {
			t.Fatal(got, err, derr)
		}
	}
	for _, invalid := range []macrocodec.X6Sequence{{}, {Buttons: s.Buttons, Repeat: 256}, {Buttons: []macros.EventType{"unknown"}, Repeat: 1}, {Buttons: append(append([]macros.EventType(nil), s.Buttons...), macros.MouseLeft), Repeat: 1}} {
		if _, err := MacroSequenceAssignmentBatch(a, invalid); err == nil {
			t.Fatalf("invalid sequence %+v accepted", invalid)
		}
	}
}

func TestMultiMacroSequenceBatchOrderAndGuards(t *testing.T) {
	cfg := protocol.DefaultRemapConfig()
	s1 := macrocodec.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	s2 := macrocodec.X6Sequence{Buttons: []macros.EventType{macros.MouseMiddle, macros.MouseBack}, Repeat: 2}
	items := []MacroSequenceItem{{Button: 6, Sequence: s1}, {Button: 7, Sequence: s2}}
	batch, err := MultiMacroSequenceBatch(cfg, items)
	report, _ := protocol.EncodeMultiMacroAssignmentReport(cfg, []uint8{6, 7})
	chunks1, _ := macrocodec.EncodeX6SequenceUpload(macrocodec.X6SequenceUpload{Destination: 5, Sequence: s1})
	chunks2, _ := macrocodec.EncodeX6SequenceUpload(macrocodec.X6SequenceUpload{Destination: 6, Sequence: s2})
	if err != nil || !reflect.DeepEqual(batch.Setup, report) || !reflect.DeepEqual(batch.Chunks, append(chunks1, chunks2...)) {
		t.Fatal(err, batch)
	}
	for name, bad := range map[string][]MacroSequenceItem{
		"empty":     nil,
		"duplicate": {{Button: 6, Sequence: s1}, {Button: 6, Sequence: s2}},
		"button":    {{Button: 8, Sequence: s1}},
		"sequence":  {{Button: 6, Sequence: macrocodec.X6Sequence{}}, {Button: 7, Sequence: s2}},
	} {
		if _, err := MultiMacroSequenceBatch(cfg, bad); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	badConfig := protocol.DefaultRemapConfig()
	badConfig.Buttons = nil
	if _, err := MultiMacroSequenceBatch(badConfig, items); err == nil {
		t.Fatal("invalid config accepted")
	}
}

func localItems() []MacroLocalItem {
	return []MacroLocalItem{
		{Button: 6, Events: []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}}, Repeat: 1},
		{Button: 7, Events: []macros.Event{keyEvent("KeyA", macros.Down, 100), keyEvent("KeyA", macros.Up, 100)}, Repeat: 2, Mode: 1},
	}
}

func TestMultiMacroLocalBatchAdmitsEverythingBeforeIO(t *testing.T) {
	for _, mutation := range []string{"empty", "config", "duplicate", "destination", "unsupported", "unbalanced", "overflow", "oversize", "repeat", "zero repeat", "mode", "stale mouse identity"} {
		t.Run(mutation, func(t *testing.T) {
			items := localItems()
			config := protocol.DefaultRemapConfig()
			switch mutation {
			case "empty":
				items = nil
			case "config":
				config.Buttons[0].Action = "invalid"
			case "duplicate":
				items[1].Button = 6
			case "destination":
				items[1].Button = 8
			case "unsupported":
				items[1].Events[0].KeyCode = "F13"
			case "unbalanced":
				items[1].Events = items[1].Events[:1]
			case "overflow":
				items[1].Events[1].DelayMS = 51200
			case "oversize":
				items[1].Events = nil
				for i := 0; i < 24; i++ {
					items[1].Events = append(items[1].Events, keyEvent("KeyF", macros.Down, 0), keyEvent("KeyF", macros.Up, 0))
				}
			case "repeat":
				items[1].Repeat = 256
			case "zero repeat":
				items[1].Repeat = 0
			case "mode":
				items[1].Mode = 3
			case "stale mouse identity":
				items[0].Events[0].KeyCode = "KeyA"
			}
			if batch, err := MultiMacroLocalBatch(config, items); err == nil {
				t.Fatalf("accepted: %+v", batch)
			}
			// The same inputs never reach the command through the service either.
			command := &batchCommand{}
			service, binding := macroService(t, command)
			before, _ := service.State(binding.ID)
			progress, err := ApplyMultiMacroLocalAssignmentBound(context.Background(), service, binding, config, items)
			after, _ := service.State(binding.ID)
			if err == nil || progress != (mouse.MacroProgress{}) || len(command.batches) != 0 || !reflect.DeepEqual(before, after) {
				t.Fatal(progress, err, command.batches)
			}
		})
	}
}

func TestMultiMacroLocalCapturedPackets(t *testing.T) {
	events := []macros.Event{keyEvent("ShiftLeft", macros.Down, 100), keyEvent("KeyA", macros.Down, 100), keyEvent("KeyA", macros.Up, 100), keyEvent("ShiftLeft", macros.Up, 100)}
	batch, err := MultiMacroLocalBatch(protocol.DefaultRemapConfig(), []MacroLocalItem{{Button: 6, Events: events, Repeat: 1}})
	if err != nil || len(batch.Chunks) != 3 {
		t.Fatal(err, batch)
	}
	// Expected captured block assembled independently, not through codec helpers.
	block := make([]byte, 128)
	block[4] = 1
	block[25] = 4
	copy(block[26:], []byte{0x0b, 0xe1, 0x0b, 4, 0x8b, 4, 0x81, 0xe1})
	block[126] = 2
	block[127] = 0xf1
	for i, size := range []int{60, 60, 8} {
		want := make([]byte, 64)
		want[0] = 9
		want[1] = 0x40
		if i == 2 {
			want[1] = 12
		}
		want[2] = 5
		want[3] = byte(i)
		copy(want[4:], block[i*60:i*60+size])
		if !reflect.DeepEqual(batch.Chunks[i], want) {
			t.Fatalf("packet %d: %x want %x", i, batch.Chunks[i], want)
		}
	}
}

func officialFDSEvents() []macros.Event {
	var events []macros.Event
	for i, key := range []string{"KeyF", "KeyD", "KeyS", "KeyF", "KeyD"} {
		events = append(events, keyEvent(key, macros.Down, int64(100*(i+1))), keyEvent(key, macros.Up, int64(20+10*i)))
	}
	events[9].DelayMS = 600 // retained locally; the terminal writer encodes 10
	return events
}

func TestOfficialHIDFDSTenEventReports(t *testing.T) {
	events := officialFDSEvents()
	before := append([]macros.Event(nil), events...)
	config := protocol.DefaultRemapConfig()
	config.Buttons[3].Action = protocol.RemapFire
	batch, err := MultiMacroLocalBatch(config, []MacroLocalItem{{Button: 7, Events: events, Repeat: 3, Mode: 2}})
	if err != nil || len(batch.Chunks) != 3 || !reflect.DeepEqual(events, before) {
		t.Fatal(err, batch)
	}
	association, err := protocol.EncodeMultiMacroAssignmentReport(config, []uint8{7})
	if err != nil || !reflect.DeepEqual(batch.Setup, association) {
		t.Fatal("ordinary remap overlay lost", err)
	}
	// Literal expected official IDs/order/timing and checksum; not mapper-derived.
	block := make([]byte, 128)
	block[0], block[4], block[25] = 2, 3, 10
	copy(block[26:], []byte{0x0b, 9, 0x83, 9, 0x15, 7, 0x83, 7, 0x1f, 0x16, 0x85, 0x16, 0x29, 9, 0x85, 9, 0x33, 7, 0x81, 7})
	block[126], block[127] = 3, 0xa7
	for i, size := range []int{60, 60, 8} {
		want := make([]byte, 64)
		want[0], want[1], want[2], want[3] = 9, 0x40, 6, byte(i)
		if i == 2 {
			want[1] = 0x0c
		}
		copy(want[4:], block[i*60:i*60+size])
		if !reflect.DeepEqual(batch.Chunks[i], want) {
			t.Fatalf("HID report %d: %x want %x", i, batch.Chunks[i], want)
		}
	}
}

func TestOfficialHIDAliasesBeforeOpen(t *testing.T) {
	for _, events := range [][]macros.Event{
		{keyEvent("Enter", macros.Down, 10), keyEvent("NumpadEnter", macros.Up, 10)},
		{keyEvent("Enter", macros.Down, 10), keyEvent("NumpadEnter", macros.Down, 10), keyEvent("Enter", macros.Up, 10), keyEvent("NumpadEnter", macros.Up, 10)},
		{keyEvent("Enter", macros.Down, 10), keyEvent("Enter", macros.Down, 10), keyEvent("Enter", macros.Up, 10)},
		{keyEvent("__proto__", macros.Down, 10), keyEvent("__proto__", macros.Up, 10)},
	} {
		command := &batchCommand{}
		service, binding := macroService(t, command)
		snapshot := append([]macros.Event(nil), events...)
		before, _ := service.State(binding.ID)
		p, err := ApplyMultiMacroLocalAssignmentBound(context.Background(), service, binding, protocol.DefaultRemapConfig(), []MacroLocalItem{{Button: 7, Events: events, Repeat: 1}})
		after, _ := service.State(binding.ID)
		if err == nil || p != (mouse.MacroProgress{}) || len(command.batches) != 0 || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(events, snapshot) {
			t.Fatal(p, err)
		}
	}
	// Sequential use of the inferential aliases remains encodable at raw 0x28.
	events := []macros.Event{keyEvent("Enter", macros.Down, 10), keyEvent("Enter", macros.Up, 10), keyEvent("NumpadEnter", macros.Down, 10), keyEvent("NumpadEnter", macros.Up, 10)}
	batch, err := MultiMacroLocalBatch(protocol.DefaultRemapConfig(), []MacroLocalItem{{Button: 7, Events: events, Repeat: 1}})
	if err != nil || !reflect.DeepEqual(batch.Chunks[0][30:38], []byte{1, 0x28, 0x81, 0x28, 1, 0x28, 0x81, 0x28}) {
		t.Fatal(err, batch.Chunks)
	}
}

func TestMultiMacroLocalMixedOrderAndAssociation(t *testing.T) {
	config := protocol.DefaultRemapConfig()
	config.Buttons[3].Action = protocol.RemapFire
	batch, err := MultiMacroLocalBatch(config, localItems())
	if err != nil || len(batch.Chunks) != 6 {
		t.Fatal(err, batch)
	}
	wantAssociation := []byte{8, 0x3b, 1, 2, 0, 0, 3, 0, 0, 4, 0, 0, 0x0d, 0, 0, 0x12, 0, 5, 0x12, 0, 6, 8, 0, 0, 5, 0, 0, 0x3c, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 0x0a, 0, 0, 9, 0, 0, 0, 0xa8}
	if !reflect.DeepEqual(batch.Setup, wantAssociation) {
		t.Fatalf("association: %x want %x", batch.Setup, wantAssociation)
	}
	// First item is the unchanged legacy click; second retains timed order/mode/repeat.
	if !reflect.DeepEqual(batch.Chunks[0][30:34], []byte{1, 0xf1, 0x81, 0xf1}) || !reflect.DeepEqual(batch.Chunks[3][:4], []byte{9, 0x40, 6, 0}) || batch.Chunks[3][4] != 1 || batch.Chunks[3][8] != 2 || !reflect.DeepEqual(batch.Chunks[3][30:34], []byte{0x0b, 4, 0x81, 4}) {
		t.Fatalf("mixed payload mismatch: %x", batch.Chunks)
	}
}

func TestApplyHelpersReachCommandOnlyWhenValid(t *testing.T) {
	want := mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}
	command := &batchCommand{progress: want}
	service, binding := macroService(t, command)
	a := protocol.MacroAssignment{Config: protocol.DefaultRemapConfig(), Button: 7}
	s := macrocodec.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	items := []MacroSequenceItem{{Button: 6, Sequence: s}, {Button: 7, Sequence: s}}
	stale := binding
	stale.Path = "other"
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	for name, apply := range map[string]func(context.Context, mouse.Binding, bool) (mouse.MacroProgress, error){
		"click": func(ctx context.Context, b mouse.Binding, valid bool) (mouse.MacroProgress, error) {
			click := macroClick
			if !valid {
				click = macrocodec.X6Click{}
			}
			return ApplyMacroAssignmentBound(ctx, service, b, a, click)
		},
		"sequence": func(ctx context.Context, b mouse.Binding, valid bool) (mouse.MacroProgress, error) {
			seq := s
			if !valid {
				seq = macrocodec.X6Sequence{}
			}
			return ApplyMacroSequenceAssignmentBound(ctx, service, b, a, seq)
		},
		"multi sequence": func(ctx context.Context, b mouse.Binding, valid bool) (mouse.MacroProgress, error) {
			list := items
			if !valid {
				list = nil
			}
			return ApplyMultiMacroSequenceAssignmentBound(ctx, service, b, a.Config, list)
		},
		"multi local": func(ctx context.Context, b mouse.Binding, valid bool) (mouse.MacroProgress, error) {
			list := localItems()
			if !valid {
				list = nil
			}
			return ApplyMultiMacroLocalAssignmentBound(ctx, service, b, a.Config, list)
		},
	} {
		t.Run(name, func(t *testing.T) {
			command.batches = nil
			if _, err := apply(context.Background(), binding, false); err == nil || len(command.batches) != 0 {
				t.Fatal("invalid payload reached transport")
			}
			if _, err := apply(context.Background(), stale, true); !errors.Is(err, mouse.ErrStaleBinding) || len(command.batches) != 0 {
				t.Fatal(err)
			}
			if _, err := apply(canceled, binding, true); !errors.Is(err, context.Canceled) || len(command.batches) != 0 {
				t.Fatal(err)
			}
			p, err := apply(context.Background(), binding, true)
			if err != nil || p != want || len(command.batches) != 1 || len(command.batches[0].Setup) == 0 {
				t.Fatal(p, err)
			}
		})
	}
}

func TestApplyHelpersWithoutBatchCapability(t *testing.T) {
	service, binding := macroService(t, plainCommand{})
	if service.BatchAvailable() {
		t.Fatal("plain command reported batch capability")
	}
	p, err := ApplyMultiMacroLocalAssignmentBound(context.Background(), service, binding, protocol.DefaultRemapConfig(), localItems())
	if err == nil || p != (mouse.MacroProgress{}) {
		t.Fatal(p, err)
	}
}

type plainCommand struct{}

func (plainCommand) SendAndAwaitBound(context.Context, mouse.Binding, []byte, func([]byte) bool) error {
	return nil
}
