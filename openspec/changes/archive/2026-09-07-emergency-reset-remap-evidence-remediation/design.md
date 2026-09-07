# Design: Emergency Reset/Remap Evidence Remediation

## Technical Approach

Make one production correction: carry `SelectionRequired` in the polling snapshot returned by `Service.ApplyPollingRate` when the selected binding is absent, stale, or otherwise invalid, and reject the operation before any I/O. Close the other seven predecessor findings with deterministic, hardware-free assertions around the existing remap, reset, quiescence, persistence, and restart paths. The predecessor capability specs remain authoritative; protocol, lighting, automatic retry, UI behavior, and `ApplyRemap` context changes remain out of scope.

## Architecture Decisions

| Decision | Alternatives considered | Rationale |
|---|---|---|
| Add `Error Error` to `PollingSnapshot`, store it in `pollingState`, and return `failPolling(state, SelectionRequired)` before I/O when the selected binding is absent, stale, or otherwise fails validation. | Return a raw Go error; reuse `Firmware`; change the method signature. | Matches the existing immutable desktop DTO and closed `ErrorCode` contract without exposing error text or reshaping the Wails method. Every invalid-selection case fails closed with zero transport or persistence I/O; other polling behavior remains unchanged. |
| Build lifecycle evidence from real `desktop.Service`, `mouse.TargetedService`, existing scheduler seams, a recording `TargetedCommand`, and `t.TempDir` stores. | Keep the fake reset runner; use hidraw; add production abstractions. | Exercises selection, binding revalidation, ACK gating, quiescence, and persistence while remaining deterministic and hardware-free. |
| Record canonical evidence only in the remediation artifacts: `openspec/changes/emergency-reset-remap-evidence-remediation/tasks.md` for TDD evidence, `openspec/changes/emergency-reset-remap-evidence-remediation/apply-progress.md` for apply evidence, and `openspec/changes/emergency-reset-remap-evidence-remediation/verify-report.md` for independent verification evidence. | Rewrite or supplement artifacts in the blocked predecessor change. | Every artifact under `openspec/changes/emergency-reset-remap-confirmation/` remains unchanged as historical evidence; remediation artifacts may reference predecessor findings but never replace, amend, or delete predecessor artifacts. |
| Prove retry safety through caller-driven repeated invocation and persistence idempotence only. | Add automatic retries in the service, scheduler, or transport. | A caller may explicitly invoke the operation again after a reported failure; already-persisted state prevents a duplicate device write. Automatic retry behavior is out of scope and receives no production path. |

## Data Flow

    ApplyPollingRate(ctx)
      ├─ binding absent/stale/invalid → pollingState copy
      │    → Error{SelectionRequired} → Wails DTO → zero I/O
      └─ valid current binding → existing applyPolling
           → TargetedService → recording command/ACK

    confirmed reset → EmergencyReset.Run → Service.Quiesce
      → DPI ACK → polling ACK → remap ACK → StatePurger.PurgeAll

`Quiesce` cancels both coordinators and increments DPI/polling revisions. Tests retain the scheduled callbacks and invoke them after cancellation; revision and binding checks must reject them before the recording command. Locks, state snapshots, and event emission retain their existing ordering and copy semantics.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/desktop/service.go` | Modify | Add polling error state/DTO mapping and the no-selection failure result. |
| `internal/desktop/service_explicit_apply_test.go` | Modify | Add polling no-selection proof and table-driven remap validation, binding, ACK, persistence, and retry assertions. |
| `internal/desktop/emergency_reset_test.go` | Modify | Assert exact ordered lane values/reports, no lighting report, and unchanged persisted bytes for each failed lane. |
| `internal/desktop/reset_runtime_test.go` | Modify | Replace/extend fake-runner coverage with the real-service quiescence, interruption, restart, reselection, and fresh-confirmation harness. |
| `frontend/src/desktop-contract.ts` | Modify | Add required `Error: { Code: string }` to `PollingSnapshot`. |
| `frontend/src/App.test.tsx` | Modify | Keep polling snapshot fixtures structurally complete; no UI behavior change. |
| `cmd/x6configurator/frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models.ts` | Regenerate | Generated polling DTO gains `Error`. |
| `frontend/bindings/github.com/blak0p/attack-shark-linux/internal/desktop/models.ts` | Regenerate | Mirror the generated polling DTO contract. |
| `cmd/x6configurator/main_test.go` | Modify | Assert both generated roots contain the polling error field. |

`internal/desktop/emergency_reset.go`, protocol packages, `frontend/src/wails-service.ts`, and predecessor artifacts require no change.

## Interfaces / Contracts

`PollingSnapshot.Error.Code` is the existing closed `desktop.ErrorCode`; an absent, stale, or otherwise invalid selected binding yields `selection_required`, zero transport and persistence I/O, and unchanged desired/applied/persisted values. Generated files are changed only by `wails3 generate bindings -ts`; the facade method remains `ApplyPollingRate(): Promise<PollingSnapshot>`.

The test command fake implements `mouse.TargetedCommand`, records exact `Binding` and report bytes, supplies operation-specific ACKs, and can fail a selected lane. The manual scheduler records callbacks and exposes explicit late firing without wall-clock sleeps. Same-package tests inject the existing remap persistence seam; production wiring is not expanded.

## Testing Strategy

| Finding | Evidence |
|---|---|
| 1 | Polling table for absent, stale, and otherwise invalid selected bindings; every case asserts `selection_required`, zero command/open/persist operations, and the generated contract. |
| 2 | Remap table: invalid/missing/stale, exact target, ACK fail/success, persistence fail/success, and a caller-driven repeated invocation proving persisted idempotence without a second device write. No automatic retry is introduced or asserted. |
| 3, 6 | Stage and schedule DPI/polling, quiesce through reset, fire captured callbacks, assert no late command or state overwrite. |
| 4 | Compare all three encoded values/order with documented DPI, 1000 Hz, and default remap; reject report ID `0x05`. |
| 5 | For failures at lanes 1–3, compare seeded recovery files byte-for-byte and assert retryable stop state. |
| 7 | Interrupt a real reset, reconstruct services over the same temporary store, refresh/reselect, assert zero writes/purge before a fresh confirmed three-lane run. |
| 8 | `openspec/changes/emergency-reset-remap-evidence-remediation/tasks.md` records concrete RED/GREEN TDD evidence with `✅ Written`/`✅ Passed`; `apply-progress.md` records cumulative apply evidence; `verify-report.md` maps every claim to executable assertions and the independent verdict. |

Later apply/verify runs focused Go tests, race tests, full Go tests/build/vet, binding generation, and frontend tests/build. No test opens hidraw, claims USB, or assumes root. The remediation evidence is additive and change-local; all predecessor files under `openspec/changes/emergency-reset-remap-confirmation/` are preserved unchanged.

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable classification, or process-integration execution boundary changes.

## Migration / Rollout

No data migration or feature flag. Regenerate both binding roots atomically with the additive DTO change.

## Open Questions

None.
