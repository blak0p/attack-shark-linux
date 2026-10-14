package mouse

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type compositeCommandFake struct {
	commandFake
	calls int
}

func (c *compositeCommandFake) SendX6MacroAssignmentBound(context.Context, Binding, x6.MacroAssignment, macros.X6Click) (MacroProgress, error) {
	c.calls++
	return MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}, nil
}
func (c *compositeCommandFake) SendX6MacroSequenceAssignmentBound(_ context.Context, _ Binding, _ x6.MacroAssignment, _ macros.X6Sequence) (MacroProgress, error) {
	c.calls++
	return MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}, nil
}
func (c *compositeCommandFake) SendX6MultiMacroSequenceAssignmentBound(_ context.Context, _ Binding, _ x6.RemapConfig, items []MacroSequenceItem) (MacroProgress, error) {
	c.calls++
	return MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}, nil
}

type localCommandFake struct {
	commandFake
	localCalls int
	config     x6.RemapConfig
	items      []MacroLocalItem
	progress   MacroProgress
	err        error
	during     func()
}

func (c *localCommandFake) SendX6MultiMacroLocalAssignmentBound(_ context.Context, _ Binding, config x6.RemapConfig, items []MacroLocalItem) (MacroProgress, error) {
	c.localCalls++
	if c.during != nil {
		c.during()
	}
	c.config = config
	c.items = items
	return c.progress, c.err
}
func localServiceFixture(t *testing.T, command TargetedCommand) (*TargetedService, Binding) {
	t.Helper()
	registry, _ := NewProfileRegistry(targetedProfile{})
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}}, command)
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, _ := svc.Selection()
	return svc, binding
}

type officialLocalPacketCommand struct {
	localCommandFake
	reports [][]byte
}

func (c *officialLocalPacketCommand) SendX6MultiMacroLocalAssignmentBound(ctx context.Context, binding Binding, config x6.RemapConfig, items []MacroLocalItem) (MacroProgress, error) {
	buttons := make([]uint8, len(items))
	var chunks [][]byte
	for i, item := range items {
		buttons[i] = item.Button
		destination, err := x6.MacroDestinationForButton(item.Button)
		if err != nil {
			return MacroProgress{}, err
		}
		reports, err := macros.EncodeX6LocalUpload(destination, item.Events, item.Repeat, item.Mode)
		if err != nil {
			return MacroProgress{}, err
		}
		chunks = append(chunks, reports...)
	}
	report, err := x6.EncodeMultiMacroAssignmentReport(config, buttons)
	if err != nil {
		return MacroProgress{}, err
	}
	c.reports = append([][]byte{report}, chunks...)
	return c.localCommandFake.SendX6MultiMacroLocalAssignmentBound(ctx, binding, config, items)
}
func TestX6OfficialTargetedFDSPayload(t *testing.T) {
	command := &officialLocalPacketCommand{localCommandFake: localCommandFake{progress: MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}}}
	svc, binding := localServiceFixture(t, command)
	var events []macros.Event
	for i, key := range []string{"KeyF", "KeyD", "KeyS", "KeyF", "KeyD"} {
		events = append(events, macros.Event{Type: macros.Keyboard, KeyCode: key, Action: macros.Down, DelayMS: int64(100 * (i + 1))}, macros.Event{Type: macros.Keyboard, KeyCode: key, Action: macros.Up, DelayMS: int64(20 + 10*i)})
	}
	events[9].DelayMS = 600
	before := append([]macros.Event(nil), events...)
	config := x6.DefaultRemapConfig()
	config.Buttons[3].Action = x6.RemapFire
	progress, err := svc.ApplyMultiMacroLocalAssignmentBound(context.Background(), binding, config, []MacroLocalItem{{Button: 7, Events: events, Repeat: 3, Mode: 2}})
	if err != nil || progress != command.progress || command.localCalls != 1 || len(command.reports) != 4 || !reflect.DeepEqual(command.config, config) || !reflect.DeepEqual(events, before) {
		t.Fatal(progress, err, command)
	}
	expected := []byte{0x0b, 9, 0x83, 9, 0x15, 7, 0x83, 7, 0x1f, 0x16, 0x85, 0x16, 0x29, 9, 0x85, 9, 0x33, 7, 0x81, 7}
	if !bytes.Equal(command.reports[1][30:50], expected) || command.reports[1][4] != 2 || command.reports[1][8] != 3 || command.reports[1][29] != 10 {
		t.Fatalf("F/D/S order/timing: %x", command.reports)
	}
	for i, header := range [][]byte{{9, 0x40, 6, 0}, {9, 0x40, 6, 1}, {9, 0x0c, 6, 2}} {
		if !bytes.Equal(command.reports[i+1][:4], header) {
			t.Fatal("destination/chunks")
		}
	}
	if !bytes.Equal(command.reports[3][10:12], []byte{3, 0xa7}) {
		t.Fatal("terminal/checksum")
	}
}

func TestX6OfficialTargetedAliasRejectionBeforeMutation(t *testing.T) {
	for _, codes := range [][]string{{"Enter", "NumpadEnter"}, {"Enter", "Enter", "NumpadEnter", "NumpadEnter"}} {
		command := &officialLocalPacketCommand{}
		svc, binding := localServiceFixture(t, command)
		events := []macros.Event{{Type: macros.Keyboard, KeyCode: codes[0], Action: macros.Down}, {Type: macros.Keyboard, KeyCode: codes[1], Action: macros.Up}}
		if len(codes) == 4 {
			events = []macros.Event{{Type: macros.Keyboard, KeyCode: "Enter", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "NumpadEnter", Action: macros.Down}, {Type: macros.Keyboard, KeyCode: "Enter", Action: macros.Up}, {Type: macros.Keyboard, KeyCode: "NumpadEnter", Action: macros.Up}}
		}
		beforeEvents := append([]macros.Event(nil), events...)
		before, _ := svc.State(binding.ID)
		p, err := svc.ApplyMultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), []MacroLocalItem{{Button: 7, Events: events, Repeat: 1}})
		after, _ := svc.State(binding.ID)
		if err == nil || p != (MacroProgress{}) || command.localCalls != 0 || len(command.reports) != 0 || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(events, beforeEvents) {
			t.Fatal(p, err, before, after)
		}
	}
}

func localServiceItems() []MacroLocalItem {
	return []MacroLocalItem{{Button: 6, Events: []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}}, Repeat: 255}, {Button: 7, Events: []macros.Event{{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up}}, Repeat: 1, Mode: 2}}
}
func TestTargetedLocalPrevalidationAndCapability(t *testing.T) {
	for _, mutation := range []string{"empty", "config", "destination", "duplicate", "unsupported", "unbalanced", "overflow", "repeat", "mode", "mouse identity", "stale", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			command := &localCommandFake{}
			svc, binding := localServiceFixture(t, command)
			config := x6.DefaultRemapConfig()
			items := localServiceItems()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mutation {
			case "empty":
				items = nil
			case "config":
				config.Buttons = nil
			case "destination":
				items[1].Button = 8
			case "duplicate":
				items[1].Button = 6
			case "unsupported":
				items[1].Events[0].KeyCode = "F13"
			case "unbalanced":
				items[1].Events = items[1].Events[:1]
			case "overflow":
				items[1].Events[1].DelayMS = 51200
			case "repeat":
				items[1].Repeat = 0
			case "mode":
				items[1].Mode = 3
			case "mouse identity":
				items[0].Events[0].KeyCode = "KeyA"
			case "stale":
				binding.Path = "other"
			case "cancel":
				cancel()
			}
			before, _ := svc.State(binding.ID)
			p, err := svc.ApplyMultiMacroLocalAssignmentBound(ctx, binding, config, items)
			after, _ := svc.State(binding.ID)
			if err == nil || p != (MacroProgress{}) || command.localCalls+command.calls != 0 || !reflect.DeepEqual(before, after) {
				t.Fatal(p, err, before, after)
			}
		})
	}
	legacy := &compositeCommandFake{}
	svc, binding := localServiceFixture(t, legacy)
	if svc.MacroLocalAssignmentsAvailable() || !svc.MacroSequenceAssignmentsAvailable() {
		t.Fatal("wrong capabilities")
	}
	p, err := svc.ApplyMultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), localServiceItems())
	if err == nil || p != (MacroProgress{}) || legacy.calls != 0 {
		t.Fatal(p, err)
	}
}
func TestTargetedLocalFrozenCopiesAndPartialEvidence(t *testing.T) {
	command := &localCommandFake{progress: MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadPossiblyPartial}, err: errors.New("short write")}
	svc, binding := localServiceFixture(t, command)
	items := localServiceItems()
	config := x6.DefaultRemapConfig()
	config.Buttons[3].Action = x6.RemapFire
	command.during = func() {
		items[1].Events[0].KeyCode = "KeyB"
		items[1].Repeat = 255
		config.Buttons[3].Action = x6.RemapOff
	}
	p, err := svc.ApplyMultiMacroLocalAssignmentBound(context.Background(), binding, config, items)
	if err != command.err || p != command.progress || command.localCalls != 1 || command.items[1].Events[0].KeyCode != "KeyA" || command.items[1].Repeat != 1 || command.config.Buttons[3].Action != x6.RemapFire {
		t.Fatal(p, err, command)
	}
	if command.items[0].Button != 6 || command.items[1].Button != 7 {
		t.Fatal("ordered items lost")
	}
}

// Err is the earliest cancellation checkpoint after all inputs are frozen.
type queueCheckpointContext struct {
	context.Context
	entered, release chan struct{}
	once             sync.Once
}

func (c *queueCheckpointContext) Err() error {
	c.once.Do(func() { close(c.entered); <-c.release })
	return c.Context.Err()
}
func TestTargetedLocalFreezeBeforeQueueAndCancellation(t *testing.T) {
	for _, mutation := range []string{"copy", "cancel", "selection"} {
		t.Run(mutation, func(t *testing.T) {
			command := &localCommandFake{progress: MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}}
			svc, binding := localServiceFixture(t, command)
			_, state, _, _ := svc.selectedState()
			state.applyMu.Lock()
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &queueCheckpointContext{Context: base, entered: make(chan struct{}), release: make(chan struct{})}
			items := localServiceItems()
			config := x6.DefaultRemapConfig()
			type result struct {
				p   MacroProgress
				err error
			}
			done := make(chan result, 1)
			go func() {
				p, err := svc.ApplyMultiMacroLocalAssignmentBound(ctx, binding, config, items)
				done <- result{p, err}
			}()
			select {
			case <-ctx.entered:
			case <-time.After(time.Second):
				state.applyMu.Unlock()
				t.Fatal("no prequeue cancellation checkpoint")
			}
			items[1].Events[0].KeyCode = "KeyB"
			config.Buttons[0].Action = x6.RemapOff
			if mutation == "cancel" {
				cancel()
			}
			if mutation == "selection" {
				svc.mu.Lock()
				svc.selection = nil
				svc.mu.Unlock()
			}
			close(ctx.release)
			state.applyMu.Unlock()
			got := <-done
			if mutation == "copy" {
				if got.err != nil || command.localCalls != 1 || command.items[1].Events[0].KeyCode != "KeyA" || command.config.Buttons[0].Action != x6.RemapLeft {
					t.Fatal(got, command)
				}
			} else {
				if got.err == nil || command.localCalls != 0 || got.p != (MacroProgress{}) {
					t.Fatal(got)
				}
				if mutation == "cancel" && !errors.Is(got.err, context.Canceled) {
					t.Fatal(got.err)
				}
				if mutation == "selection" && !errors.Is(got.err, ErrStaleBinding) {
					t.Fatal(got.err)
				}
			}
		})
	}
}

func TestTargetedMultiMacroSequenceSeam(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	command := &compositeCommandFake{}
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}}, command)
	_, _ = svc.Refresh(context.Background())
	binding, _ := svc.Selection()
	cfg := x6.DefaultRemapConfig()
	s1 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	s2 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseMiddle, macros.MouseBack}, Repeat: 2}
	items := []MacroSequenceItem{
		{Button: 6, Sequence: s1},
		{Button: 7, Sequence: s2},
	}
	// Empty items rejected
	if _, err := svc.ApplyMultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, nil); err == nil || command.calls != 0 {
		t.Fatal("empty items accepted")
	}
	// Duplicate button rejected
	dup := []MacroSequenceItem{
		{Button: 6, Sequence: s1},
		{Button: 6, Sequence: s2},
	}
	if _, err := svc.ApplyMultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, dup); err == nil || command.calls != 0 {
		t.Fatal("duplicate button accepted")
	}
	// Invalid sequence rejected
	bad := []MacroSequenceItem{
		{Button: 6, Sequence: macros.X6Sequence{}},
		{Button: 7, Sequence: s2},
	}
	if _, err := svc.ApplyMultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, bad); err == nil || command.calls != 0 {
		t.Fatal("invalid sequence reached transport")
	}
	// Stale binding rejected
	stale := binding
	stale.Path = "other"
	if _, err := svc.ApplyMultiMacroSequenceAssignmentBound(context.Background(), stale, cfg, items); !errors.Is(err, ErrStaleBinding) {
		t.Fatal(err)
	}
	// Canceled context rejected
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ApplyMultiMacroSequenceAssignmentBound(ctx, binding, cfg, items); !errors.Is(err, context.Canceled) || command.calls != 0 {
		t.Fatal(err)
	}
	// Successful multi-macro assignment
	p, err := svc.ApplyMultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, items)
	if err != nil || command.calls != 1 || p.Upload != MacroUploadConfirmed {
		t.Fatal(p, err)
	}
}

func TestTargetedSequenceMacroSeam(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	command := &compositeCommandFake{}
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}}, command)
	_, _ = svc.Refresh(context.Background())
	binding, _ := svc.Selection()
	a := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}
	s := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	if _, err := svc.ApplyMacroSequenceAssignmentBound(context.Background(), binding, a, macros.X6Sequence{}); err == nil || command.calls != 0 {
		t.Fatal("invalid sequence reached transport")
	}
	stale := binding
	stale.Path = "other"
	if _, err := svc.ApplyMacroSequenceAssignmentBound(context.Background(), stale, a, s); !errors.Is(err, ErrStaleBinding) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ApplyMacroSequenceAssignmentBound(ctx, binding, a, s); !errors.Is(err, context.Canceled) || command.calls != 0 {
		t.Fatal(err)
	}
	p, err := svc.ApplyMacroSequenceAssignmentBound(context.Background(), binding, a, s)
	if err != nil || command.calls != 1 || p.Upload != MacroUploadConfirmed {
		t.Fatal(p, err)
	}
}

func TestTargetedCompositeMacroSeam(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	command := &compositeCommandFake{}
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}}, command)
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, _ := svc.Selection()
	assignment := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}
	click := macros.X6Click{Button: macros.MouseLeft, Repeat: 1}
	if _, err := svc.ApplyMacroAssignmentBound(context.Background(), binding, assignment, macros.X6Click{}); err == nil || command.calls != 0 {
		t.Fatal("invalid click reached transport")
	}
	stale := binding
	stale.Path = "other"
	if _, err := svc.ApplyMacroAssignmentBound(context.Background(), stale, assignment, click); !errors.Is(err, ErrStaleBinding) {
		t.Fatal(err)
	}
	p, err := svc.ApplyMacroAssignmentBound(context.Background(), binding, assignment, click)
	if err != nil || command.calls != 1 || p.Upload != MacroUploadConfirmed {
		t.Fatal(p, err)
	}
}

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

func TestTargetedServiceMakesSoleValidatedSeriallessX6Durable(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{serialless: true})
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
	if len(devices) != 1 || !devices[0].Eligible || devices[0].Warning != "" || devices[0].ID.Serial != "" || devices[0].ID.Key() != "1d57:fa60" {
		t.Fatalf("Refresh() = %#v; want sole validated durable serialless device", devices)
	}
	binding, ok := service.Selection()
	if !ok || binding.SessionOnly || binding.ID.Serial != "" || binding.ID.Key() != "1d57:fa60" {
		t.Fatalf("Selection() = %#v, %t; want durable serialless binding", binding, ok)
	}
	if second, ok := service.Selection(); !ok || second.ID != binding.ID {
		t.Fatalf("Selection() = %#v, %t; want deterministic session identity", second, ok)
	}
	output := logs.String()
	for _, want := range []string{"event=inventory_validation", "event=inventory_selection", "candidate_index=0", "vid_pid=1d57:fa60", "serial_present=false", "profile_match=true", "profile_validation=true", "eligibility=true", "warning=none", "selected_binding_present=true"} {
		if !strings.Contains(output, want) {
			t.Fatalf("diagnostic output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, candidate.Path) {
		t.Fatalf("diagnostic output contains candidate path:\n%s", output)
	}
}

func TestTargetedServiceSelectsSoleValidatedSeriallessCandidateFromMixedInventory(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{serialless: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name       string
		candidates []transport.Candidate
		validPaths map[string]bool
		warnings   []string
	}{
		{
			name: "profile mismatch and unsupported candidates remain ineligible",
			candidates: []transport.Candidate{
				{Path: "hidraw-valid", VendorID: 0x1D57, ProductID: 0xFA60},
				{Path: "hidraw-mismatch", VendorID: 0x1D57, ProductID: 0xFA60, Serial: "mismatch"},
				{Path: "hidraw-unsupported", VendorID: 0x9999, ProductID: 0x0001},
			},
			validPaths: map[string]bool{"hidraw-valid": true},
			warnings:   []string{"", "profile/interface mismatch", "unsupported profile"},
		},
		{
			name: "second serialless candidate with failed validation remains ineligible",
			candidates: []transport.Candidate{
				{Path: "hidraw-valid", VendorID: 0x1D57, ProductID: 0xFA60},
				{Path: "hidraw-invalid", VendorID: 0x1D57, ProductID: 0xFA60},
			},
			validPaths: map[string]bool{"hidraw-valid": true},
			warnings:   []string{"", "profile/interface mismatch"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTargetedService(registry, profileValidatedInventory{
				candidates:  tt.candidates,
				validByPath: tt.validPaths,
			}, &commandFake{})

			devices, err := service.Refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(devices) != len(tt.candidates) || !devices[0].Eligible {
				t.Fatalf("Refresh() = %#v; want the validated serialless X6 eligible", devices)
			}
			for index, device := range devices {
				if device.Eligible != (index == 0) || device.Warning != tt.warnings[index] {
					t.Fatalf("Refresh() = %#v; candidate %d eligible=%t warning=%q, want eligible=%t warning=%q", devices, index, device.Eligible, device.Warning, index == 0, tt.warnings[index])
				}
			}
			binding, ok := service.Selection()
			if !ok || binding.ID != devices[0].ID || binding.SessionOnly {
				t.Fatalf("Selection() = %#v, %t; want durable binding for the sole validated candidate", binding, ok)
			}
		})
	}
}

func TestTargetedServiceResolvesOnlyUniqueAuthorizedSeriallessIdentity(t *testing.T) {
	serialless := transport.Candidate{Path: "hidraw-serialless", VendorID: 0x1D57, ProductID: 0xFA60}

	for _, tt := range []struct {
		name          string
		profile       targetedProfile
		candidates    []transport.Candidate
		valid         bool
		wantErr       error
		wantEligible  bool
		wantSelection bool
	}{
		{name: "unique authorized candidate", profile: targetedProfile{serialless: true}, candidates: []transport.Candidate{serialless}, valid: true, wantEligible: true, wantSelection: true},
		{name: "profile does not authorize serialless identity", candidates: []transport.Candidate{serialless}, valid: true, wantEligible: false},
		{name: "invalid candidate", profile: targetedProfile{serialless: true}, candidates: []transport.Candidate{serialless}, valid: false, wantEligible: false},
		{name: "duplicate identity fails closed", profile: targetedProfile{serialless: true}, candidates: []transport.Candidate{serialless, {Path: "hidraw-serialless-2", VendorID: 0x1D57, ProductID: 0xFA60}}, valid: true, wantErr: ErrAmbiguousIdentity},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := NewProfileRegistry(tt.profile)
			if err != nil {
				t.Fatal(err)
			}
			service := NewTargetedService(registry, profileValidatedInventory{candidates: tt.candidates, valid: tt.valid}, &commandFake{})
			devices, err := service.Refresh(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Refresh() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if len(devices) != 1 || devices[0].Eligible != tt.wantEligible {
				t.Fatalf("Refresh() = %#v; want one device eligible=%t", devices, tt.wantEligible)
			}
			if _, ok := service.Selection(); ok != tt.wantSelection {
				t.Fatalf("Selection() present = %t, want %t", ok, tt.wantSelection)
			}
		})
	}
}

func TestTargetedServiceKeepsSeriallessIdentityDurableAcrossPathChanges(t *testing.T) {
	registry, err := NewProfileRegistry(targetedProfile{serialless: true})
	if err != nil {
		t.Fatal(err)
	}
	source := &mutableValidatedInventory{candidates: []transport.Candidate{{Path: "hidraw-1", VendorID: 0x1D57, ProductID: 0xFA60}}, valid: true}
	service := NewTargetedService(registry, source, &commandFake{})

	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, ok := service.Selection()
	if !ok || first.ID.Key() != "1d57:fa60" || first.SessionOnly {
		t.Fatalf("first Selection() = %#v, %t; want durable serialless binding", first, ok)
	}

	source.candidates[0].Path = "hidraw-2"
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, ok := service.Selection()
	if !ok || second.ID != first.ID || second.Path != "hidraw-2" || second.SessionOnly {
		t.Fatalf("second Selection() = %#v, %t; want same durable ID at new path", second, ok)
	}
}

func TestTargetedServiceSelectsSoleEligibleSerialBearingCandidateWithUnauthorizedSeriallessCandidate(t *testing.T) {
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
	binding, ok := service.Selection()
	if !ok || binding.ID.Serial != stable.Serial || binding.Path != stable.Path {
		t.Fatalf("Selection() = %#v, %t; want the sole eligible serial-bearing binding", binding, ok)
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

func TestApplyOperationBoundRejectsStaleBindingBeforeCommand(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	candidate := transport.Candidate{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}
	source := &mutableInventory{candidates: []transport.Candidate{candidate}}
	command := &commandFake{}
	service := NewTargetedService(registry, source, command)
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, ok := service.Selection()
	if !ok {
		t.Fatal("Selection() = no binding")
	}
	source.candidates[0].Path = "hidraw-2"

	if err := service.ApplyOperationBound(context.Background(), binding, operationFake{}, "polling"); !errors.Is(err, ErrStaleBinding) {
		t.Fatalf("ApplyOperationBound() error = %v, want ErrStaleBinding", err)
	}
	if command.calls != 0 {
		t.Fatalf("command calls = %d, want 0", command.calls)
	}
}

func TestApplyOperationBoundIgnoresUnrelatedACK(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	candidate := transport.Candidate{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}
	command := &ackCommandFake{reports: [][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}}, err: context.DeadlineExceeded}
	service := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{candidate}}, command)
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, _ := service.Selection()

	if err := service.ApplyOperationBound(context.Background(), binding, operationFake{}, "polling"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ApplyOperationBound() error = %v, want unrelated ACK to time out", err)
	}
	if command.keepReading[0] != true {
		t.Fatal("polling operation accepted a DPI acknowledgement")
	}
}

func TestApplyOperationBoundSerializesCommandsForTheSelectedDevice(t *testing.T) {
	registry, _ := NewProfileRegistry(targetedProfile{})
	candidate := transport.Candidate{Path: "hidraw-1", Serial: "A", VendorID: 0x1D57, ProductID: 0xFA60}
	command := &serialCommandFake{entered: make(chan struct{}), release: make(chan struct{})}
	service := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{candidate}}, command)
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, _ := service.Selection()

	results := make(chan error, 2)
	for range 2 {
		go func() {
			results <- service.ApplyOperationBound(context.Background(), binding, operationFake{}, "polling")
		}()
	}
	<-command.entered
	if calls, maximum := command.values(); calls != 1 || maximum != 1 {
		t.Fatalf("command calls/in flight = %d/%d, want 1/1 while first operation blocks", calls, maximum)
	}
	close(command.release)
	if err := <-results; err != nil {
		t.Fatalf("first ApplyOperationBound() error = %v", err)
	}
	if err := <-results; err != nil {
		t.Fatalf("second ApplyOperationBound() error = %v", err)
	}
	if calls, maximum := command.values(); calls != 2 || maximum != 1 {
		t.Fatalf("command calls/in flight = %d/%d, want 2/1", calls, maximum)
	}
}

type inventoryFake struct{ candidates []transport.Candidate }

func (f inventoryFake) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type profileValidatedInventory struct {
	candidates  []transport.Candidate
	valid       bool
	validByPath map[string]bool
}

func (f profileValidatedInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

func (f profileValidatedInventory) ProfileValid(_ context.Context, candidate transport.Candidate, _ HIDFacts) bool {
	if f.validByPath != nil {
		return f.validByPath[candidate.Path]
	}
	return f.valid
}

type mutableInventory struct{ candidates []transport.Candidate }

func (f *mutableInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

type mutableValidatedInventory struct {
	candidates []transport.Candidate
	valid      bool
}

func (f *mutableValidatedInventory) Enumerate(context.Context) ([]transport.Candidate, error) {
	return f.candidates, nil
}

func (f *mutableValidatedInventory) ProfileValid(context.Context, transport.Candidate, HIDFacts) bool {
	return f.valid
}

type commandFake struct {
	calls   int
	onApply func()
}

type targetedProfile struct{ serialless bool }

func (targetedProfile) ID() string                       { return "x6" }
func (targetedProfile) Match() transport.Match           { return transport.X6Match() }
func (p targetedProfile) AllowsSeriallessIdentity() bool { return p.serialless }
func (targetedProfile) HIDFacts() HIDFacts               { return HIDFacts{} }
func (targetedProfile) Codec() Codec                     { return targetedCodec{} }

type targetedCodec struct{}

func (targetedCodec) Validate(any) error              { return nil }
func (targetedCodec) Encode(any) ([]byte, error)      { return []byte{0x04}, nil }
func (targetedCodec) DecodeStatus([]byte) (any, bool) { return nil, false }
func (targetedCodec) MatchesACK([]byte) bool          { return true }
func (targetedCodec) Defaults() any                   { return nil }

type operationFake struct{}

func (operationFake) Validate(any) error         { return nil }
func (operationFake) Encode(any) ([]byte, error) { return []byte{0x06}, nil }
func (operationFake) MatchesACK(report []byte) bool {
	return len(report) == 5 && report[4] == 0x06
}

type ackCommandFake struct {
	reports     [][]byte
	err         error
	keepReading []bool
}

type serialCommandFake struct {
	entered, release chan struct{}
	mu               sync.Mutex
	calls, inFlight  int
	maximum          int
}

func (f *serialCommandFake) SendAndAwaitBound(context.Context, Binding, []byte, func([]byte) bool) error {
	f.mu.Lock()
	f.calls++
	f.inFlight++
	if f.inFlight > f.maximum {
		f.maximum = f.inFlight
	}
	calls := f.calls
	f.mu.Unlock()
	if calls == 1 {
		close(f.entered)
	}
	<-f.release
	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	return nil
}

func (f *serialCommandFake) values() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.maximum
}

func (f *ackCommandFake) SendAndAwaitBound(_ context.Context, _ Binding, _ []byte, keepReading func([]byte) bool) error {
	for _, report := range f.reports {
		f.keepReading = append(f.keepReading, keepReading(report))
	}
	return f.err
}

func (f *commandFake) SendAndAwaitBound(context.Context, Binding, []byte, func([]byte) bool) error {
	f.calls++
	if f.onApply != nil {
		f.onApply()
	}
	return nil
}
