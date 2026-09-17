# Tasks: Mouse Controls Button Remapping

## Review Workload Forecast

| Field | Forecast |
|---|---|
| Expected authored changed lines | 260–360, including production and tests; generated files excluded |
| 400-line budget risk | Medium; measure the implementation diff before delivery |
| Chained PRs | Not expected; stop and request maintainer direction if the budget is exceeded |
| Hardware activity | None; all tests use protocol fixtures and fakes |
| Expected production files | `internal/protocol/x6/remap.go`; `internal/x6/remap.go`; `internal/desktop/service.go`; `frontend/src/desktop-contract.ts`; `frontend/src/components/panels/GnomeSelect.tsx`; `frontend/src/components/panels/ButtonRemapPanel.tsx`; `frontend/src/hooks/useDesktopWorkspace.ts` |
| Expected test files | `internal/protocol/x6/remap_test.go`; `internal/x6/remap_test.go` if adapter coverage is required; `internal/desktop/service_explicit_apply_test.go`; `frontend/src/components/panels/ButtonRemapPanel.test.tsx` |
| Explicitly not changed | `internal/mouse/**`, `internal/hidlinux/**`, `internal/transport/**`, persistence/reset implementations, generated bindings, report infrastructure, captures, and documentation outside this change |

The forecast covers the approved extension only. If actual authored changes exceed 400 lines, pause before delivery preparation and obtain a maintainer decision; do not silently chain or claim an exception.

## RED

- [x] 1. Add table-driven protocol tests for the five exact action values/IDs, explicit catalog order, Buttons 2–7 eligibility, every Button 1 rejection, excluded/unknown values, seven-button shape, non-linear mapping, hidden wire group 4, unchanged parameters, checksum, and exact remap ACK.
- [x] 2. Add adapter/desktop tests for catalog publication, selected binding, Button 1 rejection before pending/applied/revision/config changes, encoding, persistence, transport, and device I/O, ACK failure/success, persistence retry without a second write, discard, and Basic reset regression.
- [x] 3. Add panel/selector tests for seven visible grouped selectors, exact Mouse Controls labels/order, Button 1 disabled pointer/keyboard behavior, Button 2–7 staging, existing-category retention, and Buttons 6/7 marker behavior.

## GREEN

- [x] 4. Extend the closed protocol action set and `ValidateRemapConfig` in `internal/protocol/x6/remap.go`; preserve report construction, mapping, checksum, and ACK handling.
- [x] 5. Re-export the five typed values in `internal/x6/remap.go` and publish the categorized catalog from `internal/desktop/service.go` without changing snapshot/lifecycle contracts.
- [x] 6. Extend the handwritten frontend contract and existing selector/panel with category metadata and accessible disabled options; keep ungrouped `GnomeSelect` callers unchanged.
- [x] 7. Preserve the established `useDesktopWorkspace.ts` staging behavior so an explicit replacement clears only the targeted Button 6/7 preserved-default marker.

## TRIANGULATE

- [x] 8. Cover all 30 eligible button/action combinations (Buttons 2–7 × five actions) at the protocol layer and all five Button 1 rejections through authoritative validation and desktop boundaries.
- [x] 9. Verify exact IDs at each eligible non-linear wire target, report length/header/groups/parameters/checksum, exact ACK, excluded values, and no exposure of an eighth button or wire group 4.
- [x] 10. Verify staging is inert; apply is explicit and ACK-gated; failed ACK leaves applied state unchanged; persistence retry performs no second device write; discard and Basic reset remain unchanged.
- [x] 11. Verify the seven selectors, category/order, accessible disabled semantics, keyboard navigation through the selector trigger, Button 2–7 selection, and targeted DPI-marker clearing.

## REFACTOR

- [x] 12. Consolidate catalog/action metadata and test fixtures only after the full RED/GREEN/TRIANGULATE boundary is green. Keep product order explicit and fail-closed handling obvious.
- [x] 13. Inspect the final implementation diff for the 400-line budget and forbidden generated/transport/HID/persistence/reset/report changes.

## Verification

Run the commands required by `openspec/config.yaml`, one at a time, during implementation:

```text
go test ./...
go test -race ./internal/desktop/...
go vet ./...
cd frontend && npm test
cd frontend && npm run build
```

Compare every implementation scenario in `specs/button-remapping/spec.md` against its test delta before delivery. This planning change itself does not run implementation verification.

## Boundaries

No task may run as root, claim USB interfaces, replay captures against hardware, write hidraw, alter generated bindings, change transport/HID/persistence/reset/report infrastructure, add a physical button, or expose internal wire group 4. Do not edit production code or tests as part of this OpenSpec authoring task.
