package desktop

import (
	"sync"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/x6"
)

const syncDebounceDelay = time.Second

// SyncCancel prevents a scheduled debounce callback from running.
type SyncCancel func()

// SyncScheduler makes debounce timing deterministic in coordinator tests.
type SyncScheduler interface {
	After(time.Duration, func()) SyncCancel
}

// BoundSync applies one complete configuration to its captured binding.
// Later work supplies the binding validation and acknowledgement boundary.
type BoundSync func(Binding, uint64, x6.DPIConfig) error

// BindingValidator confirms that a captured binding is still current before I/O.
type BindingValidator func(Binding) bool

// SyncCoordinator keeps debounced desired configurations independent per binding.
type SyncCoordinator struct {
	scheduler SyncScheduler
	valid     BindingValidator
	apply     BoundSync

	mu     sync.Mutex
	states map[Binding]*syncState
}

type syncState struct {
	revision uint64
	desired  x6.DPIConfig
	cancel   SyncCancel
}

func NewSyncCoordinator(scheduler SyncScheduler, valid BindingValidator, apply BoundSync) *SyncCoordinator {
	return &SyncCoordinator{scheduler: scheduler, valid: valid, apply: apply, states: make(map[Binding]*syncState)}
}

// Schedule replaces a binding's desired configuration and restarts its debounce.
// Serial-less bindings remain memory-only; persistence is excluded by the caller.
func (c *SyncCoordinator) Schedule(binding Binding, config x6.DPIConfig) (uint64, error) {
	return c.schedule(binding, 0, config)
}

// ScheduleAt keeps a binding's debounce revision aligned with desktop state.
func (c *SyncCoordinator) ScheduleAt(binding Binding, revision uint64, config x6.DPIConfig) error {
	_, err := c.schedule(binding, revision, config)
	return err
}

func (c *SyncCoordinator) schedule(binding Binding, revision uint64, config x6.DPIConfig) (uint64, error) {
	c.mu.Lock()
	state := c.states[binding]
	if state == nil {
		state = &syncState{}
		c.states[binding] = state
	}
	if state.cancel != nil {
		state.cancel()
	}
	if revision == 0 {
		state.revision++
	} else {
		state.revision = revision
	}
	state.desired = config
	revision = state.revision
	state.cancel = c.scheduler.After(syncDebounceDelay, func() { c.expire(binding, revision) })
	c.mu.Unlock()
	return revision, nil
}

// Cancel discards unsent work for a binding when its lifecycle changes.
func (c *SyncCoordinator) Cancel(binding Binding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.states[binding]; state != nil && state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
}

func (c *SyncCoordinator) expire(binding Binding, revision uint64) {
	c.mu.Lock()
	state := c.states[binding]
	if state == nil || state.revision != revision {
		c.mu.Unlock()
		return
	}
	config := state.desired
	state.cancel = nil
	c.mu.Unlock()

	if c.valid != nil && !c.valid(binding) {
		return
	}
	if c.apply != nil {
		_ = c.apply(binding, revision, config)
	}
}
