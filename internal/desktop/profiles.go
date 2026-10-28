package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

const (
	ProfileUnavailable ErrorCode = "profile_unavailable"
	InvalidProfile     ErrorCode = "invalid_profile"
	ProtectedProfile   ErrorCode = "protected_profile"
	DuplicateProfile   ErrorCode = "duplicate_profile_name"
)

// ProfileSnapshot describes local files, never onboard slots or hardware readback.
// Keep these desktop-owned DTOs separate from the storage JSON schema.
type ProfileSnapshot struct {
	Profiles []ProfileSummary
	ActiveID string
	Notice   string
	Error    Error
	Applied  bool
}
type ProfileSummary struct{ ID, Name, CreatedAt string }

// profileConfiguration wraps the complete model configuration. Metadata absent
// from historical device records is optional, not inferred hardware state.
type profileConfiguration struct {
	Version  int                   `json:"version"`
	Source   string                `json:"source"`
	Device   x6.DeviceConfig       `json:"device"`
	Lighting *x6.LightingSelection `json:"lighting,omitempty"`
	Macros   []MacroDraft          `json:"macros,omitempty"`
}
type profileComponent struct {
	store     *configstore.ProfileStore
	load      func(Binding) (x6.DeviceConfig, error)
	restoreMu sync.Mutex // protects hydration flags, independent of operationMu
	loaded    map[DeviceID]bool
}

// AttachProfiles wires one store per application root before publication. The
// loader must return os.ErrNotExist only for genuinely absent local settings.
func (s *Service) AttachProfiles(store *configstore.ProfileStore, load func(Binding) (x6.DeviceConfig, error)) *Service {
	s.profiles = &profileComponent{store: store, load: load, loaded: make(map[DeviceID]bool)}
	return s
}

func (s *Service) profileSelection() (Binding, ErrorCode) {
	binding, ok := s.selectedBinding()
	if !ok {
		return binding, SelectionRequired
	}
	if binding.SessionOnly || s.profiles == nil || s.profiles.store == nil {
		return binding, ProfileUnavailable
	}
	return binding, ""
}
func profileError(err error) ErrorCode {
	if errors.Is(err, configstore.ErrProtectedProfile) {
		return ProtectedProfile
	}
	if errors.Is(err, configstore.ErrDuplicateProfileName) {
		return DuplicateProfile
	}
	if errors.Is(err, configstore.ErrInvalidProfile) {
		return InvalidProfile
	}
	return PersistenceFailed
}
func (s *Service) validateProfile(binding Binding, p profileConfiguration) error {
	if p.Version != 1 || (p.Source != "local settings" && p.Source != "app defaults") {
		return configstore.ErrInvalidProfile
	}
	if _, err := x6.EncodeDPIReport(p.Device.DPIConfig); err != nil {
		return err
	}
	if err := x6.NewPollingOperation().Validate(p.Device.PollingRate); err != nil {
		return err
	}
	if err := x6.ValidateNormalSleepMinutes(p.Device.NormalSleepMinutes); err != nil {
		return err
	}
	if err := x6.ValidateResponseTime(p.Device.ResponseTimeMs); err != nil {
		return err
	}
	remap := x6.DefaultRemapConfig()
	if p.Device.Remap != nil {
		remap = *p.Device.Remap
	}
	if err := x6.NewRemapOperation().Validate(remap); err != nil {
		return err
	}
	if p.Lighting != nil {
		if err := x6.NewLightingOperation().Validate(*p.Lighting); err != nil {
			return err
		}
	}
	if len(p.Macros) > 0 {
		drafts := make([]*MacroDraft, len(p.Macros))
		for i := range p.Macros {
			drafts[i] = cloneMacroDraft(&p.Macros[i])
			// Saved names are presentation metadata. Validate identity/content
			// against the shared library without rewriting either saved object.
			if m, err := s.ReadMacro(drafts[i].ID); err == nil {
				drafts[i].Name = m.Name
			}
		}
		if code := s.remapComponent.prevalidateMacroBound(binding, remap, drafts); code != "" {
			return fmt.Errorf("profile macros: %s", code)
		}
	}
	return nil
}
func (s *Service) readProfile(binding Binding, id string) (profileConfiguration, error) {
	saved, err := s.profiles.store.Read(binding.ID, binding.ProfileID, id)
	if err != nil {
		return profileConfiguration{}, err
	}
	var p profileConfiguration
	if err = json.Unmarshal(saved.Configuration, &p); err != nil {
		return p, err
	}
	return p, s.validateProfile(binding, p)
}
func (s *Service) profileSnapshot(binding Binding, saved configstore.ProfileSnapshot) ProfileSnapshot {
	result := ProfileSnapshot{Profiles: make([]ProfileSummary, 0, len(saved.Profiles)), ActiveID: saved.ActiveID, Notice: saved.Notice}
	for _, p := range saved.Profiles {
		result.Profiles = append(result.Profiles, ProfileSummary{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt})
	}
	// Bootstrap's source is opaque to storage. Correct its notice using durable
	// payload metadata, including after restart, without rewriting original files.
	if result.Notice != "" {
		if p, err := s.readProfile(binding, saved.ActiveID); err == nil && p.Source == "app defaults" {
			result.Notice = strings.Replace(result.Notice, "from local settings", "from app defaults", 1)
		}
	}
	return result
}

// profilesLocked only reads saved files or creates the first automatic local
// copy. It never reads or writes hardware, and never turns damaged data into
// defaults. Caller holds operationMu to serialize active-pointer changes.
func (s *Service) profilesLocked(binding Binding) ProfileSnapshot {
	saved, err := s.profiles.store.List(binding.ID, binding.ProfileID)
	if errors.Is(err, os.ErrNotExist) {
		config := x6.DefaultDeviceConfig()
		source := "app defaults"
		if s.profiles.load != nil {
			loaded, loadErr := s.profiles.load(binding)
			if loadErr == nil {
				config = loaded
				source = "local settings"
			} else if !errors.Is(loadErr, os.ErrNotExist) {
				return ProfileSnapshot{Error: Error{Code: ProfileUnavailable}}
			}
		}
		p := profileConfiguration{Version: 1, Source: source, Device: config}
		if err = s.validateProfile(binding, p); err != nil {
			return ProfileSnapshot{Error: Error{Code: InvalidProfile}}
		}
		raw, marshalErr := json.Marshal(p)
		if marshalErr != nil {
			return ProfileSnapshot{Error: Error{Code: InvalidProfile}}
		}
		saved, err = s.profiles.store.Bootstrap(binding.ID, binding.ProfileID, raw)
	}
	if err != nil {
		return ProfileSnapshot{Error: Error{Code: ProfileUnavailable}}
	}
	result := s.profileSnapshot(binding, saved)
	// Validate all saved payloads, not just index metadata or the active profile.
	for _, p := range saved.Profiles {
		if _, err = s.readProfile(binding, p.ID); err != nil {
			result.Error = Error{Code: InvalidProfile}
			return result
		}
	}
	return result
}
func (s *Service) GetProfiles() ProfileSnapshot {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, code := s.profileSelection()
	if code != "" {
		return ProfileSnapshot{Error: Error{Code: code}}
	}
	result := s.profilesLocked(binding)
	if result.Error.Code == "" {
		result.Error.Code = s.restoreSavedProfile(binding)
	}
	return result
}

// profileLoaded distinguishes cached session truth from legacy records.
func (s *Service) profileLoaded(b Binding) bool {
	if s.profiles == nil || b.SessionOnly {
		return false
	}
	s.profiles.restoreMu.Lock()
	defer s.profiles.restoreMu.Unlock()
	return s.profiles.loaded[b.ID]
}

// restoreSavedProfile reads only existing profiles at selection boundaries. It
// never bootstraps, takes operationMu, or sends hardware commands, so a transport
// callback may reenter SelectDevice while an apply owns operationMu.
func (s *Service) restoreSavedProfile(b Binding) ErrorCode {
	if s.profiles == nil || s.profiles.store == nil || b.SessionOnly {
		return ""
	}
	c := s.profiles
	c.restoreMu.Lock()
	defer c.restoreMu.Unlock()
	if c.loaded[b.ID] {
		return ""
	}
	saved, err := c.store.List(b.ID, b.ProfileID)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return ProfileUnavailable
	}
	p, err := s.readProfile(b, saved.ActiveID)
	if err != nil {
		return InvalidProfile
	}
	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		return StaleBinding
	}
	if err := s.restoreProfile(inventory, b, p); err != nil {
		return errorCode(err, false)
	}
	c.loaded[b.ID] = true
	return ""
}

// restoreProfile hydrates captured local state only; it makes no hardware claims
// or writes. It must not resolve selection while CompleteBound owns that lock.
func (s *Service) restoreProfile(inventory *mouse.TargetedService, b Binding, p profileConfiguration) error {
	s.mu.Lock()
	dpi := s.states[b.ID]
	pol := s.pollingComponent.states[b.ID]
	s.mu.Unlock()
	settings := s.settingsStateForBinding(b)
	light := s.lightingComponent.stateForBinding(b)
	rem := s.remapComponent.stateForBinding(b)
	return completeProfileBinding(inventory, b, func() error {
		dpi.applied, dpi.pending = p.Device.DPIConfig, p.Device.DPIConfig
		pol.applied, pol.desired = p.Device.PollingRate, p.Device.PollingRate
		savedRate := p.Device.PollingRate
		pol.persisted = &savedRate
		pol.persistence = "success"
		settings.replaceLocked(p.Device)
		light.pending = x6.DefaultLightingSettings().LightingSelection
		if p.Lighting != nil {
			light.pending = *p.Lighting
		}
		remap := x6.DefaultRemapConfig()
		if p.Device.Remap != nil {
			remap = *p.Device.Remap
		}
		rem.applied, rem.pending = cloneRemapConfig(remap), cloneRemapConfig(remap)
		rem.macroPendingMap = make(map[uint8]*MacroDraft)
		rem.macroAppliedMap = make(map[uint8]*MacroDraft)
		rem.macroPending, rem.macroApplied = nil, nil
		for i := range p.Macros {
			d := &p.Macros[i]
			rem.macroPendingMap[d.Button] = cloneMacroDraft(d)
			rem.macroAppliedMap[d.Button] = cloneMacroDraft(d)
		}
		return nil
	}, &dpi.mu, &pol.mu, &settings.mu, &light.mu, &rem.mu)
}

// State locks precede the inventory mutex, matching existing staging paths that
// check a binding while holding a lane lock. No callback here resolves selection,
// emits events, takes operationMu, or invokes hardware or an injected saver.
func completeProfileBinding(inventory *mouse.TargetedService, b Binding, commit func() error, locks ...*sync.Mutex) error {
	for _, lock := range locks {
		lock.Lock()
	}
	defer func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}()
	return inventory.CompleteBound(b, commit)
}
func (s *Service) profileMutation(change func(*configstore.ProfileStore, Binding) (configstore.ProfileSnapshot, error)) ProfileSnapshot {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	binding, code := s.profileSelection()
	if code != "" {
		return ProfileSnapshot{Error: Error{Code: code}}
	}
	before := s.profilesLocked(binding)
	if before.Error.Code != "" {
		return before
	}
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	saved, err := change(s.profiles.store, binding)
	if err != nil {
		before.Error = Error{Code: profileError(err)}
		return before
	}
	return s.profileSnapshot(binding, saved)
}

// CreateProfile activates an identical clone of the active saved configuration,
// not the pending UI draft. No device operation is implied by this local action.
func (s *Service) CreateProfile(name string) ProfileSnapshot {
	return s.profileMutation(func(store *configstore.ProfileStore, b Binding) (configstore.ProfileSnapshot, error) {
		return store.Create(b.ID, b.ProfileID, name)
	})
}
func (s *Service) RenameProfile(id, name string) ProfileSnapshot {
	return s.profileMutation(func(store *configstore.ProfileStore, b Binding) (configstore.ProfileSnapshot, error) {
		return store.Rename(b.ID, b.ProfileID, id, name)
	})
}
func (s *Service) DuplicateProfile(id, name string) ProfileSnapshot {
	return s.profileMutation(func(store *configstore.ProfileStore, b Binding) (configstore.ProfileSnapshot, error) {
		return store.Duplicate(b.ID, b.ProfileID, id, name)
	})
}
func (s *Service) DeleteProfile(id string) ProfileSnapshot {
	return s.profileMutation(func(store *configstore.ProfileStore, b Binding) (configstore.ProfileSnapshot, error) {
		return store.Delete(b.ID, b.ProfileID, id)
	})
}
func (s *Service) DismissProfileNotice() ProfileSnapshot {
	return s.profileMutation(func(store *configstore.ProfileStore, b Binding) (configstore.ProfileSnapshot, error) {
		return store.DismissNotice(b.ID, b.ProfileID)
	})
}

func (s *Service) quiesceProfiles(binding Binding) {
	s.dpiComponent.cancelSync(binding)
	s.pollingComponent.cancelSync(binding)
	s.mu.Lock()
	dpiState := s.states[binding.ID]
	pollingState := s.pollingComponent.states[binding.ID]
	s.mu.Unlock()
	dpiState.mu.Lock()
	dpiState.revision++
	dpiState.retry = nil
	dpiState.mu.Unlock()
	pollingState.mu.Lock()
	pollingState.revision++
	pollingState.retry = nil
	pollingState.mu.Unlock()
	settings := s.settingsStateForBinding(binding)
	settings.mu.Lock()
	settings.retryNormal, settings.retryResponse = nil, nil
	settings.mu.Unlock()
	remap := s.remapComponent.stateForBinding(binding)
	remap.mu.Lock()
	remap.retry = nil
	remap.mu.Unlock()
}

// SwitchProfile applies the entire target configuration across all lanes using
// the same captured binding and targeted bound operations. The active pointer in
// the profile store is committed only after all ACKs succeed.
func (s *Service) SwitchProfile(id string) ProfileSnapshot {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()

	binding, code := s.profileSelection()
	if code != "" {
		return ProfileSnapshot{Error: Error{Code: code}}
	}

	before := s.profilesLocked(binding)
	if before.Error.Code != "" {
		return before
	}
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}

	target, err := s.readProfile(binding, id)
	if err != nil {
		before.Error = Error{Code: profileError(err)}
		return before
	}

	s.mu.Lock()
	inventory := s.inventory
	s.mu.Unlock()
	if inventory == nil {
		before.Error = Error{Code: StaleBinding}
		return before
	}

	s.quiesceProfiles(binding)
	// Capture all state before I/O: callbacks may select another mouse.
	s.mu.Lock()
	dpiState := s.states[binding.ID]
	polState := s.pollingComponent.states[binding.ID]
	s.mu.Unlock()

	ctx := context.Background()

	// 1. DPI
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	if err := inventory.ApplyOperationBound(ctx, binding, x6.NewDPIOperation(), target.Device.DPIConfig); err != nil {
		dpiState.mu.Lock()
		dpiState.firmware, dpiState.err = "failed", Error{Code: errorCode(err, false)}
		dpiState.mu.Unlock()
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	if err := completeProfileBinding(inventory, binding, func() error {
		dpiState.applied, dpiState.pending = target.Device.DPIConfig, target.Device.DPIConfig
		dpiState.firmware, dpiState.persistence, dpiState.err = "success", "", Error{}
		return nil
	}, &dpiState.mu); err != nil {
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	s.emitConfiguration(binding, dpiState)

	// 2. Polling
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	if err := inventory.ApplyOperationBound(ctx, binding, x6.NewPollingOperation(), target.Device.PollingRate); err != nil {
		polState.mu.Lock()
		polState.firmware, polState.err = "failed", Error{Code: errorCode(err, false)}
		polState.mu.Unlock()
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	if err := completeProfileBinding(inventory, binding, func() error {
		polState.applied, polState.desired = target.Device.PollingRate, target.Device.PollingRate
		polState.firmware, polState.persistence, polState.err = "success", "", Error{}
		return nil
	}, &polState.mu); err != nil {
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	s.pollingComponent.emitConfiguration(binding, pollingSnapshotOf(polState))

	// 3. LightingSettings (Lighting + NormalSleep + Debounce)
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	lightingSelection := x6.DefaultLightingSettings().LightingSelection
	if target.Lighting != nil {
		lightingSelection = *target.Lighting
	}
	lightingSettings := x6.LightingSettings{
		LightingSelection:  lightingSelection,
		NormalSleepMinutes: target.Device.NormalSleepMinutes,
		ResponseTimeMs:     target.Device.ResponseTimeMs,
	}
	if err := inventory.ApplyOperationBound(ctx, binding, x6.NewLightingSettingsOperation(), lightingSettings); err != nil {
		stState := s.settingsStateForBinding(binding)
		stState.mu.Lock()
		stState.normalFirmware, stState.responseFirmware = "failed", "failed"
		stState.normalError = Error{Code: errorCode(err, false)}
		stState.responseError = Error{Code: errorCode(err, false)}
		stState.mu.Unlock()

		ltState := s.lightingComponent.stateForBinding(binding)
		ltState.mu.Lock()
		ltState.firmware = "failed"
		ltState.err = Error{Code: errorCode(err, false)}
		ltState.mu.Unlock()

		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	stState := s.settingsStateForBinding(binding)
	ltState := s.lightingComponent.stateForBinding(binding)
	if err := completeProfileBinding(inventory, binding, func() error {
		stState.normalSleep, stState.responseTimeMs = target.Device.NormalSleepMinutes, target.Device.ResponseTimeMs
		stState.normalFirmware, stState.responseFirmware = "success", "success"
		stState.normalPersistence, stState.responsePersistence = "", ""
		stState.normalError, stState.responseError = Error{}, Error{}
		appliedLight := lightingSelection
		ltState.applied = &appliedLight
		ltState.pending = lightingSelection
		ltState.firmware = "success"
		ltState.err = Error{}
		return nil
	}, &stState.mu, &ltState.mu); err != nil {
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}

	// 4. Remap and Macros
	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	remapConfig := x6.DefaultRemapConfig()
	if target.Device.Remap != nil {
		remapConfig = *target.Device.Remap
	}
	remState := s.remapComponent.stateForBinding(binding)

	if len(target.Macros) > 0 {
		drafts := make([]*MacroDraft, len(target.Macros))
		for i := range target.Macros {
			drafts[i] = cloneMacroDraft(&target.Macros[i])
		}
		items := make([]mouse.MacroSequenceItem, len(drafts))
		localItems := make([]mouse.MacroLocalItem, len(drafts))
		legacy := true
		for i, draft := range drafts {
			seq, valid := admittedSequenceWithMode(draft.Events, draft.Repeat, draft.Mode)
			legacy = legacy && valid
			localItems[i] = mouse.MacroLocalItem{Button: draft.Button, Events: append([]macros.Event(nil), draft.Events...), Repeat: draft.Repeat, Mode: draft.Mode}
			items[i] = mouse.MacroSequenceItem{Button: draft.Button, Sequence: seq}
		}
		var progress mouse.MacroProgress
		var remapErr error
		if legacy {
			progress, remapErr = inventory.ApplyMultiMacroSequenceAssignmentBound(ctx, binding, remapConfig, items)
		} else {
			progress, remapErr = inventory.ApplyMultiMacroLocalAssignmentBound(ctx, binding, remapConfig, localItems)
		}
		remState.mu.Lock()
		remState.macroProgress = progress
		remState.mu.Unlock()
		if !s.bindingCurrent(binding) {
			before.Error = Error{Code: StaleBinding}
			return before
		}
		if remapErr != nil || progress.Assignment != mouse.MacroAssignmentACKConfirmed || progress.Upload != mouse.MacroUploadConfirmed {
			remState.mu.Lock()
			remState.firmware = "failed"
			if remapErr != nil {
				remState.err = Error{Code: errorCode(remapErr, false)}
			} else {
				logApplyError(ApplyFailed, fmt.Sprintf("profile_macro_unconfirmed assignment=%v upload=%v", progress.Assignment, progress.Upload), nil)
				remState.err = Error{Code: ApplyFailed}
			}
			remState.mu.Unlock()
			before.Error = remState.err
			return before
		}
		if err := completeProfileBinding(inventory, binding, func() error {
			remState.applied, remState.pending = cloneRemapConfig(remapConfig), cloneRemapConfig(remapConfig)
			remState.macroAppliedMap = make(map[uint8]*MacroDraft, len(drafts))
			remState.macroPendingMap = make(map[uint8]*MacroDraft, len(drafts))
			for _, d := range drafts {
				remState.macroAppliedMap[d.Button] = cloneMacroDraft(d)
				remState.macroPendingMap[d.Button] = cloneMacroDraft(d)
			}
			remState.macroApplied = cloneMacroDraft(drafts[0])
			remState.macroPending = cloneMacroDraft(drafts[0])
			remState.firmware, remState.persistence, remState.err = "success", "", Error{}
			return nil
		}, &remState.mu); err != nil {
			before.Error = Error{Code: errorCode(err, false)}
			return before
		}
		s.emitRemapConfiguration(binding, remapSnapshotOf(remState))
	} else {
		if err := inventory.ApplyOperationBound(ctx, binding, x6.NewRemapOperation(), remapConfig); err != nil {
			remState.mu.Lock()
			remState.firmware, remState.err = "failed", Error{Code: errorCode(err, false)}
			remState.mu.Unlock()
			before.Error = Error{Code: errorCode(err, false)}
			return before
		}
		if !s.bindingCurrent(binding) {
			before.Error = Error{Code: StaleBinding}
			return before
		}
		if err := completeProfileBinding(inventory, binding, func() error {
			remState.applied, remState.pending = cloneRemapConfig(remapConfig), cloneRemapConfig(remapConfig)
			remState.macroAppliedMap = make(map[uint8]*MacroDraft)
			remState.macroPendingMap = make(map[uint8]*MacroDraft)
			remState.macroApplied, remState.macroPending = nil, nil
			remState.firmware, remState.persistence, remState.err = "success", "", Error{}
			return nil
		}, &remState.mu); err != nil {
			before.Error = Error{Code: errorCode(err, false)}
			return before
		}
		s.emitRemapConfiguration(binding, remapSnapshotOf(remState))
	}

	if !s.bindingCurrent(binding) {
		before.Error = Error{Code: StaleBinding}
		return before
	}
	if s.pollingComponent.persistence != nil {
		err = s.saveDeviceConfig(binding, s.pollingComponent.persistence, func(c *x6.DeviceConfig) { *c = target.Device })
	} else if s.devicePersistence != nil {
		err = s.devicePersistence.Save(binding, target.Device.DPIConfig)
	}
	// Do not hold the selection lock across an injected legacy saver: it may
	// reenter selection just like an ACK callback. Only activation and captured
	// snapshot completion share the final selection critical section.
	var switched configstore.ProfileSnapshot
	saveErr := err
	err = completeProfileBinding(inventory, binding, func() error {
		if saveErr == nil {
			switched, saveErr = s.profiles.store.Switch(binding.ID, binding.ProfileID, id)
		}
		persistence, completionError := "success", Error{}
		if saveErr != nil {
			persistence, completionError = "failed", Error{Code: PersistenceFailed}
		}
		dpiState.persistence, dpiState.err = persistence, completionError
		polState.persistence, polState.err = persistence, completionError
		if saveErr == nil {
			polState.persisted = &target.Device.PollingRate
		}
		stState.normalPersistence, stState.responsePersistence = persistence, persistence
		stState.normalError, stState.responseError = completionError, completionError
		if saveErr == nil {
			stState.persistedNormal, stState.persistedResponse = &target.Device.NormalSleepMinutes, &target.Device.ResponseTimeMs
		}
		ltState.err = completionError
		remState.persistence, remState.err = persistence, completionError
		return nil
	}, &dpiState.mu, &polState.mu, &stState.mu, &ltState.mu, &remState.mu)
	if err != nil {
		before.Error = Error{Code: errorCode(err, false)}
		return before
	}
	if saveErr != nil {
		before.Error = Error{Code: profileError(saveErr)}
		return before
	}

	result := s.profileSnapshot(binding, switched)
	result.Applied = true
	result.Error = Error{}
	return result
}

// saveActiveProfile updates the active local profile JSON alongside ordinary
// setting edits. It never changes the active pointer or duplicates hardware writes.
func (s *Service) saveActiveProfile(binding Binding, mutate func(*profileConfiguration)) error {
	if s.profiles == nil || s.profiles.store == nil || binding.SessionOnly {
		return nil
	}
	snap := s.profilesLocked(binding)
	if snap.Error.Code != "" {
		return errors.New(string(snap.Error.Code))
	}
	if snap.ActiveID == "" {
		return configstore.ErrInvalidProfile
	}
	prof, err := s.readProfile(binding, snap.ActiveID)
	if err != nil {
		return err
	}
	mutate(&prof)
	if err := s.validateProfile(binding, prof); err != nil {
		return err
	}
	raw, err := json.Marshal(prof)
	if err != nil {
		return err
	}
	return s.profiles.store.SaveActive(binding.ID, binding.ProfileID, raw)
}
