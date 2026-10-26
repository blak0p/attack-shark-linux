package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	"github.com/blak0p/attack-shark-linux/internal/udevpolicy"
	"github.com/blak0p/attack-shark-linux/internal/update"
)

// PolicyApplyResult distinguishes a completed write from failed reload/trigger.
type PolicyApplyResult struct{ Installed bool }

// PolicyUpdateBackend is trusted Go composition, never a Service capability.
// Check must return only authenticated T3a candidates. Apply must independently
// revalidate the signed candidate and installed policy under the helper lock.
// An Installed result, even with an error, must never be applied again.
type PolicyUpdateBackend interface {
	Check(context.Context) (*udevauth.Policy, error)
	Apply(context.Context, *udevauth.Policy) (PolicyApplyResult, error)
}

// UnifiedUpdateInfo contains display values only, not privileged inputs.
type UnifiedUpdateInfo struct {
	Version         string
	AppAvailable    bool
	AppInstalled    bool
	AppError        string
	PolicyState     string
	PolicyInstalled bool
	PolicyError     string
	RelaunchError   string
}

type unifiedUpdateState struct {
	backend   PolicyUpdateBackend
	source    udevpolicy.Source
	candidate *udevauth.Policy
	checked   bool
	info      UnifiedUpdateInfo
}

// ConfigureUnifiedUpdates must run after ConfigureUpdater and before registration.
// No policy, path, key, callback or release selector is renderer configurable.
func ConfigureUnifiedUpdates(s *Service, backend PolicyUpdateBackend, source udevpolicy.Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.update == nil {
		s.update = &updateState{}
	}
	s.update.unified = &unifiedUpdateState{backend: backend, source: source}
}

func (s *Service) updateState() *updateState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update
}

// policyState uses authenticated remote history, not the local embedded current.
func policyState(source udevpolicy.Source, candidate *udevauth.Policy) (string, error) {
	if candidate == nil {
		return "unavailable", nil
	}
	if source == nil {
		return "error", errors.New("udev policy source is nil")
	}
	mode, err := source.Lstat()
	if errors.Is(err, fs.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "error", err
	}
	if !mode.IsRegular() {
		return "error", errors.New("udev policy is not a regular file")
	}
	data, err := source.Read(udevpolicy.MaxPolicyBytes + 1)
	if err != nil {
		return "error", err
	}
	if len(data) > udevpolicy.MaxPolicyBytes {
		return "error", errors.New("udev policy exceeds size limit")
	}
	if bytes.Equal(data, candidate.Bytes) {
		return "current", nil
	}
	sum := sha256.Sum256(data)
	for _, previous := range candidate.Previous {
		if hex.EncodeToString(sum[:]) == previous {
			return "outdated", nil
		}
	}
	return "custom", nil
}

// CheckUnifiedUpdates only inspects; no authorization or apply callback runs.
// Checks and both old/new applies serialize on the same AppImage state lock.
func (s *Service) CheckUnifiedUpdates(ctx context.Context) (*UnifiedUpdateInfo, error) {
	state := s.updateState()
	if state == nil {
		return nil, errors.New("updates are unavailable")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	u := state.unified
	if u == nil {
		return nil, errors.New("updates are unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, updateOperationTimeout)
	defer cancel()
	u.checked = false
	if err := bounded.Err(); err != nil {
		return nil, err
	}
	previous := u.candidate
	var policy *udevauth.Policy
	info := UnifiedUpdateInfo{AppInstalled: state.replacementComplete}
	if state.replacementComplete {
		info.Version = u.info.Version
		info.AppAvailable = u.info.AppAvailable
		info.AppError = u.info.AppError
		info.RelaunchError = u.info.RelaunchError
	}
	if state.check != nil && !state.replacementComplete {
		candidate, err := state.check(bounded)
		state.verified = nil
		if err != nil && !errors.Is(err, update.ErrNoUpdate) && !errors.Is(err, update.ErrNoRCUpdate) {
			info.AppError = err.Error()
		}
		if err == nil && candidate != nil {
			state.verified = candidate
			info.Version = candidate.Release.Version
			info.AppAvailable = true
			if info.Version == u.info.Version {
				info.AppError = u.info.AppError
			}
		}
	}
	if err := bounded.Err(); err != nil {
		return nil, err
	}
	if u.backend != nil {
		candidate, err := u.backend.Check(bounded)
		if err != nil {
			info.PolicyState = "error"
			info.PolicyError = err.Error()
		} else if candidate != nil {
			// Detach backend-owned mutable slices before retaining an approved candidate.
			copy := *candidate
			copy.Bytes = append([]byte(nil), candidate.Bytes...)
			copy.Previous = append([]string(nil), candidate.Previous...)
			policy = &copy
		}
	}
	if info.PolicyError == "" {
		status, err := policyState(u.source, policy)
		info.PolicyState = status
		if err != nil {
			info.PolicyError = err.Error()
		}
	}
	if previous != nil && policy != nil && previous.Version == policy.Version && bytes.Equal(previous.Bytes, policy.Bytes) {
		info.PolicyInstalled = u.info.PolicyInstalled
		if info.PolicyError == "" {
			info.PolicyError = u.info.PolicyError
		}
	}
	if err := bounded.Err(); err != nil {
		return nil, err
	}
	u.candidate = policy
	u.info = info
	u.checked = true
	result := u.info
	return &result, nil
}

// ApplyUnifiedUpdate is the single explicit approval action. Writes are not
// atomic: each completed operation is retained across failure and retry.
func (s *Service) ApplyUnifiedUpdate(ctx context.Context) (*UnifiedUpdateInfo, error) {
	state := s.updateState()
	if state == nil {
		return nil, errors.New("updates are unavailable")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	u := state.unified
	if u == nil || !u.checked {
		return nil, errors.New("check for updates before applying")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, updateApplyTimeout)
	defer cancel()
	info := &u.info
	previouslyInstalled := info.PolicyInstalled
	if !info.PolicyInstalled && u.candidate != nil && (info.PolicyState == "missing" || info.PolicyState == "outdated") {
		status, err := policyState(u.source, u.candidate)
		info.PolicyState = status
		info.PolicyError = ""
		if err != nil {
			info.PolicyError = err.Error()
		} else if status == "missing" || status == "outdated" {
			if err := bounded.Err(); err != nil {
				result := *info
				return &result, err
			}
			candidate := *u.candidate
			candidate.Bytes = append([]byte(nil), candidate.Bytes...)
			candidate.Previous = append([]string(nil), candidate.Previous...)
			applied, err := u.backend.Apply(bounded, &candidate)
			info.PolicyInstalled = applied.Installed
			if err != nil {
				info.PolicyError = err.Error()
			}
		}
	}
	if err := bounded.Err(); err != nil {
		result := *info
		return &result, err
	}
	if info.AppAvailable && !state.replacementComplete {
		info.AppError = ""
		if state.apply == nil {
			info.AppError = "updates are unavailable"
		} else if err := state.apply(bounded, state.verified, true); err != nil {
			info.AppError = err.Error()
		} else {
			state.replacementComplete = true
		}
	}
	info.AppInstalled = state.replacementComplete
	if err := bounded.Err(); err != nil {
		result := *info
		return &result, err
	}
	// A partial failure stays visible; a later explicit action can relaunch, but
	// does not repeat a policy write that already succeeded before reload failed.
	if state.replacementComplete && info.AppError == "" && (info.PolicyError == "" || previouslyInstalled) {
		info.RelaunchError = ""
		if state.relaunch != nil {
			if err := state.relaunch(); err != nil {
				info.RelaunchError = err.Error()
			}
		}
	}
	result := *info
	return &result, bounded.Err()
}
