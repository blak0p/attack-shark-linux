# Design: Emergency Reset and Remap Confirmation

## Technical Approach

Make `desktop.Service` the explicit user-command boundary while retaining pure X6 operations and `mouse.TargetedService.ApplyOperationBound` as the validated HID/ACK boundary. DPI and polling staging will mutate only pending state; remap drafting remains React-local until `ApplyRemap`. `ResetToFactory` will delegate to the same `EmergencyReset.Run` used by the headless command. No listener, debounce expiry, reconnect, or retry may authorize hardware I/O.

## Architecture Decisions

| Decision | Alternatives | Rationale |
|---|---|---|
| Add explicit `ApplyDPI`/`ApplyPollingRate`; keep `ApplyRemap(config)` explicit | Preserve debounce writes | Separates editing from physical writes and closes the implicit `StageDPI`/polling path. |
| Inject a local `resetRunner` into `Service`; `ResetToFactory(ctx)` only delegates and maps its result | Duplicate reset logic in the Wails method | One ordered DPI → 1000 Hz polling → default-remap implementation serves GUI and CLI. |
| Serialize explicit applies and reset with one service operation mutex; never acquire it while holding state/service locks | Independent lane mutexes | Prevents another confirmed write interleaving the three reset lanes. Existing state-lock-before-service-lock order remains unchanged; events emit after unlock. |
| Reconcile in-memory defaults only after `Run` and purge succeed | Update per ACK | Any ACK, cancellation, binding, or cleanup failure preserves pending/applied/retry recovery state. |
| Keep generated Wails bindings generated | Hand-edit TypeScript bindings | Maintains Wails ownership and a single typed facade. |

## Data Flow

    UI edit → local pending state (zero I/O)
    Apply confirmation → Service → selected Binding → ApplyOperationBound → HID ACK
                               └─ ACK → applied state → persistence/retry state

    Reset confirmation → Service.ResetToFactory → EmergencyReset.Run
      → quiesce/invalidate → DPI ACK → polling ACK → remap ACK → transactional purge
      → success-only in-memory reconciliation/events

Each operation carries `context.Context`, revalidates the exact binding, sends one bounded feature report, and never claims/detaches a USB interface or assumes root.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/desktop/service.go` | Modify | Remove scheduling from DPI/polling staging; add explicit polling apply, remap context, reset injection/delegation, operation serialization, ACK/persistence recovery, and success-only reset reconciliation. |
| `internal/desktop/emergency_reset.go` | Modify | Keep the authoritative three-lane sequence; strengthen cancellation/result mapping and quiesce stale revisions. |
| `internal/configstore/reset.go` | Modify | Roll back quarantine on every cleanup failure so recovery files remain addressable. |
| `cmd/x6configurator/main.go` | Modify | Construct one reset runner per service with the existing inventory, service quiescer, and state purger; reuse the same constructor path for CLI. |
| `internal/desktop/service_explicit_apply_test.go` | Create | Service-level DPI, polling, and remap blocker tests. |
| `internal/desktop/emergency_reset_test.go`, `internal/desktop/reset_runtime_test.go` | Modify/Create | Lane matrix plus real-service cancellation, restart, and recovery tests. |
| `internal/configstore/reset_test.go`, `cmd/x6configurator/main_test.go` | Modify | Transactional purge failures and composition/delegation. |
| `frontend/src/desktop-contract.ts`, `frontend/src/wails-service.ts` | Modify | Add explicit apply/reset result contracts and facade mappings. |
| `frontend/src/hooks/useDesktopWorkspace.ts` | Modify | Replace queued notices with staged/apply actions; hold confirmation state and consume reset results. |
| `frontend/src/components/panels/{DpiPanel,PollingPanel,ResetPanel,ButtonRemapPanel}.tsx` | Modify | Present apply controls and reset confirmation/cancel UX; no backend logic. |
| `frontend/src/**/*.test.{ts,tsx}` | Modify | Prove inert staging/cancel and explicit confirmation behavior. |
| `cmd/x6configurator/frontend/bindings/**` | Regenerate | Refresh Wails output; never hand-edit. |

## Interfaces / Contracts

`resetRunner.Run(context.Context) (ResetResult, error)` is a private desktop interface. `ResetResult` retains per-lane and cleanup states and adds a typed top-level `Error` plus `RetryAvailable`. Wails exposes `ApplyDPI()`, `ApplyPollingRate()`, `ApplyRemap(config)`, and `ResetToFactory()` through `DesktopService`; remap has no staging RPC.

## Testing Strategy

| Layer | Blocker scenarios |
|---|---|
| Desktop unit | Advance injected schedulers after DPI/polling stage and assert zero command/persistence calls. Table-test remap invalid/missing/stale selection, exact target, ACK failure/success, persistence failure, and persistence-only retry with no second device write. Apply equivalent ACK-gated assertions to DPI/polling. |
| Reset unit | Assert exact lane values/order, no lighting, stop/no purge for failure at each lane, all-ACK purge ordering, cleanup rollback, and typed retryable results. |
| Runtime/race | Build real `Service` + `TargetedService` with fake command and `t.TempDir`: prove `ResetToFactory` delegation, zero/multiple device rejection, quiesced late callbacks, cancellation/disconnect, preserved state, success-only purge/reconciliation, restart reload, reselection, and fresh explicit retry. Run `go test -race ./internal/desktop/...`. |
| Frontend/binding | Vitest apply-button/Enter, Escape/cancel, reset confirm/cancel/failure, and staging-without-apply calls; facade contract test. Run `npm test` and `npm run build`, then `go test ./...` and `go vet ./...`. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable classification, or external process-integration boundary is introduced.

## Migration / Rollout

No data migration or feature flag. Regenerate bindings and ship backend/frontend together; rollback them together while retaining persisted recovery state.

## Open Questions

None.
