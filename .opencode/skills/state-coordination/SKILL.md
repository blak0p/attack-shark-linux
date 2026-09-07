---
name: state-coordination
description: "Trigger: adding shared mutable state, per-device state, debounced auto-sync, emitting events from state, injecting test schedulers, protecting writes with revision stamps. Follow the concurrency discipline."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when touching `internal/desktop` or `internal/mouse` state structs, the sync
coordinator, event emission under lock, or test seams for time-based behavior.

## Hard Rules
- Each per-device state holds a value `sync.Mutex` plus a separate `applyMu` for
  the write path (`internal/desktop/service.go:135`, `internal/mouse/service.go:65`).
  Mutexes are declared **by value** and never copied.
- **Lock order is invariant: take the state lock first, then the service lock.**
  Never reverse it. Release the state lock before acquiring the service lock when
  both are needed (`internal/desktop/service.go:436`, `internal/mouse/service.go:222`).
- Debounced auto-sync lives in `SyncCoordinator` with a `syncDebounceDelay`
  (`internal/desktop/sync.go:10`). Scheduling restarts the debounce per binding.
- Time is injected. Production uses `realSyncScheduler` (wraps `time.AfterFunc`);
  tests inject a `SyncScheduler` seam via `attachAutomaticSave`
  (`internal/desktop/service.go:235`). Never call `time.Sleep` to fake timing.
- Writes are guarded by a **revision stamp**: if `state.revision != revision` when
  the deferred apply runs, the work is discarded (`mouse.ErrRevisionChanged`).
  This makes concurrent Stage/Apply safe without blocking the UI.
- Emit events **after** releasing locks, or pass a copy. `emitConfiguration` reads
  the sink under the service lock, then emits without holding the state lock.
- Select-then-act must revalidate the binding: `bindingCurrent`/`ApplyBound`
  refuse work if the device changed (`internal/desktop/service.go:849`).

## Frontier bar
- A frontier model never shares a locked struct across a boundary, never reverses
  lock order, and never fakes time with sleeps.
- It treats the revision counter as the source of truth for "is this write still
  wanted" and cancels stale debounced work on selection changes
  (`cancelSync`/`cancelPollingSync`).
- It injects schedulers/clocks for determinism rather than reaching for wall time
  in tests.

## Decision Gates
| You need to… | Pattern |
| Add per-device state | value `sync.Mutex` + `applyMu`, follow lock order |
| Debounce a write | `SyncCoordinator.ScheduleAt(binding, revision, config)` |
| Make timing deterministic in tests | inject `SyncScheduler` via `attachAutomaticSave` |
| Drop a stale write | compare `state.revision` to the captured `revision` |
| Emit a live update | copy under lock, emit after unlock |

## Execution Steps
1. Add the field to the state struct; keep mutexes by value.
2. Read/write under `state.mu`; for writes also take `applyMu`.
3. Stamp `revision++` on every staged change; capture it for the deferred apply.
4. Schedule or cancel debounced work through the coordinator; inject a scheduler
   in tests.
5. Emit after releasing locks; verify `go vet ./...` and the race detector
   (`go test -race ./internal/desktop/...`).

## Output Contract
Report the state struct changed, lock-order compliance, revision/scheduler
decisions, and confirm tests pass under `-race`.

## References
- [references/state-coordination.md](references/state-coordination.md) — real concurrency excerpts.
