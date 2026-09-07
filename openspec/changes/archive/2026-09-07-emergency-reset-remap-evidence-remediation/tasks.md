# Tasks: Emergency Reset/Remap Evidence Remediation

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550–700 authored lines across 9 files |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: polling contract; PR 2: remap/reset assertions; PR 3: lifecycle harness and evidence |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Fail closed on invalid polling selection and propagate DTO | PR 1 | `go test ./internal/desktop -run 'Test.*Polling' -count=1` | Deterministic selected/absent/stale `Service` cases; no hardware | Revert polling error state, polling tests, contract, bindings, and fixture assertions |
| 2 | Close remap and reset lane evidence gaps | PR 2 | `go test ./internal/desktop -run 'Test.*(Remap|EmergencyReset)' -count=1` | Recording command/ACK fake with exact reports and seeded persistence | Revert the added assertion blocks in the two focused test files |
| 3 | Prove lifecycle safety through restart and reselection | PR 3 | `go test ./internal/desktop -run 'TestResetRuntime' -count=1` | Real `Service`/`TargetedService`, manual scheduler, `t.TempDir`, no sleeps | Revert `reset_runtime_test.go` harness only |

## Phase 1: Polling Contract

- [x] 1.1 **RED** — In `internal/desktop/service_explicit_apply_test.go`, add table-driven absent, stale, and invalid-selection cases asserting `selection_required`, unchanged state, and zero command/open/persistence I/O. ✅ Written ✅ Passed
- [x] 1.2 **GREEN** — In `internal/desktop/service.go`, add `Error Error` to polling state/snapshot mapping and return `failPolling(state, SelectionRequired)` before any I/O; preserve the method signature. ✅ Passed
- [x] 1.3 **RED** — In `cmd/x6configurator/main_test.go` and `frontend/src/App.test.tsx`, require complete polling snapshots including `Error`; update `frontend/src/desktop-contract.ts` to make the contract assertion fail until generated types match. ✅ Written ✅ Passed
- [x] 1.4 **GREEN** — Run the supported Wails binding generator, update both generated `models.ts` roots, and make the frontend fixtures compile without changing `frontend/src/wails-service.ts`. ✅ Passed

## Phase 2: Explicit Remap and Reset Evidence

- [x] 2.1 **RED** — Extend `internal/desktop/service_explicit_apply_test.go` with table-driven remap validation, exact binding, ACK failure/success, persistence failure/success, and caller-driven repeat-idempotence cases; forbid automatic retry. ✅ Written ✅ Passed
- [x] 2.2 **GREEN** — Make the existing remap path and test seams satisfy those assertions: applied state advances only after ACK, persistence failure is retryable, and a repeated caller invocation emits no duplicate device write. ✅ Passed
- [x] 2.3 **RED** — In `internal/desktop/emergency_reset_test.go`, assert exact DPI/1000-Hz/default-remap order and bytes, no lighting report, and byte-identical seeded recovery files for lane failures 1–3. ✅ Written ✅ Passed
- [x] 2.4 **GREEN** — Keep purge unreachable until all three ACKs succeed and make the focused reset assertions pass without changing protocol or lighting behavior. ✅ Passed

## Phase 3: Deterministic Runtime Harness and Evidence

- [x] 3.1 **RED** — In `internal/desktop/reset_runtime_test.go`, specify real-service scenarios for quiescence/late callbacks (findings 3,6), interruption/restart/reselection (7), and fresh confirmed reset (4,5). ✅ Written ✅ Passed
- [x] 3.2 **GREEN** — Implement test-only recording `mouse.TargetedCommand`, operation ACK controls, manual scheduler, and `t.TempDir` persistence; prove zero late/pre-confirmation writes and no premature purge without hardware or sleeps. ✅ Passed
- [x] 3.3 Record canonical TDD evidence in this `tasks.md` as `✅ Written`/`✅ Passed`, leave predecessor artifacts unchanged, and reserve cumulative apply/independent evidence for `apply-progress.md`/`verify-report.md`. ✅ Written ✅ Passed
