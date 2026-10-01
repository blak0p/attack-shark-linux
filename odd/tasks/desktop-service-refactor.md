# Desktop service decomposition

## Objective and rationale
Make `internal/desktop/service.go` manageable by extracting domain ownership while keeping `Service` as a thin Wails-facing facade. Preserve frontend API, device-selection safety, explicit apply/ACK, persistence, listener events and emergency-reset sequencing. Browser remapping merged as PR #115 after this branch forked; reconcile overlapping changes at integration, not within isolated extraction units.

## Baseline, scope and route
- Worktree: `/home/alejandro/dev/attack_shark_linux-worktrees/desktop-service-refactor`; branch `feat/desktop-service-refactor`, forked from `main` d015e96.
- Extract selection, inventory/listener, DPI, polling/shared settings, lighting and remap. Preserve exported Wails methods, snapshots and events; do not hand-edit generated bindings, modify sibling worktrees, redesign UI/protocol/reset policy or change storage format.
- Parent owns this task document, Engram mirror, review and delivery. Route: delegated-direct, one bounded writer per multi-file unit; read-only mapping when understanding spans 4+ files. No SDD route selected.
- Delivery: ask-on-risk; user selected stacked-to-main for future PRs. Forecast exceeds 400 authored changed lines; coherent code/tests units, not size targets. No push/PR/merge without a separate user decision. User authorized local DS-6 commit, candidate review and continuation to DS-7 on 2026-09-30; no push/PR/merge.
- Strict TDD ON from explicit existing user choice: observed RED → GREEN → REFACTOR; Go `go test`, focused desktop/race/composition checks. Frontend Vitest `npm test` covers adapter contracts. Build ignored frontend dist when required by embed; do not invent RED or successful checks.

## Tasks
- [x] DS-1 (delegated): Centralize selection resolution; characterize selected/stale bindings and isolation.
- [x] DS-2 (delegated): Extract inventory and listener; preserve events, lock ordering and scheduled-write cancellation.
- [x] DS-3 (delegated): Extract DPI stage/apply/retry, state and sync; preserve ACK and reset access.
- [x] DS-4 (delegated): Extract polling and shared settings; preserve unrelated DeviceConfig fields and sleep integration.
- [x] DS-5 (delegated): Extract lighting state/stage/apply and shared sleep/response-time behavior.
- [x] DS-6 (delegated): Extract remap state/apply/retry and serialized writes; preserve validation, ACK, events and isolation. Implementation and independent checks passed; committed as 41c69b5 and native review approved/acknowledged.
- [x] DS-7 (delegated): Reduce facade to orchestration/delegation; check composition/reset/CLI and generated-binding compatibility without hand-editing outputs. Structural RED/GREEN and independent checks passed; commit and candidate review pending.
- [ ] DS-8: Review work-unit diffs, run applicable backend/frontend checks, record blockers and leave reviewable branch for integration with merged browser work.

## Acceptance and checks
- Public Wails API remains compatible; mutable domain state and logic leave the god object.
- Reject stale bindings; no cross-device state attribution; deterministic lock ordering and no races.
- Preserve mouse:status, mouse:configuration, mouse:polling-configuration and mouse:remap-configuration events, explicit apply and persistence retry semantics.
- EmergencyReset.Run keeps quiesce → DPI/polling/remap ACK → purge ordering; reset composition and CLI remain functional.
- Per-unit focused Go checks; closure checks include desktop races, go test ./..., go vet ./..., frontend npm test and diff whitespace. Record observed results, not assumptions.

## Completed-unit evidence
- DS-1 `7eed45d`: selectionResolver compile RED/GREEN; 110 desktop and 30 focused race tests, independent verification and parent spot-check passed. Native medium review approved/acknowledged `review-98433cca594ecce8`; advisory R3-001 nonblocking. Initial untracked assessment was unassessable.
- DS-2 inventory `c05d95c`: missing inventory owner RED/GREEN; 111 desktop and 31 focused race tests, independent verification passed. Native medium review approved/acknowledged `review-aa16c71fd82926b7`. Listener `9244441`: missing listener owner RED/GREEN; 112 desktop and 17 focused race tests passed independently. Native review approved/acknowledged `review-e1db51ff7a1c13a0`. Composition reserved for DS-7.
- DS-3 `9888bcb`: missing DPI collaborator/fields RED/GREEN; 114 desktop and 56 focused race tests passed independently. Native review approved/acknowledged `review-4e92cd517cb08a8a`. Cohesive 439-line unit may require PR size exception.
- DS-4 polling `3486afa`: missing collaborator RED/GREEN; 115 desktop and 29 focused race tests passed independently; whitespace defect corrected. Its separate native outcome was not recorded, do not infer approval. Settings `880e04cf373b45955fea963be43088971b651d48`: per-field shared persistence, selected reload/reset and captured apply/retry values; observed regressions for field preservation and concurrent Stage turned GREEN. Final checks passed: 462 Go tests/11 packages, 122 desktop race tests, vet, 106 frontend tests, diff check. Native high-risk review approved/acknowledged `review-22e87aabd85c1d27` for settings against 3486afa. Nonblocking R2-001/R3-001: 20ms concurrency-test scheduling window cannot prove Stage entered before release, even with startup signal and bounded waits. Carry as test-hardening follow-up, not production failure.
- DS-5 `5610f04`: lighting owner compile RED/GREEN; deterministic sleep captured-A/B-lighting report regression RED/GREEN using synchronous selection hooks and bounded report receive. Checks passed: 464 Go tests/11 packages, 124 desktop/race tests, vet, 106 frontend tests and diff check; timeout-only hardening targeted test passed. Review deferred at user's stop request, then resumed 2026-09-30: assessment schema-incompatible/unassessable; native START scoped six paths/321 lines against 880e04c. Medium reliability review approved and exact acknowledgement burned authority `review-a34afe256b0d6932`. No delivery performed.

## DS-6 progress and verification — 2026-09-30
- Read-only mapping preceded one bounded writer (4-file mapping and multi-file write triggers). Remap drafts enter through ApplyRemap; no new Go StageRemap API. Retry remains persistence-only. Scope: new internal/desktop/remap.go and remap_tdd_test.go, service.go delegation; no protocol/frontend/generated/browser changes.
- Runtime RED: synchronous persistence-load selection switch made B receive stale_binding/failed from A's apply while A returned pending with no error. GREEN: capture stateForBinding with the binding and pass the same state through apply/retry. Ownership/snapshot-copy isolation, shared-field preservation and exactly one firmware write across successful persistence retry are covered.
- Owner encapsulates state, Apply/Retry, snapshot and reset reconciliation. Locks: operationMu → captured state applyMu, inventory physical-write serialization, shared configMu persistence. Preserve ACK, events, reset sequence and public methods.
- Writer passed gofmt, 19 focused tests and focused races, 127 desktop tests, 223 desktop/x6/protocol/mouse composition tests and git diff --check. DS-6 code/tests: +380/-188 across three files including two new untracked files; coherent extraction exceeds advisory threshold, not a reason for size-only rework.
- Native assessment over working diff returned unassessable because intended untracked files were undeclared; unknown writer profile/outcome required independent verification. Fresh read-only verifier found no concrete regression or blocker and passed, without prefixes/workarounds:
  - go test ./internal/desktop -run 'Test(RemapOwner|RemapApply|ApplyRemap)' -count=1: 19 tests.
  - go test -race ./internal/desktop -count=1: 127 tests.
  - go test ./... -count=1: 467 tests, 11 packages.
  - go vet ./...: passed.
  - git diff --check: passed (does not include untracked files).
- Verifier confirmed meaningful deterministic attribution test by source comparison; did not independently execute RED against HEAD. ACK/persistence selection-change paths retain captured state by inspection but lack equivalent new deterministic assertions. Hardware and regenerated bindings were not exercised.
- Preexisting follow-up, not fixed or accepted as new scope: failed persistence retry marks firmware failed (remap.go retryBound/failRemap), so later identical Apply may resend firmware. HEAD already had this behavior; new retry test covers successful retry only.

## Next step
DS-6 committed locally as `41c69b516bfed1073395619caae222d05f6663e8` with code, tests and task record (423 additions/211 deletions across four files). User authorized its commit/review and DS-7 continuation. Final spot check passed: 19 focused tests, diff check, gofmt and untracked whitespace scan. Committed assessment remained schema-incompatible/unassessable; independent verification already passed. Native review of exact DS-6 slice against `5610f0475062dce2e3e071f30e1084f2fe18d11b` classified medium and approved; exact acknowledgement burned authority `review-1bf8edb4e8c41d16`. Worktree was clean immediately after commit; this evidence update is parent-owned and uncommitted. DS-7 delegated mapping and implementation/verification subsequently completed as recorded below. DS-8 frontend/integration checks remain pending. No push/PR/merge authorized.

## DS-7 progress and verification — 2026-09-30
- Read-only mapper narrowed scope to existing dpiComponent RefreshStatus and successful reset DPI reconciliation; Service retains cross-domain orchestration. CLI/desktop already share EmergencyReset, so no composition rewrite required. No declaration-only line relocation or new abstraction.
- Writer changed dpi.go, service.go and service_tdd_test.go; parent task document preserved. Clean structural RED: refreshStatus/reconcileFactoryReset undefined; GREEN: six owner tests. Earlier test compile errors were corrected before the clean RED; no runtime-defect RED claimed. Tests cover status success/unavailable battery/classified failures, reset defaults/revision/retry/error/success.
- Worker initially omitted the required handoff; parent recovered actual prior evidence through read-only continuation, not inferred from diff. Prior worker checks passed: focused six/14, desktop 133, race 133, CLI 30, full Go 473/11 packages, vet, gofmt and diff check.
- Native assessment schema-incompatible/unassessable with unknown writer profile/outcome required independent verification. Fresh verifier repeated: focused status/reset 14, desktop race 133, CLI 30, full Go 473/11 packages, vet, diff check and gofmt readback; all passed. No candidate-caused blocker or regression found.
- Status IO, state-selection timing, state.mu protection, classifier and battery semantics unchanged. Reset operationMu, runner-result propagation and successful-cleanup gate unchanged; DPI unlock completes before polling/remap reconciliation. No events added.
- Binding compatibility by source inspection: Go RefreshStatus(context.Context) Snapshot and ResetToFactory(context.Context) ResetResult unchanged; both generated copies identical by cmp and unchanged against HEAD. Zero-argument wrappers retain IDs 3077675599/2708725118; DTOs and desktop-contract/wails-service forwarding match. No generation/frontend/runtime/hardware execution claimed.
- Preexisting safety limitation: status state is selected after IO, so a selection switch during IO may attribute status to later selection; tests cover legacy state, not deterministic selection-switch attribution. Preexisting coverage caveat: reset success test named OnlySuccessfulRun lacks a failure assertion on unchanged DPI snapshots. These are follow-ups, not DS-7 regressions or authorized scope expansion.

## Current next step
DS-7 implementation/independent verification complete but uncommitted; only dpi.go, service.go, service_tdd_test.go and parent task document changed. User explicitly authorized local DS-7 commit/review and DS-8 continuation. Final independent spot check passed six owner tests, diff check and gofmt readback; exactly four expected paths changed. Commit this work unit and assess/review against 41c69b5. DS-8 closure still needs frontend/integration checks and boundary reconciliation with merged browser work. No push/PR/merge authorized.
