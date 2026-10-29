package desktop

import "sync"

const AutostartUnavailable ErrorCode = "autostart_unavailable"

// AutostartSnapshot reports whether the app starts in the tray at login. It has
// no JSON tags so the frontend sees PascalCase keys like every desktop type.
type AutostartSnapshot struct {
	Enabled bool
	Error   Error
}

// autostartComponent holds the autostart hooks of one Service. Its zero value
// is usable: nothing attached means autostart_unavailable.
type autostartComponent struct {
	mu  sync.Mutex // serialises Set and guards the hooks
	get func() bool
	set func(bool) error
}

// AttachAutostart wires the autostart hooks before publication.
func (s *Service) AttachAutostart(get func() bool, set func(bool) error) *Service {
	c := &s.autostart
	c.mu.Lock()
	c.get, c.set = get, set
	c.mu.Unlock()
	return s
}

// GetAutostart returns the current autostart state.
func (s *Service) GetAutostart() AutostartSnapshot {
	c := &s.autostart
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.get == nil {
		return AutostartSnapshot{Error: Error{Code: AutostartUnavailable}}
	}
	return AutostartSnapshot{Enabled: c.get()}
}

// SetAutostartEnabled stores the setting. On failure it reports the state that
// is still in effect together with autostart_unavailable.
func (s *Service) SetAutostartEnabled(enabled bool) AutostartSnapshot {
	c := &s.autostart
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.get == nil || c.set == nil {
		return AutostartSnapshot{Error: Error{Code: AutostartUnavailable}}
	}
	if err := c.set(enabled); err != nil {
		return AutostartSnapshot{Enabled: c.get(), Error: Error{Code: AutostartUnavailable}}
	}
	return AutostartSnapshot{Enabled: c.get()}
}
