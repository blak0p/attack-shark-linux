package desktop

import (
	"sync"

	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type BoundPollingSync func(Binding, uint64, x6.PollingRate) error

// PollingSyncCoordinator independently coalesces polling edits per immutable binding.
type PollingSyncCoordinator struct {
	scheduler SyncScheduler
	valid     BindingValidator
	apply     BoundPollingSync
	mu        sync.Mutex
	states    map[Binding]*pollingSyncState
}

type pollingSyncState struct {
	revision uint64
	rate     x6.PollingRate
	cancel   SyncCancel
}

func NewPollingSyncCoordinator(scheduler SyncScheduler, valid BindingValidator, apply BoundPollingSync) *PollingSyncCoordinator {
	return &PollingSyncCoordinator{scheduler: scheduler, valid: valid, apply: apply, states: make(map[Binding]*pollingSyncState)}
}

func (c *PollingSyncCoordinator) ScheduleAt(binding Binding, revision uint64, rate x6.PollingRate) error {
	c.mu.Lock()
	state := c.states[binding]
	if state == nil {
		state = &pollingSyncState{}
		c.states[binding] = state
	}
	if state.cancel != nil {
		state.cancel()
	}
	state.revision, state.rate = revision, rate
	state.cancel = c.scheduler.After(syncDebounceDelay, func() { c.expire(binding, revision) })
	c.mu.Unlock()
	return nil
}

func (c *PollingSyncCoordinator) Cancel(binding Binding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.states[binding]; state != nil && state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
}

func (c *PollingSyncCoordinator) expire(binding Binding, revision uint64) {
	c.mu.Lock()
	state := c.states[binding]
	if state == nil || state.revision != revision {
		c.mu.Unlock()
		return
	}
	rate := state.rate
	state.cancel = nil
	c.mu.Unlock()
	if c.valid != nil && !c.valid(binding) {
		return
	}
	if c.apply != nil {
		_ = c.apply(binding, revision, rate)
	}
}
