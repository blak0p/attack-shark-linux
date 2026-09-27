# Desktop service decomposition

## Objective and rationale
Make `internal/desktop/service.go` manageable by extracting domain ownership from the monolithic `Service`, while keeping `Service` as a thin Wails-facing facade. Preserve frontend API, device-selection safety, explicit-apply/ACK behavior, persistence, listener events, and emergency-reset sequencing. The concurrent browser-button-remapping branch has uncommitted changes in its own `service.go`; do not touch or merge it during this work.

## Baseline and scope
- Isolated worktree: `/home/alejandro/dev/attack_shark_linux-worktrees/desktop-service-refactor`, branch `feat/desktop-service-refactor`, starting at `main` d015e96.
- `cmd/x6configurator/main.go` registers one `*desktop.Service`; `frontend/src/wails-service.ts` forwards its methods to generated Wails bindings. Preserve exported methods and snapshot/event shapes.
- Extract inventory, listener, DPI, polling, lighting and remap responsibilities to focused components; keep selection validation centralized across domains. Polling and lighting share settings with `sleep.go`; reset depends on serialized writes and existing composition.
- Do not hand-edit generated bindings or modify sibling worktrees. Do not redesign UI, remap protocol, reset policy or configuration format.
- Delivery strategy: ask-on-risk; user selected `stacked-to-main` for future PRs. Forecast well over 400 authored diff lines across independent work units. Each cohesive work-unit commit should be reviewable independently; do not open PRs without a separate request. Work units are coherent changes with tests, not size targets.
- Effective ODD TDD: strict, explicitly selected by user in this session. Runner: Go `go test` (focused `./internal/desktop`, plus relevant composition packages); frontend Vitest `npm test` for adapter contract. For each behavior change, record observed RED, GREEN, REFACTOR; do not invent failure evidence. Prepare `frontend/dist` via `cd frontend && npm ci && npm run build` before Go tests when needed by embed.

## Tasks
- [x] DS-1 (delegated): Characterize selected binding, stale binding, per-device isolation and coordination with a failing test; centralize selection resolution without changing observable behavior. Verify focused desktop tests and race-sensitive cases.
- [ ] DS-2 (delegated): Extract inventory and listener ownership; preserve event names/payloads, lock ordering and scheduled-write cancellation. RED/GREEN and focused listener/isolation/composition tests.
- [ ] DS-3 (delegated): Extract DPI stage/apply/retry, state and sync ownership; preserve ACK gate and reset access. RED/GREEN and focused DPI/sync tests.
- [ ] DS-4 (delegated): Extract polling and shared settings coordination; preserve unrelated DeviceConfig fields and sleep integration. RED/GREEN and focused polling/sleep tests.
- [ ] DS-5 (delegated): Extract lighting state/stage/apply, preserving shared sleep/response-time behavior. RED/GREEN and lighting tests.
- [ ] DS-6 (delegated): Extract remap state/apply/retry and serialized device write behavior; preserve validation, ACK and events. RED/GREEN and remap/isolation tests.
- [ ] DS-7 (delegated): Reduce facade to orchestration/delegation, check composition and reset/CLI contracts, verify generated-binding compatibility without hand-editing outputs. RED/GREEN where behavior changes; focused and broad checks.
- [ ] DS-8: Review actual work-unit diffs, run full applicable backend/frontend checks, capture environmental blockers accurately, and leave reviewable branch/worktree for integration with concurrent browser work.

## Acceptance and checks
- Service's public Wails API remains compatible; domain logic and mutable state no longer accumulate in one god object.
- Device-specific operations reject stale bindings; no cross-device state bleed; deterministic lock order and no races.
- Existing events (`mouse:status`, `mouse:configuration`, `mouse:polling-configuration`, `mouse:remap-configuration`), explicit apply and persistence retry semantics remain intact.
- EmergencyReset.Run retains quiesce → DPI/polling/remap ACK → purge ordering; composition and CLI reset keep working.
- Run focused Go tests per unit, desktop race tests, `go test ./...`, `go vet ./...`, and frontend `npm test` after build if environment permits. Record results, not assumptions.

## Progress and next step
2026-09-27: Worktree created clean from main; mapping completed. DS-1 extracted selection resolution into `selection.go` and added contract test. RED: targeted `go test` failed `undefined: selectionResolver` before implementation. GREEN: targeted test passed; 110 desktop tests passed, 30 targeted race tests passed. Independent read-only verifier repeated both commands and found no regression; parent spot-checked the targeted test, gofmt, and diff whitespace. Native pre-commit risk assessment was unassessable due to untracked files, so independent verification was required and completed. User selected stacked-to-main as future chain strategy. DS-1 committed as `7eed45d` and native medium review approved/acknowledged (`review-98433cca594ecce8`); advisory R3-001 is non-blocking. DS-2 inventory half: `inventory.go` now owns refresh/select/cancellation and per-device maps; observed RED (`undefined: service.inventoryComponent`), GREEN, 111 desktop tests and 31 focused race tests passing, repeated by independent verifier; parent spot-check passed. Listener half remains pending; inventory work-unit commit/review boundary pending. Route: one bounded writer per multi-file unit; parent orchestrates and mirrors progress.
