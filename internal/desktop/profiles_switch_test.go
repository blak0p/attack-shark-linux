package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type recordingSwitchCommand struct {
	calls       int
	failAtCall  int
	failWithErr error
	reports     [][]byte
	during      func()
}

func (c *recordingSwitchCommand) SendAndAwaitBound(_ context.Context, _ mouse.Binding, report []byte, continueReading func([]byte) bool) error {
	c.calls++
	c.reports = append(c.reports, append([]byte(nil), report...))
	if c.failAtCall != 0 && c.calls == c.failAtCall {
		if c.failWithErr != nil {
			return c.failWithErr
		}
		return errors.New("command ack failed")
	}
	if c.during != nil {
		c.during()
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0x00, report[0]})
	return nil
}

func (c *recordingSwitchCommand) SendX6MacroSequenceAssignmentBound(_ context.Context, _ mouse.Binding, _ protocol.MacroAssignment, _ macros.X6Sequence) (mouse.MacroProgress, error) {
	c.calls++
	if c.failAtCall != 0 && c.calls == c.failAtCall {
		if c.failWithErr != nil {
			return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, c.failWithErr
		}
		return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, errors.New("macro ack failed")
	}
	if c.during != nil {
		c.during()
	}
	return mouse.MacroProgress{Upload: mouse.MacroUploadConfirmed, Assignment: mouse.MacroAssignmentACKConfirmed}, nil
}

func (c *recordingSwitchCommand) SendX6MultiMacroSequenceAssignmentBound(_ context.Context, _ mouse.Binding, _ protocol.RemapConfig, _ []mouse.MacroSequenceItem) (mouse.MacroProgress, error) {
	c.calls++
	if c.failAtCall != 0 && c.calls == c.failAtCall {
		if c.failWithErr != nil {
			return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, c.failWithErr
		}
		return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, errors.New("macro ack failed")
	}
	if c.during != nil {
		c.during()
	}
	return mouse.MacroProgress{Upload: mouse.MacroUploadConfirmed, Assignment: mouse.MacroAssignmentACKConfirmed}, nil
}

func (c *recordingSwitchCommand) SendX6MultiMacroLocalAssignmentBound(_ context.Context, _ mouse.Binding, _ protocol.RemapConfig, _ []mouse.MacroLocalItem) (mouse.MacroProgress, error) {
	c.calls++
	if c.failAtCall != 0 && c.calls == c.failAtCall {
		if c.failWithErr != nil {
			return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, c.failWithErr
		}
		return mouse.MacroProgress{Upload: mouse.MacroUploadNotStarted, Assignment: mouse.MacroAssignmentNotStarted}, errors.New("macro ack failed")
	}
	if c.during != nil {
		c.during()
	}
	return mouse.MacroProgress{Upload: mouse.MacroUploadConfirmed, Assignment: mouse.MacroAssignmentACKConfirmed}, nil
}

func switchTestService(t *testing.T, root string, command *recordingSwitchCommand) (*Service, *configstore.ProfileStore, mouse.Binding) {
	t.Helper()
	store := configstore.NewProfileStore(root)
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidates := []transport.Candidate{
		{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "switch-device", Path: "/dev/fake-sw"},
		{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "device-two", Path: "/dev/fake-sw2"},
	}
	lib, err := macros.Open(filepath.Join(t.TempDir(), "macros.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := Compose(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}, WithMacroLibrary(lib, nil)).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: candidates}, command)).
		AttachProfiles(store, func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil })

	inventory := s.RefreshInventory(context.Background())
	selected := s.SelectDevice(inventory.Devices[0].ID)
	if selected.Selected == nil {
		t.Fatal("device selection failed")
	}
	return s, store, *selected.Selected
}

func TestProfilesSwitchSuccessAndSnapshotUpdates(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, binding := switchTestService(t, root, cmd)

	initSnap := s.GetProfiles()
	if initSnap.Error.Code != "" || len(initSnap.Profiles) != 1 {
		t.Fatalf("bootstrap profiles: %+v", initSnap)
	}
	firstID := initSnap.ActiveID

	// Create a second profile "Gaming"
	created := s.CreateProfile("Gaming")
	if created.Error.Code != "" || created.ActiveID == firstID {
		t.Fatalf("create profile: %+v", created)
	}
	gamingID := created.ActiveID

	// Modify Gaming's payload directly in the store to have distinct settings for each lane
	gamingConfig := profileConfiguration{
		Version: 1,
		Source:  "local settings",
		Device: x6.DeviceConfig{
			DPIConfig: func() x6.DPIConfig {
				d := x6.DefaultDPIConfig()
				d.DPI[0] = 3200
				return d
			}(),
			PollingRate:        x6.PollingRate500,
			NormalSleepMinutes: 10.0,
			ResponseTimeMs:     16,
			Remap: func() *x6.RemapConfig {
				r := x6.DefaultRemapConfig()
				if len(r.Buttons) > 1 {
					r.Buttons[1].Action = protocol.RemapLeft
				}
				return &r
			}(),
		},
		Lighting: &x6.LightingSelection{
			Mode:       x6.LightingBreathing,
			TemplateID: x6.LightingTemplateBreathingGreen,
		},
	}
	raw, err := json.Marshal(gamingConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveActive(binding.ID, binding.ProfileID, raw); err != nil {
		t.Fatal(err)
	}

	// Switch back to firstID first (to ensure we switch TO gamingID)
	if _, err := store.Switch(binding.ID, binding.ProfileID, firstID); err != nil {
		t.Fatal(err)
	}

	cmd.calls = 0
	cmd.reports = nil

	// SwitchProfile to Gaming
	res := s.SwitchProfile(gamingID)
	if !res.Applied || res.ActiveID != gamingID || res.Error.Code != "" {
		t.Fatalf("SwitchProfile failed: %+v", res)
	}
	if cmd.calls == 0 {
		t.Fatal("SwitchProfile made no hardware writes")
	}

	// Verify all in-memory snapshots reflect Gaming settings
	if dpi := s.GetSnapshot(); dpi.Applied.DPI[0] != 3200 {
		t.Fatalf("DPI not updated: got %d, want 3200", dpi.Applied.DPI[0])
	}
	if pol := s.GetPollingSnapshot(); pol.Applied != x6.PollingRate500 {
		t.Fatalf("Polling not updated: got %d, want 500", pol.Applied)
	}
	if sleep := s.GetNormalSleepSnapshot(); sleep.Applied != 10.0 {
		t.Fatalf("Sleep not updated: got %f, want 10.0", sleep.Applied)
	}
	if deb := s.GetDebounceSnapshot(); deb.Applied != 16 {
		t.Fatalf("Debounce not updated: got %d, want 16", deb.Applied)
	}
	if light := s.GetLightingSnapshot(); light.Applied == nil || light.Applied.Mode != x6.LightingBreathing {
		t.Fatalf("Lighting not updated: got %+v", light.Applied)
	}
	if rem := s.GetRemapSnapshot(); len(rem.Applied.Buttons) < 2 || rem.Applied.Buttons[1].Action != protocol.RemapLeft {
		t.Fatalf("Remap not updated: got %+v", rem.Applied)
	}

	// Verify active ID in store is committed
	list, err := store.List(binding.ID, binding.ProfileID)
	if err != nil || list.ActiveID != gamingID {
		t.Fatalf("store active ID: %s, want %s, err: %v", list.ActiveID, gamingID, err)
	}
}

func TestProfilesSwitchPartialFailurePreservesActiveIDAndPayloads(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{failAtCall: 2} // Let first lane (DPI) succeed, but fail on second (Polling)
	s, store, binding := switchTestService(t, root, cmd)

	initSnap := s.GetProfiles()
	firstID := initSnap.ActiveID

	created := s.CreateProfile("FailingTarget")
	targetID := created.ActiveID

	targetConfig := profileConfiguration{
		Version: 1,
		Source:  "local settings",
		Device: x6.DeviceConfig{
			DPIConfig: func() x6.DPIConfig {
				d := x6.DefaultDPIConfig()
				d.DPI[0] = 4800
				return d
			}(),
			PollingRate:        x6.PollingRate250,
			NormalSleepMinutes: 15.0,
			ResponseTimeMs:     20,
		},
	}
	raw, err := json.Marshal(targetConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveActive(binding.ID, binding.ProfileID, raw); err != nil {
		t.Fatal(err)
	}

	// Read both payloads before switch
	sourceSavedBefore, err := store.Read(binding.ID, binding.ProfileID, firstID)
	if err != nil {
		t.Fatal(err)
	}
	targetSavedBefore, err := store.Read(binding.ID, binding.ProfileID, targetID)
	if err != nil {
		t.Fatal(err)
	}

	// Switch back to firstID before test
	if _, err := store.Switch(binding.ID, binding.ProfileID, firstID); err != nil {
		t.Fatal(err)
	}

	cmd.calls = 0
	res := s.SwitchProfile(targetID)

	// Switch must fail, Applied must be false, ActiveID must remain firstID
	if res.Applied {
		t.Fatalf("expected Applied: false, got true")
	}
	if res.ActiveID != firstID {
		t.Fatalf("active ID moved to %s, want previous %s", res.ActiveID, firstID)
	}
	if res.Error.Code == "" {
		t.Fatal("expected error code on switch failure")
	}

	// Check that store still has firstID as active
	list, err := store.List(binding.ID, binding.ProfileID)
	if err != nil || list.ActiveID != firstID {
		t.Fatalf("store active ID was overwritten: %s", list.ActiveID)
	}

	// Verify both stored JSON payloads are identical to before (not overwritten by partial state)
	sourceSavedAfter, _ := store.Read(binding.ID, binding.ProfileID, firstID)
	targetSavedAfter, _ := store.Read(binding.ID, binding.ProfileID, targetID)
	if string(sourceSavedAfter.Configuration) != string(sourceSavedBefore.Configuration) {
		t.Fatal("source profile payload was corrupted/overwritten by partial switch")
	}
	if string(targetSavedAfter.Configuration) != string(targetSavedBefore.Configuration) {
		t.Fatal("target profile payload was corrupted/overwritten by partial switch")
	}

	// Snapshots must show only actually acknowledged progress:
	// Lane 1 (DPI) succeeded, so DPI snapshot reflects the new DPI
	if dpi := s.GetSnapshot(); dpi.Applied.DPI[0] != 4800 {
		t.Fatalf("DPI should have updated to 4800, got %d", dpi.Applied.DPI[0])
	}
	// Lane 2 (Polling) failed, so polling firmware is "failed" and was not applied
	if pol := s.GetPollingSnapshot(); pol.Firmware != "failed" || pol.Applied == x6.PollingRate250 {
		t.Fatalf("Polling snapshot should show failure and unapplied, got %+v", pol)
	}
	// Lane 3 (Sleep) was not reached, so sleep snapshot remains at previous
	if sleep := s.GetNormalSleepSnapshot(); sleep.Applied == 15.0 {
		t.Fatalf("Sleep should not have been applied, got %f", sleep.Applied)
	}
}

func TestProfilesSwitchInvalidTargetPreflightSendsZeroWrites(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, binding := switchTestService(t, root, cmd)

	initSnap := s.GetProfiles()
	firstID := initSnap.ActiveID

	created := s.CreateProfile("InvalidDPI")
	targetID := created.ActiveID

	// Write invalid configuration (e.g. invalid DPI or sleep)
	invalidConfig := profileConfiguration{
		Version: 1,
		Source:  "local settings",
		Device: x6.DeviceConfig{
			DPIConfig: func() x6.DPIConfig {
				d := x6.DefaultDPIConfig()
				d.DPI[0] = -500 // invalid DPI
				return d
			}(),
		},
	}
	raw, _ := json.Marshal(invalidConfig)
	// Write directly via file to bypass store validation
	dir := filepath.Join(root, filepath.Base(filepath.Dir(root)))
	_ = dir
	_ = store.SaveActive(binding.ID, binding.ProfileID, raw)

	// Switch back to firstID
	store.Switch(binding.ID, binding.ProfileID, firstID)

	cmd.calls = 0
	res := s.SwitchProfile(targetID)
	if res.Applied || res.ActiveID != firstID || res.Error.Code != InvalidProfile {
		t.Fatalf("expected InvalidProfile without apply: %+v", res)
	}
	if cmd.calls != 0 {
		t.Fatalf("preflight failed but made %d device writes; want 0", cmd.calls)
	}
}

func TestProfilesSwitchCancelsPendingAutosavesAndIgnoresLateCallbacks(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, binding := switchTestService(t, root, cmd)

	scheduler := &lateScheduler{}
	s.attachAutomaticSave(scheduler)
	s.attachPollingAutomaticSave(scheduler)

	initSnap := s.GetProfiles()
	firstID := initSnap.ActiveID

	created := s.CreateProfile("Target")
	targetID := created.ActiveID

	// Switch back to firstID
	store.Switch(binding.ID, binding.ProfileID, firstID)

	// Stage an autosave on first profile
	dpi := s.GetSnapshot().Pending
	dpi.DPI[0] = 1600
	stagedDPI := s.StageDPI(dpi)
	if err := s.dpiComponent.sync.ScheduleAt(binding, stagedDPI.Revision, fromDTO(dpi)); err != nil {
		t.Fatal(err)
	}

	pol := s.StagePollingRate(x6.PollingRate250)
	if err := s.pollingComponent.sync.ScheduleAt(binding, pol.Revision, x6.PollingRate250); err != nil {
		t.Fatal(err)
	}

	cmd.calls = 0

	// Now switch profile to Target
	res := s.SwitchProfile(targetID)
	if !res.Applied || res.ActiveID != targetID {
		t.Fatalf("SwitchProfile failed: %+v", res)
	}
	switchCalls := cmd.calls

	// Fire all scheduled late callbacks
	scheduler.FireAll()

	// Verify that late callbacks made NO additional hardware writes
	if cmd.calls != switchCalls {
		t.Fatalf("late callbacks made %d additional writes; want 0", cmd.calls-switchCalls)
	}

	// Verify target profile still has its settings, not the staged 1600 DPI
	currentDPI := s.GetSnapshot().Applied.DPI[0]
	if currentDPI == 1600 {
		t.Fatal("late callback overwritten target profile DPI with 1600")
	}
}

func TestProfilesOrdinaryEditsSaveActiveProfileAndRetry(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, binding := switchTestService(t, root, cmd)

	initSnap := s.GetProfiles()
	activeID := initSnap.ActiveID

	// 1. DPI apply updates active profile JSON
	dpi := s.GetSnapshot().Pending
	dpi.DPI[0] = 2800
	s.StageDPI(dpi)
	appliedDPI := s.ApplyDPI(context.Background())
	if appliedDPI.Error.Code != "" || appliedDPI.Applied.DPI[0] != 2800 {
		t.Fatalf("ApplyDPI failed: %+v", appliedDPI)
	}

	saved, err := store.Read(binding.ID, binding.ProfileID, activeID)
	if err != nil {
		t.Fatal(err)
	}
	var p profileConfiguration
	if err := json.Unmarshal(saved.Configuration, &p); err != nil || p.Device.DPI[0] != 2800 {
		t.Fatalf("active profile DPI not saved: %+v %v", p, err)
	}

	// 2. Polling apply updates active profile JSON
	s.StagePollingRate(x6.PollingRate500)
	appliedPol := s.ApplyPollingRate(context.Background())
	if appliedPol.Error.Code != "" || appliedPol.Applied != x6.PollingRate500 {
		t.Fatalf("ApplyPollingRate failed: %+v", appliedPol)
	}

	saved, _ = store.Read(binding.ID, binding.ProfileID, activeID)
	json.Unmarshal(saved.Configuration, &p)
	if p.Device.PollingRate != x6.PollingRate500 {
		t.Fatalf("active profile Polling not saved: %d", p.Device.PollingRate)
	}

	// 3. Sleep and Debounce apply update active profile JSON
	s.StageNormalSleep(7.0)
	appliedSleep := s.ApplyNormalSleep()
	if appliedSleep.Error.Code != "" || appliedSleep.Applied != 7.0 {
		t.Fatalf("ApplyNormalSleep failed: %+v", appliedSleep)
	}

	s.StageDebounce(12)
	appliedDeb := s.ApplyDebounce()
	if appliedDeb.Error.Code != "" || appliedDeb.Applied != 12 {
		t.Fatalf("ApplyDebounce failed: %+v", appliedDeb)
	}

	saved, _ = store.Read(binding.ID, binding.ProfileID, activeID)
	json.Unmarshal(saved.Configuration, &p)
	if p.Device.NormalSleepMinutes != 7.0 || p.Device.ResponseTimeMs != 12 {
		t.Fatalf("active profile Sleep/Debounce not saved: %f %d", p.Device.NormalSleepMinutes, p.Device.ResponseTimeMs)
	}

	// 4. Lighting apply updates active profile JSON
	lightSel := x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonTwo}
	s.StageLighting(lightSel)
	appliedLight := s.ApplyLighting()
	if appliedLight.Error.Code != "" || appliedLight.Applied == nil || appliedLight.Applied.Mode != x6.LightingNeon {
		t.Fatalf("ApplyLighting failed: %+v", appliedLight)
	}

	saved, _ = store.Read(binding.ID, binding.ProfileID, activeID)
	json.Unmarshal(saved.Configuration, &p)
	if p.Lighting == nil || p.Lighting.Mode != x6.LightingNeon {
		t.Fatalf("active profile Lighting not saved: %+v", p.Lighting)
	}

	// 5. Remap apply updates active profile JSON
	remap := x6.DefaultRemapConfig()
	remap.Buttons[0].Action = protocol.RemapRight
	appliedRemap := s.ApplyRemap(remap)
	if appliedRemap.Error.Code != "" || appliedRemap.Applied.Buttons[0].Action != protocol.RemapRight {
		t.Fatalf("ApplyRemap failed: %+v", appliedRemap)
	}

	saved, _ = store.Read(binding.ID, binding.ProfileID, activeID)
	json.Unmarshal(saved.Configuration, &p)
	if p.Device.Remap == nil || p.Device.Remap.Buttons[0].Action != protocol.RemapRight {
		t.Fatalf("active profile Remap not saved: %+v", p.Device.Remap)
	}
}

func TestProfilesMacroAssignmentsSaveAndRoundtrip(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, store, binding := switchTestService(t, root, cmd)

	initSnap := s.GetProfiles()
	activeID := initSnap.ActiveID

	// Create a macro in the shared library
	events := []macros.Event{
		{Type: macros.MouseLeft, Action: macros.Down},
		{Type: macros.MouseLeft, Action: macros.Up},
	}
	macro, err := s.CreateMacro("RapidFire", events)
	if err != nil {
		t.Fatal(err)
	}

	// Stage macro assignment to button 6
	staged := s.StageMacroAssignment(macro.ID, 6, 3)
	if staged.Error.Code != "" {
		t.Fatalf("StageMacroAssignment failed: %+v", staged)
	}

	// Apply remap with macro
	applied := s.ApplyRemap(x6.DefaultRemapConfig())
	if applied.Error.Code != "" || applied.Firmware != "success" {
		t.Fatalf("ApplyRemap with macro failed: %+v", applied)
	}

	// Verify macro assignment saved in active profile JSON
	saved, err := store.Read(binding.ID, binding.ProfileID, activeID)
	if err != nil {
		t.Fatal(err)
	}
	var p profileConfiguration
	if err := json.Unmarshal(saved.Configuration, &p); err != nil || len(p.Macros) == 0 {
		t.Fatalf("macro assignment not saved in profile: %+v %v", p, err)
	}
	if p.Macros[0].Button != 6 || p.Macros[0].Repeat != 3 {
		t.Fatalf("saved macro draft mismatch: %+v", p.Macros[0])
	}
}
