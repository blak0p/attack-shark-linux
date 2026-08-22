package mouse

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
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
