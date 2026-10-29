package desktop

import (
	"errors"
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
)

const (
	InvalidWatcherSettings ErrorCode = "invalid_watcher_settings"
	WatcherUnavailable     ErrorCode = "watcher_unavailable"
)

// WatcherRule is the desktop view of a configstore rule. It has no JSON tags
// so the frontend sees PascalCase keys like every other desktop type; the
// on-disk names live on configstore.WatcherRule.
type WatcherRule struct {
	Executable string
	Folder     string
	Name       string
	ProfileID  string
}

func rulesFromStore(in []configstore.WatcherRule) []WatcherRule {
	out := make([]WatcherRule, 0, len(in))
	for _, r := range in {
		out = append(out, WatcherRule{Executable: r.Executable, Folder: r.Folder, Name: r.Name, ProfileID: r.ProfileID})
	}
	return out
}

func rulesToStore(in []WatcherRule) []configstore.WatcherRule {
	out := make([]configstore.WatcherRule, 0, len(in))
	for _, r := range in {
		out = append(out, configstore.WatcherRule{Executable: r.Executable, Folder: r.Folder, Name: r.Name, ProfileID: r.ProfileID})
	}
	return out
}

// WatcherSettingsSnapshot carries the stored settings plus an optional error.
type WatcherSettingsSnapshot struct {
	Enabled bool
	Rules   []WatcherRule
	Error   Error
}

// watcherComponent holds the watcher settings state of one Service. Its zero
// value is usable: no store attached means watcher_unavailable.
type watcherComponent struct {
	mu      sync.Mutex // guards store and handler
	setMu   sync.Mutex // serializes saves so the handler sees them in order
	store   *configstore.WatcherStore
	handler func(configstore.WatcherSettings)
}

func (c *watcherComponent) current() (*configstore.WatcherStore, func(configstore.WatcherSettings)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.store, c.handler
}

// AttachWatcherSettings wires the single watcher settings store before publication.
func (s *Service) AttachWatcherSettings(store *configstore.WatcherStore) *Service {
	c := &s.watcherSettings
	c.mu.Lock()
	c.store = store
	c.mu.Unlock()
	return s
}

// SetWatcherChangeHandler registers a callback invoked after every successful
// SetWatcherSettings, never on errors or reads. Pass nil to clear it. The
// handler may call GetWatcherSettings but must not call SetWatcherSettings.
func (s *Service) SetWatcherChangeHandler(handler func(configstore.WatcherSettings)) {
	c := &s.watcherSettings
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()
}

func watcherError(err error) ErrorCode {
	if errors.Is(err, configstore.ErrInvalidWatcherSettings) {
		return InvalidWatcherSettings
	}
	return PersistenceFailed
}

func watcherSnapshot(settings configstore.WatcherSettings, code ErrorCode) WatcherSettingsSnapshot {
	return WatcherSettingsSnapshot{Enabled: settings.Enabled, Rules: rulesFromStore(settings.Rules), Error: Error{Code: code}}
}

// GetWatcherSettings reads the stored settings; a missing file yields defaults.
func (s *Service) GetWatcherSettings() WatcherSettingsSnapshot {
	store, _ := s.watcherSettings.current()
	if store == nil {
		return watcherSnapshot(configstore.WatcherSettings{}, WatcherUnavailable)
	}
	settings, err := store.Load()
	if err != nil {
		return watcherSnapshot(configstore.WatcherSettings{}, watcherError(err))
	}
	return watcherSnapshot(settings, "")
}

// SetWatcherSettings validates and persists the settings, then notifies the
// change handler once. On error the stored settings are returned unchanged.
func (s *Service) SetWatcherSettings(enabled bool, rules []WatcherRule) WatcherSettingsSnapshot {
	c := &s.watcherSettings
	c.setMu.Lock()
	defer c.setMu.Unlock()
	store, handler := c.current()
	if store == nil {
		return watcherSnapshot(configstore.WatcherSettings{}, WatcherUnavailable)
	}
	if err := store.Save(configstore.WatcherSettings{Enabled: enabled, Rules: rulesToStore(rules)}); err != nil {
		stored, _ := store.Load()
		return watcherSnapshot(stored, watcherError(err))
	}
	saved, err := store.Load()
	if err != nil {
		return watcherSnapshot(configstore.WatcherSettings{}, watcherError(err))
	}
	if handler != nil {
		handler(saved)
	}
	return watcherSnapshot(saved, "")
}
