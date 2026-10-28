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
	Tray    bool
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
	extra   []func(configstore.WatcherSettings) // added listeners, kept apart from handler
}

// current returns the store and one function that notifies the handler and
// every added listener, or nil when there are none.
func (c *watcherComponent) current() (*configstore.WatcherStore, func(configstore.WatcherSettings)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	handlers := make([]func(configstore.WatcherSettings), 0, 1+len(c.extra))
	if c.handler != nil {
		handlers = append(handlers, c.handler)
	}
	handlers = append(handlers, c.extra...)
	if len(handlers) == 0 {
		return c.store, nil
	}
	return c.store, func(settings configstore.WatcherSettings) {
		for _, handler := range handlers {
			handler(settings)
		}
	}
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

// AddWatcherChangeHandler registers an extra listener notified after every
// successful change, in addition to the one set by SetWatcherChangeHandler.
// Added listeners are never replaced or cleared by SetWatcherChangeHandler and
// follow the same rules: they must not call a setter.
func (s *Service) AddWatcherChangeHandler(handler func(configstore.WatcherSettings)) {
	if handler == nil {
		return
	}
	c := &s.watcherSettings
	c.mu.Lock()
	c.extra = append(c.extra, handler)
	c.mu.Unlock()
}

func watcherError(err error) ErrorCode {
	if errors.Is(err, configstore.ErrInvalidWatcherSettings) {
		return InvalidWatcherSettings
	}
	return PersistenceFailed
}

func watcherSnapshot(settings configstore.WatcherSettings, code ErrorCode) WatcherSettingsSnapshot {
	return WatcherSettingsSnapshot{Enabled: settings.Enabled, Tray: settings.Tray, Rules: rulesFromStore(settings.Rules), Error: Error{Code: code}}
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
	// The tray flag is not an argument here: keep whatever is stored. An
	// unreadable file has no flag to keep, so it is rewritten with Tray off.
	stored, _ := store.Load()
	if err := store.Save(configstore.WatcherSettings{Enabled: enabled, Tray: stored.Tray, Rules: rulesToStore(rules)}); err != nil {
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

// SetTrayEnabled stores only the tray flag, keeping Enabled and Rules, and
// notifies the change handler once on success. On error nothing is saved and
// the handler is not called.
func (s *Service) SetTrayEnabled(enabled bool) WatcherSettingsSnapshot {
	c := &s.watcherSettings
	c.setMu.Lock()
	defer c.setMu.Unlock()
	store, handler := c.current()
	if store == nil {
		return watcherSnapshot(configstore.WatcherSettings{}, WatcherUnavailable)
	}
	stored, err := store.Load()
	if err != nil {
		return watcherSnapshot(configstore.WatcherSettings{}, watcherError(err))
	}
	stored.Tray = enabled
	if err := store.Save(stored); err != nil {
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
