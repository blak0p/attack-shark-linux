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
	maxWatcherName       = 255
	maxWatcherFolder     = 4096
)

var watcherProfileID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// WatcherRule maps either a bare executable name or an install folder to a
// profile ID. Exactly one of Executable and Folder is set. Name is the display
// name; folder rules default it to the folder base name.
type WatcherRule struct {
	Executable string `json:"executable,omitempty"`
	Folder     string `json:"folder,omitempty"`
	Name       string `json:"name,omitempty"`
	ProfileID  string `json:"profile_id"`
}

// WatcherSettings is the opt-in process watcher configuration.
type WatcherSettings struct {
	Enabled bool          `json:"enabled"`
	Tray    bool          `json:"tray,omitempty"`
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

// normalizeWatcherSettings trims and cleans every rule, validates it and
// returns a copy that shares no memory with the input.
func normalizeWatcherSettings(in WatcherSettings) (WatcherSettings, error) {
	if len(in.Rules) > maxWatcherRules {
		return WatcherSettings{}, fmt.Errorf("%w: more than %d rules", ErrInvalidWatcherSettings, maxWatcherRules)
	}
	out := WatcherSettings{Enabled: in.Enabled, Tray: in.Tray, Rules: make([]WatcherRule, 0, len(in.Rules))}
	seenExecutable := make(map[string]bool, len(in.Rules))
	seenFolder := make(map[string]bool, len(in.Rules))
	for i, rule := range in.Rules {
		exe := strings.TrimSpace(rule.Executable)
		folder := strings.TrimSpace(rule.Folder)
		name := strings.TrimSpace(rule.Name)
		switch {
		case exe != "" && folder != "":
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: set either executable or folder, not both", ErrInvalidWatcherSettings, i)
		case folder != "":
			switch {
			case len(folder) > maxWatcherFolder:
				return WatcherSettings{}, fmt.Errorf("%w: rule %d: folder exceeds %d bytes", ErrInvalidWatcherSettings, i, maxWatcherFolder)
			case strings.Contains(folder, "\x00"):
				return WatcherSettings{}, fmt.Errorf("%w: rule %d: folder contains NUL", ErrInvalidWatcherSettings, i)
			case !filepath.IsAbs(folder):
				return WatcherSettings{}, fmt.Errorf("%w: rule %d: folder must be an absolute path", ErrInvalidWatcherSettings, i)
			}
			folder = filepath.Clean(folder)
			if seenFolder[folder] {
				return WatcherSettings{}, fmt.Errorf("%w: rule %d: duplicate folder %q", ErrInvalidWatcherSettings, i, folder)
			}
			seenFolder[folder] = true
			if name == "" {
				name = filepath.Base(folder)
			}
		case exe == "":
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable is blank", ErrInvalidWatcherSettings, i)
		case len(exe) > maxWatcherExecutable:
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable exceeds %d bytes", ErrInvalidWatcherSettings, i, maxWatcherExecutable)
		case strings.ContainsAny(exe, "/\\\x00"):
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: executable must be a bare name", ErrInvalidWatcherSettings, i)
		case seenExecutable[exe]:
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: duplicate executable %q", ErrInvalidWatcherSettings, i, exe)
		default:
			seenExecutable[exe] = true
		}
		switch {
		case len(name) > maxWatcherName:
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: name exceeds %d bytes", ErrInvalidWatcherSettings, i, maxWatcherName)
		case strings.Contains(name, "\x00"):
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: name contains NUL", ErrInvalidWatcherSettings, i)
		case !watcherProfileID.MatchString(rule.ProfileID):
			return WatcherSettings{}, fmt.Errorf("%w: rule %d: profile ID must be 32 lowercase hex characters", ErrInvalidWatcherSettings, i)
		}
		out.Rules = append(out.Rules, WatcherRule{Executable: exe, Folder: folder, Name: name, ProfileID: rule.ProfileID})
	}
	return out, nil
}
