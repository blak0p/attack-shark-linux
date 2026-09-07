## Exploration: emergency-reset-remap-confirmation

### Current State
The desktop service has two different write behaviors. `StageDPI` mutates pending state and immediately schedules `SyncCoordinator.ScheduleAt`, whose one-second expiry invokes the bound hardware apply path. `StagePollingRate` follows an equivalent automatic debounce lane. This means staging DPI is not merely local editing; it can write without a subsequent explicit confirmation.

`Service.ApplyRemap` is already an explicit, complete-draft operation: it validates through `x6.NewRemapOperation`, sends through the selected binding with `ApplyOperationBound`, advances applied state only after the ACK, and persists the acknowledged configuration. The protocol and adapter have remap-focused tests, but CodeGraph found no service-level coverage for `Service.ApplyRemap`; the frontend binding is also not covered by a test that proves the native service call path.

`EmergencyReset.Run` is a synchronous orchestration path that refreshes and requires exactly one eligible device, quiesces pending DPI and polling writes, applies DPI, polling, and remap lanes in order, and purges persisted state only after all three ACKs succeed. Existing tests exercise this orchestration with fakes, including lane failure and cleanup failure. They do not prove the real `Service` wiring, restart/reinitialization behavior, or that scheduled writes remain cancelled during and after reset. `Service.ResetToFactory` currently stages only DPI and polling through their automatic lanes and does not invoke the emergency reset path or reset remap state.

The checkout has no existing `openspec/` configuration or source specs, and the dispatcher has declared OpenSpec as authoritative with previous artifacts unavailable. This exploration therefore uses the current Go/TypeScript source and tests as the only implementation evidence.

### Affected Areas
- `internal/desktop/service.go` — `StageDPI` currently schedules an automatic physical write; `ApplyRemap` needs direct service coverage; `ResetToFactory` currently stages only two lanes.
- `internal/desktop/sync.go` — debounce expiry is the automatic DPI write boundary and must be cancellable or made confirmation-driven without weakening binding and revision checks.
- `internal/desktop/emergency_reset.go` — authoritative three-lane reset sequencing and `Quiesce` behavior need runtime-level verification.
- `internal/desktop/service_tdd_test.go` — appropriate home for service-level `ApplyRemap` success, validation, ACK/failure, persistence, and explicit-write assertions.
- `internal/desktop/emergency_reset_test.go` — existing fake-based orchestration tests should be complemented by a real-service runtime test rather than treated as reset integration coverage.
- `internal/desktop/polling_sync_tdd_test.go` — existing deterministic scheduler patterns can prove cancellation and restart behavior for the reset flow.
- `internal/protocol/x6/remap_test.go` and `internal/x6/remap_test.go` — existing protocol/operation coverage should be checked against the claimed remap scenario; add the missing scenario at the narrowest layer only if it is genuinely absent.
- `frontend/src/components/panels/ButtonRemapPanel.tsx` and `frontend/src/hooks/useDesktopWorkspace.ts` — the UI already separates draft editing from `Apply remap`; its contract should remain explicit while backend confirmation semantics are corrected.

### Approaches
1. **Make all staging local, apply on explicit commands** — remove automatic physical writes from `StageDPI` (and preserve a separate explicit apply path), keep `ApplyRemap` as the model for complete-draft writes, and have reset use one explicit, quiesced three-lane operation.
   - Pros: directly satisfies the safety rule; makes UI confirmation semantics honest; gives reset one authoritative lifecycle; easiest to prove with no-write-before-confirmation tests.
   - Cons: changes existing DPI/polling UX and requires careful compatibility handling for callers that currently rely on debounce application.
   - Effort: High

2. **Retain debounce but add a confirmation token/arm step** — staging schedules only an armed operation after a separate explicit confirmation signal, with reset invalidating all arms and revisions.
   - Pros: preserves debounce/coalescing behavior after confirmation; smaller change to scheduler mechanics.
   - Cons: more state and race cases; a stale or accidentally retained arm can still violate the safety invariant; reset and UI contracts become harder to reason about.
   - Effort: High

3. **Test-only correction without changing write semantics** — add the missing service/runtime tests and document the automatic stage write as intended.
   - Pros: lowest implementation effort.
   - Cons: explicitly fails the hardware-safety requirement that writes happen only on explicit user demand; it cannot be recommended.
   - Effort: Low

### Recommendation
Choose Approach 1. Define an explicit confirmation boundary for every physical configuration write, then test that boundary at the service level. Preserve the existing pure X6 encoder and typed operation adapters; keep hardware access in `mouse`/`hidlinux` behind `ApplyOperationBound`; and keep reset sequencing in `EmergencyReset` with `Quiesce` before writes and purge after all ACKs.

The proposal should make the four verification corrections concrete:
1. Identify the exact missing claimed remap scenario and add a real assertion at the service boundary, not only protocol-byte tests. It should prove validation, selected-binding targeting, ACK-gated applied state, and persistence behavior.
2. Add Go coverage for `Service.ApplyRemap`, including no selection, invalid complete draft, command failure/no applied advance, successful ACK, and persistence failure/retry state.
3. Add a runtime reset test using the real service/coordinators plus deterministic scheduler and command/persistence fakes. Prove reset cancels pending work, restarts cleanly after cancellation or reconnect, applies all three lanes in order, and does not purge on any failed ACK.
4. Correct `StageDPI` so staging cannot reach `SyncCoordinator` hardware application without explicit confirmation. Add a regression test that stages, advances the scheduler, and observes zero command writes; then explicitly applies and observes one bound, ACK-checked write. Apply the same invariant to any reset-triggered lane so reset is not undermined by a late callback.

Keep tests table-driven where cases vary, use deterministic scheduler seams, and assert side effects rather than implementation details. Do not add hardware-dependent tests; hidraw tests must remain passive or use bounded command fakes, with no interface claiming, detaching, rebinding, or root assumption.

### Risks
- Removing automatic staging writes may require a coordinated frontend/service contract change so users still have a clear Apply action for DPI and polling.
- Reset spans three independent state lanes; a late callback, stale binding, or revision race could write after quiescence unless every callback checks cancellation and revision.
- `Service.ResetToFactory` and `EmergencyReset.Run` currently represent different reset semantics; proposal/design must choose one authoritative public behavior rather than leave two partially overlapping paths.
- Existing generated Wails bindings and built frontend assets are tracked in the working tree; implementation must regenerate or update only through the repository's established binding workflow, without touching them during exploration.
- The working tree already contains unrelated staged and untracked changes; downstream implementation must isolate its diff and avoid treating those changes as part of this change.

### Ready for Proposal
Yes. The proposal should establish explicit confirmation as the non-negotiable safety invariant, define whether `ResetToFactory` delegates to the three-lane emergency reset or remains a distinct staged preview, and enumerate the service/runtime tests that close each verification blocker. No source or test edits were made during exploration.
