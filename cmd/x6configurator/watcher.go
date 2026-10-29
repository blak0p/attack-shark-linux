package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
	"github.com/blak0p/attack-shark-linux/internal/procwatch"
)

const (
	watcherInterval = 3 * time.Second
	procRoot        = "/proc"
)

var errSwitchRefused = errors.New("profile switch refused")

// serviceProfileSwitcher adapts the desktop service to procwatch.Switcher.
type serviceProfileSwitcher struct {
	service profileSwitcher
}

// ActiveID returns the active profile, or "" when it cannot be read.
func (a serviceProfileSwitcher) ActiveID() string {
	snapshot := a.service.GetProfiles()
	if snapshot.Error.Code != "" {
		return ""
	}
	return snapshot.ActiveID
}

func (a serviceProfileSwitcher) Switch(id string) error {
	if result := a.service.SwitchProfile(id); result.Error.Code != "" {
		return fmt.Errorf("%w: %s", errSwitchRefused, result.Error.Code)
	}
	slog.Info("process watcher switched profile", "profile_id", id)
	return nil
}

// watcherController keeps at most one watcher run alive and restarts it when
// the settings change.
type watcherController struct {
	run func(ctx context.Context, rules []procwatch.Rule)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newWatcherController(run func(ctx context.Context, rules []procwatch.Rule)) *watcherController {
	return &watcherController{run: run}
}

// Apply stops the current run, if any, and starts a new one when the settings
// are enabled and hold at least one rule. A run that was active when the
// settings changed does not restore the previous profile.
func (c *watcherController) Apply(settings configstore.WatcherSettings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
	if !settings.Enabled || len(settings.Rules) == 0 {
		return
	}
	rules := make([]procwatch.Rule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = procwatch.Rule{Executable: r.Executable, Folder: r.Folder, ProfileID: r.ProfileID}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	c.cancel, c.done = cancel, done
	go func() {
		defer close(done)
		c.run(ctx, rules)
	}()
}

// Stop ends the current run and waits for it to return.
func (c *watcherController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *watcherController) stopLocked() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	<-c.done
	c.cancel, c.done = nil, nil
}

// startProcessWatcher wires the persisted watcher settings to a running
// watcher and returns the function that stops it.
func startProcessWatcher(service *desktop.Service, store *configstore.WatcherStore) (stop func()) {
	controller := newWatcherController(func(ctx context.Context, rules []procwatch.Rule) {
		watcher := procwatch.NewWatcher(serviceProfileSwitcher{service: service}, rules)
		procwatch.Run(ctx, watcher, procRoot, watcherInterval, func(err error) {
			slog.Warn("process watcher", "error", err)
		})
	})
	service.AttachWatcherSettings(store)
	service.SetWatcherChangeHandler(controller.Apply)
	if settings, err := store.Load(); err != nil {
		slog.Warn("process watcher settings unreadable, watcher stays off", "error", err)
	} else {
		controller.Apply(settings)
	}
	return controller.Stop
}
