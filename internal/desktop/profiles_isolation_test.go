package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"reflect"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestProfileSwitchSelectionAtACK(t *testing.T) {
	for _, lane := range []int{1, 2, 3, 4} {
		t.Run(string(rune('0'+lane)), func(t *testing.T) {
			cmd := &recordingSwitchCommand{}
			s, store, b := switchTestService(t, t.TempDir(), cmd)
			source := s.GetProfiles().ActiveID
			target := s.CreateProfile("target").ActiveID
			p := profileConfiguration{Version: 1, Source: "local settings", Device: x6.DefaultDeviceConfig()}
			p.Device.DPI[0] = 3200
			p.Device.PollingRate = x6.PollingRate500
			raw, _ := json.Marshal(p)
			if err := store.SaveActive(b.ID, b.ProfileID, raw); err != nil {
				t.Fatal(err)
			}
			store.Switch(b.ID, b.ProfileID, source)
			beforeSource, _ := store.Read(b.ID, b.ProfileID, source)
			beforeTarget, _ := store.Read(b.ID, b.ProfileID, target)
			inv := s.RefreshInventory(context.Background())
			otherBinding := s.SelectDevice(inv.Devices[1].ID).Selected
			otherProfiles := s.GetProfiles()
			replacement := workspace(s)
			otherPayload, _ := store.Read(otherBinding.ID, otherBinding.ProfileID, otherProfiles.ActiveID)
			s.SelectDevice(b.ID)
			cmd.during = func() {
				if cmd.calls == lane {
					s.SelectDevice(inv.Devices[1].ID)
				}
			}
			result := s.SwitchProfile(target)
			if result.Applied || result.Error.Code != StaleBinding {
				t.Errorf("stale switch: %+v", result)
			}
			if got := workspace(s); !reflect.DeepEqual(got, replacement) {
				t.Errorf("replacement selection contaminated: before=%+v after=%+v", replacement, got)
			}
			afterOther, _ := store.Read(otherBinding.ID, otherBinding.ProfileID, otherProfiles.ActiveID)
			if !reflect.DeepEqual(otherPayload, afterOther) {
				t.Error("replacement payload changed")
			}
			list, _ := store.List(b.ID, b.ProfileID)
			afterSource, _ := store.Read(b.ID, b.ProfileID, source)
			afterTarget, _ := store.Read(b.ID, b.ProfileID, target)
			if list.ActiveID != source || string(beforeSource.Configuration) != string(afterSource.Configuration) || string(beforeTarget.Configuration) != string(afterTarget.Configuration) {
				t.Error("stale switch changed durable profiles")
			}
			if cmd.calls != lane {
				t.Errorf("writes=%d want %d", cmd.calls, lane)
			}
		})
	}
}

func TestProfilesRetryInvalidation(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	source := s.GetProfiles().ActiveID
	target := s.CreateProfile("target").ActiveID
	store.Switch(b.ID, b.ProfileID, source)
	fail := true
	load := func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }
	save := func(Binding, x6.DeviceConfig) error {
		if fail {
			return errors.New("disk")
		}
		return nil
	}
	s.AttachPollingPersistence(load, save).AttachNormalSleepPersistence(load, save)
	s.AttachDevicePersistence(func(Binding) (x6.DPIConfig, error) { return x6.DefaultDPIConfig(), nil }, func(Binding, x6.DPIConfig) error {
		if fail {
			return errors.New("disk")
		}
		return nil
	})
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
	fail = false
	if r := s.SwitchProfile(target); !r.Applied {
		t.Fatal(r)
	}
	if s.GetSnapshot().RetryAvailable || s.GetPollingSnapshot().RetryAvailable || s.GetNormalSleepSnapshot().RetryAvailable || s.GetDebounceSnapshot().RetryAvailable {
		t.Error("obsolete retries retained")
	}
	before, _ := store.Read(b.ID, b.ProfileID, target)
	calls := cmd.calls
	s.RetryPersistence()
	s.RetryPollingPersistence()
	s.RetryNormalSleepPersistence()
	s.RetryDebouncePersistence()
	after, _ := store.Read(b.ID, b.ProfileID, target)
	if string(before.Configuration) != string(after.Configuration) || cmd.calls != calls {
		t.Error("retry changed target or wrote hardware")
	}
}

func TestProfilesRestoreLightingAndMacros(t *testing.T) {
	root := t.TempDir()
	cmd := &recordingSwitchCommand{}
	s, _, _ := switchTestService(t, root, cmd)
	active := s.GetProfiles().ActiveID
	m, err := s.CreateMacro("macro", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	s.StageMacroAssignment(m.ID, 6, 3)
	if r := s.ApplyRemap(x6.DefaultRemapConfig()); r.Error.Code != "" {
		t.Fatal(r)
	}
	light := x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonTwo}
	s.StageLighting(light)
	if r := s.ApplyLighting(); r.Error.Code != "" {
		t.Fatal(r)
	}
	restored, _, _ := switchTestService(t, root, &recordingSwitchCommand{})
	restored.macroLibrary = s.macroLibrary
	if r := restored.GetProfiles(); r.ActiveID != active || r.Error.Code != "" {
		t.Fatal(r)
	}
	if restored.GetLightingSnapshot().Pending != light || len(restored.GetRemapSnapshot().MacroDrafts) != 1 {
		t.Error("saved lighting/macros not restored")
	}
	if _, err := s.UpdateMacro(m.ID, "renamed", m.Events); err != nil {
		t.Fatal(err)
	}
	if r := s.GetProfiles(); r.Error.Code != "" {
		t.Fatal(r)
	}
	s.StagePollingRate(x6.PollingRate500)
	if r := s.ApplyPollingRate(context.Background()); r.Error.Code != "" || r.Persistence != "success" {
		t.Fatal(r)
	}
}

func TestProfilesInflightRetrySerializesSwitch(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	source := s.GetProfiles().ActiveID
	target := s.CreateProfile("target").ActiveID
	store.Switch(b.ID, b.ProfileID, source)
	entered, release := make(chan struct{}), make(chan struct{})
	fail := true
	s.AttachDevicePersistence(func(Binding) (x6.DPIConfig, error) { return x6.DefaultDPIConfig(), nil }, func(Binding, x6.DPIConfig) error {
		if fail {
			return errors.New("disk")
		}
		close(entered)
		<-release
		return nil
	})
	d := s.GetSnapshot().Pending
	d.DPI[0] = 2800
	s.StageDPI(d)
	s.ApplyDPI(context.Background())
	fail = false
	retryDone := make(chan struct{})
	go func() { s.RetryPersistence(); close(retryDone) }()
	<-entered
	// Subsequent switch persistence uses the same saver, so remove its blocking seam
	// only after retry finishes. Switch must not enter hardware before release.
	switched := make(chan ProfileSnapshot, 1)
	s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error { return nil })
	go func() { switched <- s.SwitchProfile(target) }()
	select {
	case r := <-switched:
		close(release)
		t.Fatalf("switch overtook retry: %+v", r)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-retryDone
	if r := <-switched; !r.Applied {
		t.Fatal(r)
	}
	saved, _ := store.Read(b.ID, b.ProfileID, target)
	var p profileConfiguration
	json.Unmarshal(saved.Configuration, &p)
	if p.Device.DPI[0] != x6.DefaultDPIConfig().DPI[0] {
		t.Fatal("retry corrupted target")
	}
}

func TestProfilesSwitchRetainsFailedMacroProgress(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, _, _ := switchTestService(t, t.TempDir(), cmd)
	id := s.GetProfiles().ActiveID
	m, err := s.CreateMacro("macro", []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}})
	if err != nil {
		t.Fatal(err)
	}
	s.StageMacroAssignment(m.ID, 6, 3)
	s.ApplyRemap(x6.DefaultRemapConfig())
	if r := s.SwitchProfile(id); !r.Applied {
		t.Fatal(r)
	}
	cmd.calls = 0
	cmd.failAtCall = 4
	if r := s.SwitchProfile(id); r.Applied || r.Error.Code == "" {
		t.Fatal(r)
	}
	if p := s.GetRemapSnapshot().MacroProgress; p.Upload != mouse.MacroUploadNotStarted || p.Assignment != mouse.MacroAssignmentNotStarted {
		t.Fatalf("stale progress %+v", p)
	}
}

func TestProfilesNoStorePersistenceCompatibility(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, _, _ := switchTestService(t, t.TempDir(), cmd)
	s.profiles = nil
	s.StagePollingRate(x6.PollingRate500)
	if r := s.ApplyPollingRate(context.Background()); r.Persistence != "" {
		t.Errorf("polling persistence=%q", r.Persistence)
	}
	if r := s.ApplyRemap(x6.DefaultRemapConfig()); r.Persistence != "" {
		t.Errorf("remap persistence=%q", r.Persistence)
	}
}

func TestProfileSwitchLegacyPersistenceFailure(t *testing.T) {
	cmd := &recordingSwitchCommand{}
	s, store, b := switchTestService(t, t.TempDir(), cmd)
	source := s.GetProfiles().ActiveID
	target := s.CreateProfile("target").ActiveID
	store.Switch(b.ID, b.ProfileID, source)
	s.AttachPollingPersistence(func(Binding) (x6.DeviceConfig, error) { return x6.DefaultDeviceConfig(), nil }, func(Binding, x6.DeviceConfig) error { return errors.New("disk unavailable") })
	result := s.SwitchProfile(target)
	list, _ := store.List(b.ID, b.ProfileID)
	if result.Applied || result.Error.Code != PersistenceFailed || list.ActiveID != source {
		t.Fatalf("result=%+v active=%s", result, list.ActiveID)
	}
}
