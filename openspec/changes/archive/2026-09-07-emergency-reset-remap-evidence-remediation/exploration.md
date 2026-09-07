# Exploration: Emergency Reset/Remap Evidence Remediation

status: ready
next_recommended: sdd-new
artifact_store: openspec

## Executive Summary

Independent verification failed on eight critical gaps. The implementation is
largely present and the existing Go/frontend/build commands pass, but several
required runtime assertions are absent. One gap is a real behavior defect:
`Service.ApplyPollingRate` silently returns an unchanged snapshot when no
validated binding exists. The bounded remediation is to fix that contract,
add the missing service and deterministic runtime assertions, and replace the
non-representative reset runtime harness with a real service/targeted-service
harness. It does not add new product capability, hardware tests, lighting
reset, automatic retry, or unrelated refactoring.

## Current State

- `internal/desktop/service.go` already exposes explicit DPI, polling, and
  remap apply methods, serializes operations with `operationMu`, and delegates
  factory reset to the shared `EmergencyReset` runner.
- `internal/desktop/emergency_reset.go` already performs the documented
  `DPI → 1000 Hz polling → default remap` sequence, quiesces pending writes,
  stops on failure, and purges only after all lanes succeed.
- `internal/mouse/service.go` validates the immutable selected binding before
  encoding or sending an operation and sends through the ACK-aware bounded
  command path.
- Existing tests and verification commands pass, but the report proves only
  9/18 scenarios and 2/8 requirements completely. The current
  `reset_runtime_test.go` uses a fake reset runner, so it does not exercise the
  real `Service → EmergencyReset → TargetedService → persistence` flow.

## Eight Critical Gaps and Bounded Remediation

1. **Polling selection behavior — missing behavior.**
   `Service.ApplyPollingRate(ctx)` calls `selectedBinding()` and returns
   `pollingSnapshotOf(state)` without setting `SelectionRequired` when no
   binding exists (`internal/desktop/service.go:842-850`). Add the typed error
   to `PollingSnapshot` and map the no-selection result consistently with DPI
   and remap. Update the desktop contract/facade and regenerated bindings as
   required by the existing DTO boundary. Add a no-selection service test.

2. **Remap service coverage — missing evidence.**
   `ApplyRemap` validates and targets through
   `applyRemapBound` (`internal/desktop/service.go:670-723`), but its current
   tests do not prove invalid-draft state preservation, exact selected-binding
   targeting, ACK failure, or acknowledged persistence success. Extend
   `internal/desktop/service_explicit_apply_test.go` with one table-driven
   service suite covering invalid, missing, stale, exact-binding, ACK-failure,
   ACK-success, persistence-failure, and persistence-only retry cases. Do not
   change remap protocol encoding.

3. **Configuration cancellation/restart runtime proof — missing evidence.**
   Existing coordinator tests do not combine a staged configuration, service
   cancellation/reset, a late callback, and a fresh explicit operation. Extend
   `internal/desktop/reset_runtime_test.go` with deterministic scheduler and
   command fakes proving no late write/state overwrite and a fresh write only
   after explicit confirmation.

4. **Successful reset lane contract — missing evidence.**
   `EmergencyReset.Run` constructs the correct values at
   `internal/desktop/emergency_reset.go:81-88`, but
   `TestEmergencyResetPurgesOnlyAfterAllLanesSucceed` asserts only the first
   operation type and total count. Strengthen
   `internal/desktop/emergency_reset_test.go` to assert exact DPI values,
   polling value `1000`, default remap contents, operation order, and zero
   lighting operations.

5. **Per-lane persisted-state preservation — missing evidence.**
   Failure tests cover stopping and one no-purge path, but not persisted-state
   preservation independently for lane 1, 2, and 3. Make the reset failure
   matrix in `internal/desktop/emergency_reset_test.go` assert unchanged
   persisted recovery state and retryability for each failing lane.

6. **`Service.Quiesce` coverage — missing evidence.**
   `Service.Quiesce` (`internal/desktop/emergency_reset.go:118-138`) cancels
   both coordinators and increments DPI/polling revisions, but has 0% runtime
   coverage. Add a deterministic test in
   `internal/desktop/reset_runtime_test.go` that captures callbacks before
   quiescence, invokes `Quiesce`, fires them afterward, and asserts zero
   commands and no state overwrite.

7. **Real-service restart/reselection harness — missing evidence.**
   The current runtime test injects a fake runner instead of constructing the
   real reset path. Build the bounded hardware-free harness in
   `internal/desktop/reset_runtime_test.go` from a real `Service`,
   `mouse.TargetedService`, fake `CommandTransport`, and `t.TempDir`
   configstore. Cover interrupted/cancelled reset, preserved state, service
   reconstruction, reselection of the current binding, and a fresh explicit
   three-lane attempt with no pre-confirmation writes or premature purge.

8. **Strict TDD evidence quality — missing evidence.**
   The implementation rows in `apply-progress.md` use noncanonical labels
   such as “service.go tests”, and the progress table overstates runtime
   coverage. After remediation, refresh the evidence in the new change’s
   proposal/spec/tasks/design and the original verification record only in the
   later apply/verify phase: name concrete test files, use `✅ Written` and
   `✅ Passed` markers, and make the compliance matrix match executable
   assertions. This exploration itself does not modify those existing
   artifacts.

## Affected Areas

- `internal/desktop/service.go` — fix polling no-selection behavior; retain
  existing explicit apply and reset boundaries.
- `internal/desktop/emergency_reset.go` — no intended behavior change beyond
  evidence-driven test seams if strictly necessary; preserve context, ACK,
  quiescence, and purge semantics.
- `internal/desktop/service_explicit_apply_test.go` — complete remap and
  polling service-level assertions.
- `internal/desktop/emergency_reset_test.go` — exact reset lane and per-lane
  persistence assertions.
- `internal/desktop/reset_runtime_test.go` — real-service quiescence,
  cancellation, late-callback, restart, reselection, and fresh-reset harness.
- `frontend/src/desktop-contract.ts`, `frontend/src/wails-service.ts`, and
  generated Wails desktop bindings — only if the polling error DTO is changed;
  generated files remain generator-owned.
- `frontend/src/**/*.test.{ts,tsx}` — only if the polling error contract
  requires a corresponding facade/UI assertion; no new UI feature is proposed.
- Later SDD verification artifacts — reconcile TDD and scenario evidence; no
  existing artifact is changed during explore.

## Missing Behavior vs. Missing Evidence

| Gap | Classification | Proof/fix boundary |
|---|---|---|
| Polling apply without selection | Missing behavior | Change DTO/result mapping and add service test |
| Remap validation/targeting/ACK/persistence cases | Missing evidence | Add service tests; implementation path already exists |
| Cancellation plus late callback | Missing evidence | Add deterministic real-service test |
| Exact successful reset lanes/no lighting | Missing evidence | Strengthen reset fake assertions |
| State preservation for every failed lane | Missing evidence | Expand failure matrix and persistence assertions |
| `Service.Quiesce` | Missing evidence | Exercise existing revision/cancel logic |
| Restart/reselection after interruption | Missing evidence | Replace fake runner with real composed harness |
| Canonical TDD accounting | Missing evidence | Correct later verification/progress records |

## Approaches

1. **Bounded evidence remediation (recommended)** — fix the single polling
   selection defect, add the exact missing assertions, and use a real,
   hardware-free service composition only for the unproved lifecycle paths.
   - Pros: directly closes all eight findings and preserves the original scope.
   - Cons: requires careful fake command/scheduler seams and likely a generated
     DTO refresh.
   - Effort: Medium.

2. **Test-only remediation** — add assertions without changing the polling
   selection result.
   - Pros: smaller diff.
   - Cons: leaves a specification contradiction and cannot produce a passing
     verification result.
   - Effort: Low, but unacceptable.

## Recommendation

Proceed to `sdd-new` for the recommended bounded remediation. Keep the change
limited to the polling selection contract, the named desktop/reset tests, the
minimum generated contract updates caused by that DTO change, and canonical
evidence reconciliation. Do not broaden it into protocol changes, hardware
validation, lighting behavior, UI redesign, or automatic retry.

## Risks

- Adding an error field to `PollingSnapshot` crosses the Go/Wails/TypeScript
  contract and requires supported binding generation; generated bindings must
  not be hand-edited.
- A real `TargetedService` harness can accidentally test discovery details
  rather than reset behavior; keep command, enumeration, binding validation,
  and persistence fakes deterministic and assert the exact selected binding.
- Cancellation tests must prove no delayed write without claiming USB
  ownership or opening a hidraw node; all tests remain hardware-free.
- `ApplyRemap` currently uses `context.Background()` at its HID boundary, a
  design warning noted by verification. It is intentionally not an additional
  critical gap in this bounded remediation; expanding it would require an
  explicit scope decision.

## Ready for Proposal

Yes. The evidence failure is sufficiently bounded for a new OpenSpec change:
one confirmed behavior defect, seven evidence remediations, exact affected
files/tests, and explicit non-goals are identified.

## Skill Resolution

- `sdd-explore` — applied for this named OpenSpec exploration and artifact.
- `go-layering` — confirms the fix stays in `desktop`/DTO boundaries and keeps
  protocol, mouse, and hardware dependencies directed inward.
- `go-style` — requires context-aware I/O, typed errors, and no string-based
  error branching in the remediation.
- `frontend-binding` — applies only to the minimum DTO/facade regeneration;
  generated bindings remain untouched by hand.
- `hidraw-safety` — confirms all new tests use fake command paths, passive
  semantics, no USB claims, no hidraw access, and no root assumption.
