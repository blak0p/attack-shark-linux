# Tasks: Multimedia Button Remapping

## Review Workload Forecast

| Field | Value |
| --- | --- |
| Estimated changed lines | 320–380, including production, tests, and CSS |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Delivery strategy | ask-on-risk |

If the measured diff exceeds 400 lines, stop before delivery preparation and request a maintainer decision. Do not silently choose chaining or a size exception.

## RED

- [x] 1. Add table-driven protocol tests in `internal/protocol/x6/remap_test.go` for all eight stable values/IDs, all 48 eligible Button 2–7/action combinations, all Button 1 rejections, excluded/unknown values, exact non-linear offsets, report shape, zero parameters, checksum, and ACK.
- [x] 2. Add X6 and desktop boundary tests in `internal/x6/remap_test.go` and `internal/desktop/service_explicit_apply_test.go` for operation delegation, Basic-plus-Multimedia catalog order, zero-I/O Button 1 rejection, selected binding, ACK gate, persistence failure, retry, and one-write behavior.
- [x] 3. Add panel behavior tests in `frontend/src/components/panels/ButtonRemapPanel.test.tsx` for seven visible grouped selectors, exact label/order, Button 1 disabled pointer/keyboard behavior, Button 2 enabled staging, Basic retention, DPI markers, apply, and discard.

## GREEN

- [x] 4. Extend the pure closed protocol model in `internal/protocol/x6/remap.go` with eight action constants, exact ID cases, an unexported Multimedia classifier, and Button 1 Multimedia rejection in `ValidateRemapConfig`; do not alter report construction or ACK handling.
- [x] 5. Re-export the action values in `internal/x6/remap.go` and create a fresh Basic-then-Multimedia action catalog in `internal/desktop/service.go`; preserve snapshot shape, state, locks, persistence, and apply ordering.
- [x] 6. Add optional `group` and `disabled` support to `frontend/src/components/panels/GnomeSelect.tsx`; preserve ungrouped callers and ensure disabled options are visible, accessible, not selectable, and skipped by keyboard navigation.
- [x] 7. Extend `frontend/src/desktop-contract.ts`, `ButtonRemapPanel.tsx`, and `frontend/src/styles/controls.css` with exhaustive action metadata, Button 1 defense-in-depth, accessible groups/disabled styling, and only necessary typed test-fixture updates.

## TRIANGULATE

- [x] 8. Expand backend boundary coverage for every eligible button/action combination, every Button 1 rejection, excluded values, snapshot-copy isolation, exact binding/report bytes, ACK failure, persistence retry, discard, and Basic reset regression.
- [x] 9. Expand frontend coverage for disabled pointer/keyboard navigation, all seven grouped selectors, local staging without writes, explicit apply forwarding, discard restore, and Buttons 6–7 DPI-marker clearing.

## REFACTOR AND VERIFY

- [x] 10. Consolidate fixtures/helpers after boundary tests pass; format only touched files; inspect for forbidden generated/transport/hardware changes; run focused tests, `go test ./...`, `go test -race ./internal/desktop/...`, `go vet ./...`, full frontend tests, and frontend build.

## Boundaries

Do not edit generated binding trees, `frontend/src/wails-service.ts`, production `App.tsx`, `internal/mouse/**`, `internal/hidlinux/**`, `internal/transport/**`, or `cmd/x6configurator/**`. No task runs hardware commands, root processes, capture replay, USB claims, or live-device tests.
