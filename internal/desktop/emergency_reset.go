package desktop

import (
	"context"
	"fmt"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
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
type StatePurger interface{ PurgeAll() error }

type resetTarget interface {
	Refresh(context.Context) ([]mouse.Device, error)
	Selection() (mouse.Binding, bool)
	ApplyOperationBound(context.Context, mouse.Binding, mouse.CommandOperation, any) error
}

// EmergencyReset performs the documented reset lanes synchronously. Persistence
// is deliberately unreachable until every physical ACK has succeeded.
type EmergencyReset struct {
	target  resetTarget
	pending PendingWrites
	purger  StatePurger
}

func NewEmergencyReset(target resetTarget, pending PendingWrites, purger StatePurger) *EmergencyReset {
	return &EmergencyReset{target: target, pending: pending, purger: purger}
}

func (r *EmergencyReset) Run(ctx context.Context) (ResetResult, error) {
	result := ResetResult{Lanes: []ResetLaneResult{{Lane: "dpi", State: "not_attempted"}, {Lane: "polling", State: "not_attempted"}, {Lane: "remap", State: "not_attempted"}}, Cleanup: ResetLaneResult{Lane: "cleanup", State: "not_attempted"}}
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
		{x6.NewDPIOperation(), x6.DocumentedResetDPIConfig()},
		{x6.NewPollingOperation(), x6.PollingRate1000},
		{x6.NewRemapOperation(), x6.DefaultRemapConfig()},
	}
	for index, lane := range lanes {
		if err := r.target.ApplyOperationBound(ctx, binding, lane.operation, lane.value); err != nil {
			result.Lanes[index] = ResetLaneResult{Lane: result.Lanes[index].Lane, State: "failed", Code: errorCode(err, false)}
			wrapped := fmt.Errorf("reset %s: %w", result.Lanes[index].Lane, err)
			return resetFailure(result, wrapped), wrapped
		}
		result.Lanes[index] = ResetLaneResult{Lane: result.Lanes[index].Lane, State: "success"}
	}
	if r.purger == nil {
		wrapped := fmt.Errorf("purge reset state: no purger")
		return resetFailure(result, wrapped), wrapped
	}
	if err := r.purger.PurgeAll(); err != nil {
		result.Cleanup = ResetLaneResult{Lane: "cleanup", State: "failed", Code: PersistenceFailed}
		wrapped := fmt.Errorf("purge reset state: %w", err)
		result.Error = Error{Code: PersistenceFailed}
		result.RetryAvailable = true
		return result, wrapped
	}
	result.Cleanup = ResetLaneResult{Lane: "cleanup", State: "success"}
	return result, nil
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
