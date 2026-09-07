# Tasks: Emergency Reset and Remap Confirmation

Edit only listed paths; preserve unrelated worktree changes and do not reset, checkout, or regenerate other outputs.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 650–900 lines, plus generated bindings |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 service → PR 2 reset → PR 3 configstore/composition → PR 4 frontend → PR 5 bindings/final verification |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Delivery Resolution

- Resolved mode: `feature-branch-chain`.
- Current work unit: autonomous PR 1, service safety boundary; it targets the feature/tracker branch and does not target `main` directly.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Explicit service applies | PR 1 | `go test ./internal/desktop -run 'Test.*(Apply|Stage)'` | `go test -race ./internal/desktop/...` | `internal/desktop/service.go` and its tests |
| 2 | Ordered reset lifecycle | PR 2 | `go test ./internal/desktop -run 'TestEmergencyReset'` | Real `Service` reset with fake command and `t.TempDir` | `emergency_reset.go` and reset tests |
| 3 | Durable purge and wiring | PR 3 | `go test ./internal/configstore ./cmd/x6configurator` | Composition reset-delegation test | `reset.go`, `main.go`, and tests |
| 4 | Confirmation UI and facade | PR 4 | `cd frontend && npm test -- --run` | `cd frontend && npm run build` | Listed contract, hook, panel, and UI files |
| 5 | Generated contract and proof | PR 5 | `go test ./... && go vet ./...` | `cd frontend && npm run build` | Generated binding outputs only |

## Phase 1: Service Safety Boundary (test first)

- [x] 1.1 RED: Add `internal/desktop/service_explicit_apply_test.go` cases for inert staging, selection, remap validation/targeting, ACK/persistence failure, retry without a second write, and ACK-gated state.
- [x] 1.2 GREEN/REFACTOR: Update `internal/desktop/service.go` to remove scheduler writes, add explicit DPI/polling/remap operations, serialize applies/reset, preserve recovery state, and emit only after unlock.

## Phase 2: Reset Orchestration (test first)

- [x] 2.1 RED: Extend `internal/desktop/emergency_reset_test.go` and create `internal/desktop/reset_runtime_test.go` for lane order/no lighting, every failed lane, cancellation, disconnect, restart, reselection, late callbacks, and success-only purge.
- [x] 2.2 GREEN/REFACTOR: Update `internal/desktop/emergency_reset.go` and `service.go` for context cancellation, typed retryable results, quiesced revisions, reset serialization, and success-only reconciliation; wire delegation in `cmd/x6configurator/main.go`.

## Phase 3: Persistence and Composition (test first)

- [x] 3.1 RED: Extend `internal/configstore/reset_test.go` and `cmd/x6configurator/main_test.go` for quarantine/recreate/sync/remove failures, rollback addressability, and one reset runner per service.
- [x] 3.2 GREEN: Update `internal/configstore/reset.go` to restore quarantine on every cleanup failure and complete constructor wiring in `cmd/x6configurator/main.go`.

## Phase 4: Frontend, Bindings, and Verification (test first)

- [x] 4.1 RED: Extend `frontend/src/wails-service.test.ts`, `frontend/src/hooks/useDesktopWorkspace.test.ts`, and panel tests for staging, Apply/Enter, Escape/cancel, reset confirmation/failure, and facade mapping.
- [x] 4.2 GREEN: Update `frontend/src/desktop-contract.ts`, `frontend/src/wails-service.ts`, `frontend/src/hooks/useDesktopWorkspace.ts`, `frontend/src/App.tsx`, and `frontend/src/components/panels/{DpiPanel,PollingPanel,ResetPanel,ButtonRemapPanel}.tsx`; keep components presentational.
- [x] 4.3 Regenerate `cmd/x6configurator/frontend/bindings/**` (generated; never hand-edit), reconcile DTOs without editing generated files manually, then run `cd frontend && npm test`, `npm run build`, `go test -race ./internal/desktop/...`, `go test ./...`, and `go vet ./...`.
