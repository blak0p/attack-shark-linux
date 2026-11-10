package desktop

import (
	"github.com/blak0p/attack-shark-linux/internal/device"
	"context"
	"encoding/json"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

type ResetLaneResult struct {
	Lane  string
	State string
	Code  ErrorCode
}

type ResetResult struct {
	Lanes          []ResetLaneResult
	Cleanup        ResetLaneResult
	Error          Error
	RetryAvailable bool
}

type PendingWrites interface {
	Quiesce(context.Context, Binding) error
}

// ResetPersister records the reset baseline for the mouse the reset wrote. It
// must leave every other local record (profiles, macros, app settings) intact.
type ResetPersister interface{ PersistReset(Binding) error }

type resetTarget interface {
	Refresh(context.Context) ([]mouse.Device, error)
	Selection() (mouse.Binding, bool)
	ApplyOperationBound(context.Context, mouse.Binding, mouse.CommandOperation, any) error
}

// EmergencyReset performs the documented reset lanes synchronously. Persistence
// is deliberately unreachable until every physical ACK has succeeded.
type EmergencyReset struct {
	model     device.Model
	target    resetTarget
	pending   PendingWrites
	persister ResetPersister
}

func NewEmergencyReset(model device.Model, target resetTarget, pending PendingWrites, persister ResetPersister) *EmergencyReset {
	return &EmergencyReset{model: model, target: target, pending: pending, persister: persister}
}

func (r *EmergencyReset) Run(ctx context.Context) (ResetResult, error) {
	result := ResetResult{Lanes: []ResetLaneResult{{Lane: "dpi", State: "not_attempted"}, {Lane: "settings", State: "not_attempted"}, {Lane: "polling", State: "not_attempted"}, {Lane: "remap", State: "not_attempted"}}, Cleanup: ResetLaneResult{Lane: "cleanup", State: "not_attempted"}}
	if err := ctx.Err(); err != nil {
		return resetFailure(result, err), err
	}
	if r.target == nil {
		return resetFailure(result, fmt.Errorf("reset target: %w", mouse.ErrSelectionRequired)), fmt.Errorf("reset target: %w", mouse.ErrSelectionRequired)
	}
	devices, err := r.target.Refresh(ctx)
	if err != nil {
		wrapped := fmt.Errorf("refresh reset target: %w", err)
		return resetFailure(result, wrapped), wrapped
	}
	eligible := 0
	for _, device := range devices {
		if device.Eligible {
			eligible++
		}
	}
	if eligible != 1 {
		wrapped := fmt.Errorf("discover reset target: %w", mouse.ErrSelectionRequired)
		return resetFailure(result, wrapped), wrapped
	}
	binding, ok := r.target.Selection()
	if !ok {
		wrapped := fmt.Errorf("select reset target: %w", mouse.ErrSelectionRequired)
		return resetFailure(result, wrapped), wrapped
	}
	if r.pending != nil {
		if err := r.pending.Quiesce(ctx, binding); err != nil {
			wrapped := fmt.Errorf("quiesce pending writes: %w", err)
			return resetFailure(result, wrapped), wrapped
		}
	}
	lanes := []struct {
		operation mouse.CommandOperation
		value     any
	}{
		{r.model.DPIOperation(), device.DocumentedResetDPIConfig()},
		{r.model.LightingSettingsOperation(), device.DocumentedResetLightingSettings()},
		{r.model.PollingOperation(), device.PollingRate1000},
		{r.model.RemapOperation(), device.DefaultRemapConfig()},
	}
	for index, lane := range lanes {
		if err := r.target.ApplyOperationBound(ctx, binding, lane.operation, lane.value); err != nil {
			result.Lanes[index] = ResetLaneResult{Lane: result.Lanes[index].Lane, State: "failed", Code: errorCode(err, false)}
			wrapped := fmt.Errorf("reset %s: %w", result.Lanes[index].Lane, err)
			return resetFailure(result, wrapped), wrapped
		}
		result.Lanes[index] = ResetLaneResult{Lane: result.Lanes[index].Lane, State: "success"}
	}
	if r.persister == nil {
		wrapped := fmt.Errorf("persist reset state: no persister")
		return resetFailure(result, wrapped), wrapped
	}
	if err := r.persister.PersistReset(binding); err != nil {
		result.Cleanup = ResetLaneResult{Lane: "cleanup", State: "failed", Code: PersistenceFailed}
		wrapped := fmt.Errorf("persist reset state: %w", err)
		result.Error = Error{Code: PersistenceFailed}
		result.RetryAvailable = true
		return result, wrapped
	}
	result.Cleanup = ResetLaneResult{Lane: "cleanup", State: "success"}
	return result, nil
}

// resetBaseline is the configuration the four reset lanes write.
func resetBaseline() (device.DeviceConfig, device.LightingSelection) {
	settings := device.DocumentedResetLightingSettings()
	remap := device.DefaultRemapConfig()
	return device.DeviceConfig{
		DPIConfig:          device.DocumentedResetDPIConfig(),
		PollingRate:        device.PollingRate1000,
		NormalSleepMinutes: settings.NormalSleepMinutes,
		ResponseTimeMs:     settings.ResponseTimeMs,
		Remap:              &remap,
	}, settings.LightingSelection
}

// PersistReset saves the reset baseline as the mouse's device record and as the
// whole payload of its active local profile. Other profiles, the macro library
// and app settings stay untouched. A profile store the storage layer refuses as
// damaged is never overwritten: the reset then reports a persistence failure.
// Caller holds operationMu.
func (s *Service) PersistReset(binding Binding) error {
	if binding.SessionOnly {
		return nil
	}
	config, lighting := resetBaseline()
	s.mu.Lock()
	persistence := s.pollingComponent.persistence
	s.mu.Unlock()
	if persistence != nil {
		if err := s.saveDeviceConfig(binding, persistence, func(c *device.DeviceConfig) { *c = config }); err != nil {
			return fmt.Errorf("save device record: %w", err)
		}
	}
	if s.profiles == nil || s.profiles.store == nil {
		return nil
	}
	snap := s.profilesLocked(binding)
	if snap.ActiveID == "" {
		return fmt.Errorf("active profile: %s", snap.Error.Code)
	}
	profile := profileConfiguration{Version: 1, Source: "local settings", Device: config, Lighting: &lighting}
	if previous, err := s.readProfile(binding, snap.ActiveID); err == nil {
		profile.Source = previous.Source
	}
	if err := s.validateProfile(binding, profile); err != nil {
		return err
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return s.profiles.store.SaveActive(binding.ID, binding.ProfileID, raw)
}

func resetFailure(result ResetResult, err error) ResetResult {
	result.Error = Error{Code: errorCode(err, false)}
	result.RetryAvailable = true
	return result
}

// Quiesce cancels scheduled DPI and polling writes and invalidates their captured
// revisions before a reset writes the same selected device.
func (s *Service) Quiesce(_ context.Context, binding Binding) error {
	s.mu.Lock()
	sync, pollingSync := s.sync, s.pollingSync
	s.mu.Unlock()
	if sync != nil {
		sync.Cancel(binding)
	}
	if pollingSync != nil {
		pollingSync.Cancel(binding)
	}
	state := s.currentState()
	state.mu.Lock()
	state.revision++
	state.mu.Unlock()
	polling := s.currentPollingState()
	polling.mu.Lock()
	polling.revision++
	polling.mu.Unlock()
	return nil
}
