package desktop

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

// UpdateInfo is the display-only update contract exported to Wails bindings.
type UpdateInfo struct{ Version string }

const (
	updateOperationTimeout = 15 * time.Second
	updateApplyTimeout     = 3 * time.Minute
)

type updateState struct {
	mu                  sync.Mutex
	updater             *update.Updater
	verified            *update.VerifiedUpdate
	check               func(context.Context) (*update.VerifiedUpdate, error)
	apply               func(context.Context, *update.VerifiedUpdate, bool) error
	relaunch            func() error
	replacementComplete bool
}

// ConfigureUpdater composes private updater state into a desktop service before
// Wails registers it. It is deliberately not a Service method, so neither the
// updater nor its relaunch callback can cross the renderer boundary.
func ConfigureUpdater(service *Service, updater *update.Updater, relaunch func() error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	state := &updateState{updater: updater, relaunch: relaunch}
	if updater != nil {
		state.check = updater.Check
		state.apply = updater.Apply
	}
	service.update = state
}

// CheckForUpdate verifies availability only. It never downloads, replaces, or relaunches.
func (s *Service) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	s.mu.Lock()
	state := s.update
	s.mu.Unlock()
	if state == nil || state.check == nil {
		return nil, nil
	}
	bounded, cancel := context.WithTimeout(ctx, updateOperationTimeout)
	defer cancel()
	verified, err := state.check(bounded)
	if errors.Is(err, update.ErrNoUpdate) || errors.Is(err, update.ErrNoRCUpdate) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	state.mu.Lock()
	state.verified = verified
	state.mu.Unlock()
	return &UpdateInfo{Version: verified.Release.Version}, nil
}

// ApplyVerifiedUpdate requires the prior explicit frontend approval action.
func (s *Service) ApplyVerifiedUpdate(ctx context.Context) error {
	s.mu.Lock()
	state := s.update
	s.mu.Unlock()
	if state == nil {
		return errors.New("updates are unavailable")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.replacementComplete {
		if state.apply == nil {
			return errors.New("updates are unavailable")
		}
		bounded, cancel := context.WithTimeout(ctx, updateApplyTimeout)
		defer cancel()
		if err := state.apply(bounded, state.verified, true); err != nil {
			return err
		}
		state.replacementComplete = true
	}
	if state.relaunch != nil {
		return state.relaunch()
	}
	return nil
}

// RecoverUpdates completes an interrupted installed-AppImage replacement before
// Wails registers the service and before update checks begin.
func RecoverUpdates(service *Service) error {
	service.mu.Lock()
	state := service.update
	service.mu.Unlock()
	if state == nil || state.updater == nil {
		return nil
	}
	return state.updater.Recover()
}
