# Apply Progress: Multimedia Button Remapping

## Status

- Phase status consumed: `applyState: ready` for `button-remap-multimedia`.
- Action context: `repo-local`; all edits are within `/home/alejandro/dev/attack_shark_linux`.
- Strict TDD is active. The parent-owned native runtime attempt was not acquired, settled, persisted, or otherwise handled.
- Maintainer explicitly approved `size:exception` and an audited objective reset. The active native cap is 550 code/test changed lines; the final measured diff is 330 additions + 122 deletions = 452 changed lines.

## Completed Tasks

- [x] 1. Protocol RED/triangulation coverage now exercises eight exact IDs, all 48 eligible Button 2–7 assignments, all Button 1 multimedia rejections, non-linear offsets, report shape/checksum, closed-set rejection, and strict ACK behavior.
- [x] 2. X6/desktop boundary coverage includes typed operation delegation, ordered fresh catalog copies, Button 1 zero-I/O rejection, selected binding, ACK failure, persistence retry, and one-write behavior.
- [x] 4. Added the eight closed multimedia protocol actions and Button 1 authoritative validation before encoding.
- [x] 5. Re-exported multimedia values through X6 and added a fresh Basic-then-Multimedia desktop snapshot catalog.
- [x] 6. Added optional grouped/disabled GnomeSelect options, inaccessible selection rejection, and disabled-keyboard skipping.
- [x] 7. Added contract values, exhaustive presentation labels, Button 1 UI defense, group metadata, and focused styles.

The corresponding checkboxes were updated in `tasks.md` and re-read after this progress update. Remaining unchecked tasks are listed below.

## TDD Cycle Evidence

| Cycle | RED evidence | GREEN evidence | TRIANGULATE / REFACTOR evidence |
| --- | --- | --- | --- |
| Protocol | `go test ./internal/protocol/x6 -run Remap` failed because each multimedia value was rejected as unsupported. | The same focused protocol suite passed after closed-set IDs and Button 1 validation were added. | It now covers all 48 eligible button/action combinations and eight Button 1 rejections; touched Go sources/tests were formatted with `gofmt`. |
| Panel | `cd frontend && npm test -- src/components/panels/ButtonRemapPanel.test.tsx` failed because the Multimedia group was absent. | The focused panel test passed after grouped disabled options and labels were implemented. | The panel test now verifies Button 1 pointer rejection and End-key skipping to the final enabled Basic option, then Button 2 staging. |
| Desktop | Existing focused boundary tests were extended for Button 1 zero-I/O rejection and snapshot-copy/order safety. | `go test ./internal/desktop -run 'Remap|Reset'` passed. | Existing ACK, binding, persistence retry, and one-write cases remain green. |

## Files Changed

- `internal/protocol/x6/remap.go`
- `internal/protocol/x6/remap_test.go`
- `internal/x6/remap.go`
- `internal/desktop/service.go`
- `internal/desktop/service_explicit_apply_test.go`
- `frontend/src/desktop-contract.ts`
- `frontend/src/components/panels/GnomeSelect.tsx`
- `frontend/src/components/panels/ButtonRemapPanel.tsx`
- `frontend/src/components/panels/ButtonRemapPanel.test.tsx`
- `frontend/src/styles/controls.css`
- `openspec/changes/button-remap-multimedia/tasks.md`
- `openspec/changes/button-remap-multimedia/apply-progress.md`

No generated bindings, Wails facade, production `App.tsx`, transport, hidraw, mouse, command, hardware, root, or executable-inspection surfaces were touched.

## Test Commands Run

- `go test ./internal/protocol/x6 -run Remap` — failed in RED, then passed in GREEN and TRIANGULATE.
- `go test ./internal/x6 -run Remap` — passed.
- `go test ./internal/desktop -run 'Remap|Reset'` — passed.
- `cd frontend && npm test -- src/components/panels/ButtonRemapPanel.test.tsx` — failed in RED, then passed in GREEN and TRIANGULATE.
- `cd frontend && npm test -- src/components/panels/ButtonRemapPanel.test.tsx src/hooks/useDesktopWorkspace.test.ts src/App.test.tsx` — passed (3 files, 56 tests).

## Remaining Tasks

- [ ] 3. Add panel behavior tests in `frontend/src/components/panels/ButtonRemapPanel.test.tsx` for seven visible grouped selectors, exact label/order, Button 1 disabled pointer/keyboard behavior, Button 2 enabled staging, Basic retention, DPI markers, apply, and discard.
- [ ] 8. Expand backend boundary coverage for every eligible button/action combination, every Button 1 rejection, excluded values, snapshot-copy isolation, exact binding/report bytes, ACK failure, persistence retry, discard, and Basic reset regression.
- [ ] 9. Expand frontend coverage for disabled pointer/keyboard navigation, all seven grouped selectors, local staging without writes, explicit apply forwarding, discard restore, and Buttons 6–7 DPI-marker clearing.
- [ ] 10. Consolidate fixtures/helpers after boundary tests pass; format only touched files; inspect for forbidden generated/transport/hardware changes; run focused tests, `go test ./...`, `go test -race ./internal/desktop/...`, `go vet ./...`, full frontend tests, and frontend build.

## Workload / PR Boundary

The maintainer approved `size:exception`; `git diff --numstat` reports 452 changed code/test lines, within the parent-provided 550-line native cap. No commit, PR, push, review, hardware, root, or native-attempt operation was performed.

## Deviations

No design deviation was made. The 400-line product budget was exceeded only under the maintainer-approved exception; the final diff remains below the 550-line native cap.

## Resumed Completion

- [x] 3. Added panel coverage for all seven selectors, exact Basic/Multimedia group order, exact multimedia labels in product order, and Button 1 versus Buttons 2–7 disabled semantics.
- [x] 8. Added excluded-action triangulation and confirmed invalid Button 1 Multimedia application leaves pending, applied, and revision state unchanged before I/O; existing table cases retain ACK, binding, persistence retry, and one-write coverage.
- [x] 9. Completed frontend triangulation: all seven groups, Button 1 pointer rejection and End-key disabled-option skipping, Button 2 staging, Basic retention, DPI labels, explicit apply, and discard behavior are covered.
- [x] 10. Formatted touched Go files, checked the diff for whitespace and forbidden surfaces, and completed all required verification.

### Final Verification

- `go test ./...` — passed.
- `go test -race ./internal/desktop/...` — passed.
- `go vet ./...` — passed.
- `cd frontend && npm test` — passed (8 files, 81 tests).
- `cd frontend && npm run build` — passed.
- `git diff --check` — passed.

### Additional TDD Evidence

| Task | Test layer | Safety net | RED / prior GREEN | TRIANGULATE | REFACTOR |
| --- | --- | --- | --- | --- | --- |
| 3 | React component | 4 focused panel tests passed before extension | Parent RED was recorded; grouped UI GREEN already passed | Added seven-selector product-order and disabled-semantics coverage; 5 focused panel tests passed | No production refactor required. |
| 8 | Go unit/desktop boundary | Focused remap and desktop suites passed before extension | Parent RED and existing GREEN were preserved | Added excluded action cases and no pending/revision advance validation; focused desktop/protocol suites passed | `gofmt` applied to touched Go tests. |
| 9 | React component | 4 focused panel tests passed before extension | Parent RED and grouped selector GREEN were preserved | Existing pointer/keyboard, stage/apply/discard, and DPI-marker cases plus new all-selector coverage passed | No production refactor required. |
| 10 | Verification | Focused suites passed | N/A | Full Go, race, vet, frontend test, and build commands passed | Diff check passed; no further refactor needed. |

All implementation tasks are now visibly checked in `tasks.md`.
