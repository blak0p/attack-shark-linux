# state-coordination — concurrency references (real repo excerpts)

## 1. Per-device state: value mutex + apply mutex

`internal/desktop/service.go:135` and `internal/mouse/service.go:65` declare
mutexes by value, one for general state and one for the apply path:

```go
// internal/desktop/service.go:135
type deviceState struct {
	mu                         sync.Mutex
	applyMu                    sync.Mutex
	applied, pending, factory  x6.DPIConfig
	...
}
```

## 2. Invariant lock order: state lock, then service lock

`internal/desktop/service.go:436` takes `state.mu` (already held by the caller)
then `s.mu` only briefly to read the sink:

```go
func (s *Service) foldStatusEvent(state *deviceState, event StatusEvent, eventName string) {
	state.mu.Lock()
	state.connection = x6.Connection(event.Connection)
	... // mutate state
	state.mu.Unlock()
	s.mu.Lock()
	sink := s.events
	s.mu.Unlock()
	if sink != nil {
		sink.Emit(eventName, event)
	}
}
```

In `mouse`, the apply path locks `applyMu` then `mu`
(`internal/mouse/service.go:230`): never reversed.

## 3. Debounce via `SyncCoordinator`, time injected

`internal/desktop/sync.go:10` fixes the delay; `realSyncScheduler` is the only
wall-clock user:

```go
const syncDebounceDelay = time.Second

type SyncScheduler interface {
	After(time.Duration, func()) SyncCancel
}

type realSyncScheduler struct{}

func (realSyncScheduler) After(delay time.Duration, f func()) SyncCancel {
	timer := time.AfterFunc(delay, f)
	return func() { timer.Stop() }
}
```

Tests replace it through a seam (`internal/desktop/service.go:235`):

```go
func (s *Service) attachAutomaticSave(scheduler SyncScheduler) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sync = NewSyncCoordinator(scheduler, s.bindingCurrent, s.applyBound)
	return s
}
```

## 4. Revision stamps drop stale writes

`internal/desktop/service.go:872` — the deferred apply bails if the state moved
on:

```go
func (s *Service) applyBound(binding Binding, revision uint64, config x6.DPIConfig) error {
	if !s.bindingCurrent(binding) {
		return mouse.ErrStaleBinding
	}
	state := s.currentState()
	defer s.emitConfiguration(binding, state)
	state.mu.Lock()
	if state.revision != revision {
		state.mu.Unlock()
		return mouse.ErrRevisionChanged
	}
	state.mu.Unlock()
	...
}
```

## 5. Cancel stale debounce on selection change

`internal/desktop/service.go:854` cancels pending sync when the selected device
changes, so a queued write never lands on the wrong mouse:

```go
func (s *Service) cancelSync(binding Binding) {
	s.mu.Lock()
	sync := s.sync
	s.mu.Unlock()
	if sync != nil {
		sync.Cancel(binding)
	}
}
```
