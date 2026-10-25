package desktop

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type profileWorkspace struct {
	DPI      Snapshot
	Polling  PollingSnapshot
	Sleep    NormalSleepSnapshot
	Debounce DebounceSnapshot
	Lighting LightingSnapshot
	Remap    RemapSnapshot
}

func storedProfile(t *testing.T, store *configstore.ProfileStore, b Binding, id string) configstore.LocalProfile {
	t.Helper()
	p, err := store.Read(b.ID, b.ProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func workspace(s *Service) profileWorkspace {
	return profileWorkspace{s.GetSnapshot(), s.GetPollingSnapshot(), s.GetNormalSleepSnapshot(), s.GetDebounceSnapshot(), s.GetLightingSnapshot(), s.GetRemapSnapshot()}
}

func TestProfilesCompletionNoStorePollingFailureCompatibility(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, _, b := switchTestService(t, t.TempDir(), cmd)
	s.profiles = nil
	config := x6.DefaultDeviceConfig()
	loads, saves := 0, 0
	s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { loads++; return config, nil }, func(Binding, x6.DeviceConfig) error { saves++; return errors.New("disk") })
	s.SelectDevice(b.ID)
	expected := s.StagePollingRate(x6.PollingRate500)
	expected.Applied, expected.Firmware, expected.Persistence, expected.RetryAvailable = x6.PollingRate500, "success", "failed", true
	calls, readCount := cmd.calls, loads
	got := s.ApplyPollingRate(context.Background())
	if !reflect.DeepEqual(got, expected) || cmd.calls-calls != 1 || saves != 1 || loads-readCount != 1 || !reflect.DeepEqual(config, x6.DefaultDeviceConfig()) {
		t.Fatalf("legacy compatibility: got=%+v want=%+v writes=%d saves=%d reads=%d", got, expected, cmd.calls-calls, saves, loads-readCount)
	}
}

func TestProfilesCompletionPersistenceFailure(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	source := s.GetProfiles().ActiveID
	target := s.CreateProfile("target").ActiveID
	d := s.GetSnapshot().Pending
	d.DPI[0] = 3200
	s.StageDPI(d)
	if r := s.ApplyDPI(context.Background()); r.Error.Code != "" {
		t.Fatal(r)
	}
	s.StagePollingRate(x6.PollingRate500)
	if r := s.ApplyPollingRate(context.Background()); r.Error.Code != "" {
		t.Fatal(r)
	}
	s.StageNormalSleep(7)
	if r := s.ApplyNormalSleep(); r.Error.Code != "" {
		t.Fatal(r)
	}
	s.StageDebounce(12)
	if r := s.ApplyDebounce(); r.Error.Code != "" {
		t.Fatal(r)
	}
	light := x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonTwo}
	s.StageLighting(light)
	if r := s.ApplyLighting(); r.Error.Code != "" {
		t.Fatal(r)
	}
	rem := x6.DefaultRemapConfig()
	if r := s.ApplyRemap(rem); r.Error.Code != "" {
		t.Fatal(r)
	}
	wantDPI, wantPol := s.GetSnapshot().Applied, s.GetPollingSnapshot().Applied
	wantSleep, wantDeb := s.GetNormalSleepSnapshot().Applied, s.GetDebounceSnapshot().Applied
	if _, err := store.Switch(b.ID, b.ProfileID, source); err != nil {
		t.Fatal(err)
	}
	if r := s.SwitchProfile(source); !r.Applied {
		t.Fatal(r)
	}
	beforeSource := storedProfile(t, store, b, source)
	beforeTarget := storedProfile(t, store, b, target)
	expected := workspace(s)
	expected.DPI.Applied, expected.DPI.Pending = wantDPI, wantDPI
	expected.DPI.Revision++
	expected.DPI.Firmware, expected.DPI.Persistence, expected.DPI.Error = "success", "failed", Error{Code: PersistenceFailed}
	expected.Polling.Applied, expected.Polling.Desired = wantPol, wantPol
	expected.Polling.Revision++
	expected.Polling.Firmware, expected.Polling.Persistence, expected.Polling.Error = "success", "failed", Error{Code: PersistenceFailed}
	expected.Sleep.Applied, expected.Sleep.Pending = wantSleep, wantSleep
	expected.Sleep.Firmware, expected.Sleep.Persistence, expected.Sleep.Error = "success", "failed", Error{Code: PersistenceFailed}
	expected.Debounce.Applied, expected.Debounce.Desired = wantDeb, wantDeb
	expected.Debounce.Firmware, expected.Debounce.Persistence, expected.Debounce.Error = "success", "failed", Error{Code: PersistenceFailed}
	expected.Lighting.Pending, expected.Lighting.Applied = light, &light
	expected.Lighting.Firmware, expected.Lighting.Error = "success", Error{Code: PersistenceFailed}
	expected.Remap.Applied, expected.Remap.Pending = rem, rem
	expected.Remap.Firmware, expected.Remap.Persistence, expected.Remap.Error = "success", "failed", Error{Code: PersistenceFailed}
	saves := 0
	s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error { saves++; return errors.New("disk full") })
	calls := cmd.calls
	r := s.SwitchProfile(target)
	if r.Applied || r.Error.Code != PersistenceFailed || r.ActiveID != source || cmd.calls-calls != 4 || saves != 1 {
		t.Fatalf("result=%+v writes=%d saves=%d", r, cmd.calls-calls, saves)
	}
	dpi, pol, sleep, deb, lighting, remap := s.GetSnapshot(), s.GetPollingSnapshot(), s.GetNormalSleepSnapshot(), s.GetDebounceSnapshot(), s.GetLightingSnapshot(), s.GetRemapSnapshot()
	if dpi.Persistence != "failed" || pol.Persistence != "failed" || sleep.Persistence != "failed" || deb.Persistence != "failed" || remap.Persistence != "failed" || dpi.Error.Code != PersistenceFailed || pol.Error.Code != PersistenceFailed || sleep.Error.Code != PersistenceFailed || deb.Error.Code != PersistenceFailed || remap.Error.Code != PersistenceFailed || lighting.Error.Code != PersistenceFailed {
		t.Errorf("optimistic persistence: dpi=%+v polling=%+v sleep=%+v debounce=%+v lighting=%+v remap=%+v", dpi, pol, sleep, deb, lighting, remap)
	}
	if got := workspace(s); !reflect.DeepEqual(got, expected) {
		t.Errorf("full failure workspace: got=%+v want=%+v", got, expected)
	}
	if dpi.Applied != wantDPI || pol.Applied != wantPol || sleep.Applied != wantSleep || deb.Applied != wantDeb || lighting.Applied == nil || *lighting.Applied != light || !reflect.DeepEqual(remap.Applied, rem) {
		t.Error("acknowledged progress lost")
	}
	list, _ := store.List(b.ID, b.ProfileID)
	afterSource := storedProfile(t, store, b, source)
	afterTarget := storedProfile(t, store, b, target)
	if list.ActiveID != source || !reflect.DeepEqual(beforeSource, afterSource) || !reflect.DeepEqual(beforeTarget, afterTarget) {
		t.Fatal("failed completion changed stored profiles")
	}
}

// A legacy saver is deliberately reentrant: final activation must reject its
// now-stale captured binding without touching the replacement workspace.
func TestProfilesCompletionSelectionDuringSave(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "failed"}[fail], func(t *testing.T) {
			cmd := &recordingSwitchCommand{}
			s, store, b := switchTestService(t, t.TempDir(), cmd)
			source := s.GetProfiles().ActiveID
			target := s.CreateProfile("target").ActiveID
			d := s.GetSnapshot().Pending
			d.DPI[0] = 3200
			s.StageDPI(d)
			s.ApplyDPI(context.Background())
			if _, err := store.Switch(b.ID, b.ProfileID, source); err != nil {
				t.Fatal(err)
			}
			inv := s.RefreshInventory(context.Background())
			otherBinding := s.SelectDevice(inv.Devices[1].ID).Selected
			otherProfiles := s.GetProfiles()
			other := workspace(s)
			otherPayload := storedProfile(t, store, *otherBinding, otherProfiles.ActiveID)
			s.SelectDevice(b.ID)
			beforeSource := storedProfile(t, store, b, source)
			beforeTarget := storedProfile(t, store, b, target)
			saves := 0
			s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error {
				saves++
				if r := s.SelectDevice(otherBinding.ID); r.Error.Code != "" {
					t.Fatal(r)
				}
				if fail {
					return errors.New("disk full")
				}
				return nil
			})
			calls := cmd.calls
			r := s.SwitchProfile(target)
			if r.Applied || r.Error.Code != StaleBinding || r.ActiveID != source || cmd.calls-calls != 4 || saves != 1 {
				t.Fatalf("result=%+v writes=%d saves=%d", r, cmd.calls-calls, saves)
			}
			if got := workspace(s); !reflect.DeepEqual(got, other) {
				t.Fatalf("replacement workspace changed: before=%+v after=%+v", other, got)
			}
			afterOther := storedProfile(t, store, *otherBinding, otherProfiles.ActiveID)
			list, _ := store.List(b.ID, b.ProfileID)
			afterSource := storedProfile(t, store, b, source)
			afterTarget := storedProfile(t, store, b, target)
			if list.ActiveID != source || !reflect.DeepEqual(beforeSource, afterSource) || !reflect.DeepEqual(beforeTarget, afterTarget) || !reflect.DeepEqual(otherPayload, afterOther) {
				t.Fatal("stale completion changed stored data")
			}
			s.SelectDevice(b.ID)
			if s.GetSnapshot().Applied != d {
				t.Fatal("reselection discarded actual partial hardware progress")
			}
		})
	}
}

func TestProfilesCompletionMacroFinalACK(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	source := s.GetProfiles().ActiveID
	target := s.CreateProfile("macro target").ActiveID
	m, err := s.CreateMacro("macro", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	if r := s.StageMacroAssignment(m.ID, 6, 3); r.Error.Code != "" {
		t.Fatal(r)
	}
	if r := s.ApplyRemap(x6.DefaultRemapConfig()); r.Error.Code != "" {
		t.Fatal(r)
	}
	if r := s.SwitchProfile(source); !r.Applied {
		t.Fatal(r)
	}
	inv := s.RefreshInventory(context.Background())
	otherBinding := s.SelectDevice(inv.Devices[1].ID).Selected
	otherProfiles := s.GetProfiles()
	other := workspace(s)
	otherPayload := storedProfile(t, store, *otherBinding, otherProfiles.ActiveID)
	s.SelectDevice(b.ID)
	beforeSource, beforeTarget := storedProfile(t, store, b, source), storedProfile(t, store, b, target)
	calls := cmd.calls
	cmd.during = func() {
		if cmd.calls == calls+4 {
			s.SelectDevice(otherBinding.ID)
		}
	}
	r := s.SwitchProfile(target)
	if r.Applied || r.Error.Code != StaleBinding || r.ActiveID != source || cmd.calls-calls != 4 {
		t.Fatalf("macro final ACK result=%+v writes=%d", r, cmd.calls-calls)
	}
	if got := workspace(s); !reflect.DeepEqual(got, other) {
		t.Fatalf("macro ACK changed replacement: before=%+v after=%+v", other, got)
	}
	list, err := store.List(b.ID, b.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	if list.ActiveID != source || !reflect.DeepEqual(beforeSource, storedProfile(t, store, b, source)) || !reflect.DeepEqual(beforeTarget, storedProfile(t, store, b, target)) || !reflect.DeepEqual(otherPayload, storedProfile(t, store, *otherBinding, otherProfiles.ActiveID)) {
		t.Fatal("stale macro ACK changed stored payloads")
	}
	s.SelectDevice(b.ID)
	if progress := s.GetRemapSnapshot().MacroProgress; progress.Upload != mouse.MacroUploadConfirmed || progress.Assignment != mouse.MacroAssignmentACKConfirmed {
		t.Fatalf("actual macro transport progress lost: %+v", progress)
	}
}

func TestProfilesCompletionImplicitSelectionRestore(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, root, cmd)
	active := s.GetProfiles().ActiveID
	d := s.GetSnapshot().Pending
	d.DPI[0] = 2800
	s.StageDPI(d)
	s.ApplyDPI(context.Background())
	s.StagePollingRate(x6.PollingRate500)
	s.ApplyPollingRate(context.Background())
	s.StageNormalSleep(7)
	s.ApplyNormalSleep()
	s.StageDebounce(12)
	s.ApplyDebounce()
	light := x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonTwo}
	s.StageLighting(light)
	s.ApplyLighting()
	m, err := s.CreateMacro("macro", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	if r := s.StageMacroAssignment(m.ID, 6, 3); r.Error.Code != "" {
		t.Fatal(r)
	}
	if r := s.ApplyRemap(x6.DefaultRemapConfig()); r.Error.Code != "" {
		t.Fatal(r)
	}
	before, _ := store.Read(b.ID, b.ProfileID, active)
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	restoredCmd := &recordingSwitchCommand{}
	restored := Compose(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}, WithMacroLibrary(s.macroLibrary, nil)).AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{{VendorID: b.ID.VendorID, ProductID: b.ID.ProductID, Serial: b.ID.Serial, Path: b.Path}}}, restoredCmd)).AttachProfiles(configstore.NewProfileStore(root), nil)
	expected := workspace(restored)
	expected.DPI.Applied, expected.DPI.Pending = d, d
	expected.Polling.Applied, expected.Polling.Desired = x6.PollingRate500, x6.PollingRate500
	savedRate := x6.PollingRate500
	expected.Polling.Persisted, expected.Polling.Persistence = &savedRate, "success"
	savedSleep, savedDebounce := float64(7), 12
	expected.Sleep.Applied, expected.Sleep.Pending, expected.Sleep.Persisted, expected.Sleep.Persistence = 7, 7, &savedSleep, "success"
	expected.Debounce.Applied, expected.Debounce.Desired, expected.Debounce.Persisted, expected.Debounce.Persistence = 12, 12, &savedDebounce, "success"
	expected.Lighting.Pending = light
	savedRemap := s.GetRemapSnapshot()
	expected.Remap.Pending, expected.Remap.Applied = savedRemap.Pending, savedRemap.Applied
	expected.Remap.MacroPending, expected.Remap.MacroApplied = savedRemap.MacroPending, savedRemap.MacroApplied
	expected.Remap.MacroDrafts, expected.Remap.MacroAppliedDrafts = savedRemap.MacroDrafts, savedRemap.MacroAppliedDrafts
	inv := restored.RefreshInventory(context.Background())
	if inv.Error.Code != "" || inv.Selected == nil {
		t.Fatal(inv)
	}
	if restored.GetSnapshot().Applied != d || restored.GetPollingSnapshot().Applied != x6.PollingRate500 || restored.GetNormalSleepSnapshot().Applied != 7 || restored.GetDebounceSnapshot().Applied != 12 || restored.GetLightingSnapshot().Pending != light || !reflect.DeepEqual(restored.GetRemapSnapshot().MacroDrafts, s.GetRemapSnapshot().MacroDrafts) {
		t.Fatalf("implicit selection did not restore all lanes: %+v", workspace(restored))
	}
	if got := workspace(restored); !reflect.DeepEqual(got, expected) {
		t.Errorf("full restored workspace: got=%+v want=%+v", got, expected)
	}
	pol := restored.GetPollingSnapshot()
	if pol.Persisted == nil || *pol.Persisted != x6.PollingRate500 {
		t.Errorf("saved polling persistence not restored: %+v", pol)
	}
	after, _ := store.Read(b.ID, b.ProfileID, active)
	if restoredCmd.calls != 0 || !reflect.DeepEqual(before, after) {
		t.Fatal("bootstrap restoration changed hardware or saved payload")
	}
}

func TestProfilesCompletionSelectionRestoration(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	active := s.GetProfiles().ActiveID
	d := s.GetSnapshot().Pending
	d.DPI[0] = 2800
	s.StageDPI(d)
	s.ApplyDPI(context.Background())
	s.StagePollingRate(x6.PollingRate500)
	s.ApplyPollingRate(context.Background())
	s.StageNormalSleep(7)
	s.ApplyNormalSleep()
	s.StageDebounce(12)
	s.ApplyDebounce()
	light := x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonTwo}
	s.StageLighting(light)
	s.ApplyLighting()
	rem := x6.DefaultRemapConfig()
	s.ApplyRemap(rem)
	before, _ := store.Read(b.ID, b.ProfileID, active)
	// Legacy records intentionally lag the complete profile payload.
	s.AttachDevicePersistence(func(Binding) (x6.DPIConfig, error) { return x6.DefaultDPIConfig(), nil }, func(Binding, x6.DPIConfig) error { return nil })
	s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error { return nil })
	s.AttachNormalSleepPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error { return nil })
	inv := s.RefreshInventory(context.Background())
	s.SelectDevice(inv.Devices[1].ID)
	other := s.GetSnapshot()
	calls := cmd.calls
	selected := s.SelectDevice(b.ID)
	if selected.Error.Code != "" {
		t.Fatal(selected)
	}
	// Deliberately do not call GetProfiles: selection itself must restore.
	if s.GetSnapshot().Applied != d || s.GetPollingSnapshot().Applied != x6.PollingRate500 || s.GetNormalSleepSnapshot().Applied != 7 || s.GetDebounceSnapshot().Applied != 12 || s.GetLightingSnapshot().Pending != light || !reflect.DeepEqual(s.GetRemapSnapshot().Applied, rem) {
		t.Errorf("selection discarded saved payload: %+v %+v %+v %+v", s.GetSnapshot(), s.GetPollingSnapshot(), s.GetNormalSleepSnapshot(), s.GetDebounceSnapshot())
	}
	after, _ := store.Read(b.ID, b.ProfileID, active)
	if cmd.calls != calls || !reflect.DeepEqual(before, after) {
		t.Fatal("restoration wrote hardware or changed payload")
	}
	s.SelectDevice(inv.Devices[1].ID)
	if got := s.GetSnapshot(); !reflect.DeepEqual(got, other) {
		t.Fatalf("other device changed: before=%+v after=%+v", other, got)
	}
}
