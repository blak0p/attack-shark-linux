package configstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ErrInvalidWatcherSettings reports a corrupt stored file or rejected input.
var ErrInvalidWatcherSettings = errors.New("invalid watcher settings")

const (
	maxWatcherRules      = 64
	maxWatcherExecutable = 255
)

var watcherProfileID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// WatcherRule maps a bare executable name to a profile ID.
type WatcherRule struct {
	Executable string `json:"executable"`
	ProfileID  string `json:"profile_id"`
}

// WatcherSettings is the opt-in process watcher configuration.
type WatcherSettings struct {
	Enabled bool          `json:"enabled"`
	Rules   []WatcherRule `json:"rules"`
}

// WatcherStore persists WatcherSettings in one file. Share one instance per
// path; separate instances or processes are not coordinated.
type WatcherStore struct {
	mu   sync.Mutex
	path string
}

func NewWatcherStore(path string) *WatcherStore { return &WatcherStore{path: path} }

// Load returns defaults for a missing file and ErrInvalidWatcherSettings for a
// corrupt or invalid one. It never modifies the file.
func (w *WatcherStore) Load() (WatcherSettings, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	contents, err := os.ReadFile(w.path)
	if errors.Is(err, os.ErrNotExist) {
		return WatcherSettings{Rules: []WatcherRule{}}, nil
	}
	if err != nil {
		return WatcherSettings{}, fmt.Errorf("read watcher settings: %w", err)
	}
	var settings WatcherSettings
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		return WatcherSettings{}, fmt.Errorf("%w: %v", ErrInvalidWatcherSettings, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return WatcherSettings{}, fmt.Errorf("%w: trailing data", ErrInvalidWatcherSettings)
	}
	normalized, err := normalizeWatcherSettings(settings)
	if err != nil {
		return WatcherSettings{}, err
	}
	return normalized, nil
}

// Save validates first, then replaces the file atomically (mode 0600). A
// rejected or failed save leaves the stored file untouched.
func (w *WatcherStore) Save(settings WatcherSettings) error {
	normalized, err := normalizeWatcherSettings(settings)
	if err != nil {
		return err
	}
	contents, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	directory := filepath.Dir(w.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create watcher directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".watcher-*.tmp")
	if err != nil {
		return fmt.Errorf("create watcher temporary file: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write watcher settings: %w", err)
	}
	if err := os.Rename(file.Name(), w.path); err != nil {
		return fmt.Errorf("replace watcher settings: %w", err)
	}
	return nil
}

// normalizeWatcherSettings trims executables, validates every rule and returns
// a copy that shares no memory with the input.
func normalizeWatcherSettings(in WatcherSettings) (WatcherSettings, error) {
	if len(in.Rules) > maxWatcherRules {
		return WatcherSettings{}, fmt.Errorf("%w: more than %d rules", ErrInvalidWatcherSettings, maxWatcherRules)
	}
	out := WatcherSettings{Enabled: in.Enabled, Rules: make([]WatcherRule, 0, len(in.Rules))}
	seen := make(map[string]bool, len(in.Rules))
	for i, rule := range in.Rules {
		exe := strings.TrimSpace(rule.Executable)
		switch {
		case exe == "":
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable is blank", ErrInvalidWatcherSettings, i)
		case len(exe) > maxWatcherExecutable:
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable exceeds %d bytes", ErrInvalidWatcherSettings, i, maxWatcherExecutable)
		case strings.ContainsAny(exe, "/\\\x00"):
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable must be a bare name", ErrInvalidWatcherSettings, i)
		case seen[exe]:
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: duplicate executable %q", ErrInvalidWatcherSettings, i, exe)
		case !watcherProfileID.MatchString(rule.ProfileID):
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: profile ID must be 32 lowercase hex characters", ErrInvalidWatcherSettings, i)
		}
		seen[exe] = true
		out.Rules = append(out.Rules, WatcherRule{Executable: exe, ProfileID: rule.ProfileID})
	}
	return out, nil
}
