# Apply Progress: Emergency Reset/Remap Evidence Remediation

## Completed Work Units

### PR 1: Polling Contract

- [x] 1.1 Table-driven absent, stale, and invalid polling-selection cases prove `selection_required`, unchanged polling state, and zero command or persistence I/O.
- [x] 1.2 Polling state and DTO snapshots carry `Error`; invalid or stale selection returns `SelectionRequired` before I/O without changing the method signature.
- [x] 1.3 Desktop and frontend polling snapshot contracts require the `Error` field; fixtures remain structurally complete.
- [x] 1.4 Supported binding generation updated both generated desktop binding roots; the typed facade remains unchanged.

## TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 1.1 | ✅ Written — `TestApplyPollingRateRejectsInvalidSelectionWithoutIO` covers absent, stale, and invalid selections. | ✅ Passed — `go test ./internal/desktop -run 'Test.*Polling' -count=1` (exit 0). | No refactor required. |
| 1.2 | Covered by task 1.1's selection assertions. | ✅ Passed — focused polling test and `go vet ./...` / `go build ./...` (all exit 0). | No refactor required. |
| 1.3 | ✅ Written — desktop binding and frontend fixture assertions require `PollingSnapshot.Error`. | ✅ Passed — `npm test -- --run src/App.test.tsx` (1 file, 46 tests; exit 0). | No refactor required. |
| 1.4 | Covered by task 1.3's generated-contract assertions. | ✅ Passed — `npm run build` (Vite production build; exit 0). | No refactor required. |
| 2.1 | ✅ Written — `TestApplyRemapValidatesBindingACKAndPersistence` covers invalid, missing, and stale bindings plus ACK and persistence outcomes. | ✅ Passed — `go test ./internal/desktop -run 'Test.*(Remap|EmergencyReset)' -count=1` (exit 0). | No refactor required. |
| 2.2 | Covered by task 2.1's caller-driven repeat assertion. | ✅ Passed — repeated identical remap retries persistence only and emits no second device write. | Added a small value-equality guard instead of a new abstraction. |
| 2.3 | ✅ Written — reset assertions compare exact DPI, 1000 Hz, and default remap reports; lane failures retain byte-identical seeded files. | ✅ Passed — `go test ./internal/desktop -run 'Test.*(Remap|EmergencyReset)' -count=1` (exit 0). | No refactor required. |
| 2.4 | Covered by task 2.3's all-lane and failure assertions. | ✅ Passed — purge remains unreachable for failures in lanes 1–3, with no lighting report. | No refactor required. |
| 3.1 | ✅ Written — real-service quiescence and restart/reselection scenarios run in `reset_runtime_test.go`. | ✅ Passed — `go test ./internal/desktop -run 'TestResetRuntime' -count=1` (exit 0). | No refactor required. |
| 3.2 | Covered by task 3.1's recording command, late scheduler, and `t.TempDir` state directory. | ✅ Passed — late callbacks and restart produce zero pre-confirmation writes; the fresh confirmed reset performs exactly three lanes. | Test-only harness; no production hardware path changed. |
| 3.3 | ✅ Written — canonical evidence is recorded in this remediation's tasks and apply-progress artifacts only. | ✅ Passed — all tasks are checked in `tasks.md`; predecessor artifacts were not edited. | No refactor required. |

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test ./internal/desktop -run 'Test.*Polling' -count=1` — exit 0, package passed. `go test ./...` — exit 0, 8 packages passed. |
| Runtime harness command/scenario and exact result | The focused command runs deterministic `desktop.Service` cases with fake targeted commands and mutable inventory; absent, stale, and invalid selections returned `selection_required` with zero writes and zero persistence saves (exit 0). No hardware boundary exists. |
| Contract verification | `npm test -- --run src/App.test.tsx` — exit 0, 1 file and 46 tests passed. `npm run build` — exit 0, 64 modules transformed. |
| Static/build verification | `go vet ./...` — exit 0. `go build ./...` — exit 0. |
| Rollback boundary | Revert only the polling error state/snapshot mapping, PR 1 polling tests, frontend polling contract/fixtures, and both regenerated desktop binding roots; no protocol, hardware, reset, remap, or facade behavior is part of this unit. |

### PR 2: Remap and Reset Assertions

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test ./internal/desktop -run 'Test.*(Remap|EmergencyReset)' -count=1` — exit 0, package passed. `go test -race ./internal/desktop -run 'Test.*(Remap|EmergencyReset|ResetRuntime)' -count=1` — exit 0, package passed. |
| Runtime harness command/scenario and exact result | The focused tests use recording `mouse.TargetedCommand` fakes and `configstore.NewStatePurger` over `t.TempDir`; ACK failure leaves remap applied state unchanged, persistence retry does not repeat the device write, and failures in reset lanes 1–3 leave seeded recovery bytes unchanged (exit 0). No hardware boundary exists. |
| Rollback boundary | Revert `remapConfigsEqual`, the remap explicit-apply assertions, and the emergency-reset report/recovery assertions; no protocol, lighting, generated binding, or hardware code is part of this unit. |

### PR 3: Lifecycle Harness

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test ./internal/desktop -run 'TestResetRuntime' -count=1` — exit 0, package passed. |
| Runtime harness command/scenario and exact result | `TestResetRuntimeQuiescesLateCallbacksBeforeResetWrites` schedules DPI and polling work on a manual scheduler, runs the real reset, then fires retained callbacks; only the three reset writes occur. `TestResetRuntimeRestartRequiresFreshConfirmedResetAfterFailure` reconstructs the service over the same `t.TempDir` state, reselection produces zero writes, and only a fresh confirmed reset purges state (exit 0). |
| Static/build verification | `go test ./...`, `go vet ./...`, and `go build ./...` — all exit 0. |
| Rollback boundary | Revert `internal/desktop/reset_runtime_test.go` harness and the remap idempotence guard; no production scheduler, protocol, persistence schema, or hardware path changes are required. |

## Scope and Status

- Delivery mode: single foundational PR with maintainer-authorized `size:exception`.
- Boundary: this cumulative apply covers polling rejection, remap/reset evidence, and deterministic lifecycle harnesses; generated bindings and predecessor artifacts remain untouched in this batch.
- Review budget: 800-line session exception authorized by the maintainer; the scope remains confined to the remediation work units.
- Remaining tasks: none. Independent `sdd-verify` is required before archive.
