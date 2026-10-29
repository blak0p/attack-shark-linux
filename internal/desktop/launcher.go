package desktop

import (
	"path/filepath"
	"sync"
)

const LauncherUnavailable ErrorCode = "launcher_unavailable"

// LauncherPathSnapshot carries the absolute path of the launcher binary. It
// has no JSON tags so the frontend sees PascalCase keys like every desktop type.
type LauncherPathSnapshot struct {
	Path  string
	Error Error
}

// launcherComponent holds the launcher path lookup of one Service. Its zero
// value is usable: nothing attached means launcher_unavailable.
type launcherComponent struct {
	mu     sync.Mutex // guards lookup
	lookup func() (string, error)
}

// AttachLauncherPath wires the launcher path lookup before publication.
func (s *Service) AttachLauncherPath(lookup func() (string, error)) *Service {
	c := &s.launcher
	c.mu.Lock()
	c.lookup = lookup
	c.mu.Unlock()
	return s
}

// GetLauncherPath returns the absolute launcher path, or launcher_unavailable
// when no lookup is attached, it fails or it yields a non-absolute path.
func (s *Service) GetLauncherPath() LauncherPathSnapshot {
	c := &s.launcher
	c.mu.Lock()
	lookup := c.lookup
	c.mu.Unlock()
	if lookup == nil {
		return LauncherPathSnapshot{Error: Error{Code: LauncherUnavailable}}
	}
	path, err := lookup()
	if err != nil || !filepath.IsAbs(path) {
		return LauncherPathSnapshot{Error: Error{Code: LauncherUnavailable}}
	}
	return LauncherPathSnapshot{Path: path}
}
