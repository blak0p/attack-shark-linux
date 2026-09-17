# Design: Mouse Controls Button Remapping

## Technical Approach

Extend the existing closed remap action model and selector metadata while preserving the current report and apply pipeline. The protocol layer owns the action set and physical-button policy; the desktop layer continues to publish the catalog; the frontend renders category and disabled metadata; staging remains local and explicit apply remains the only write path.

## Authoritative Validation Boundary

Add the five exact action values and IDs to `internal/protocol/x6/remap.go`. Keep validation closed and add a Mouse Controls classifier. `ValidateRemapConfig` MUST reject a Mouse Controls action in Button 1. The rejection must occur before `EncodeRemapReport`, pending/applied/revision mutation, persistence, transport, or device I/O. `internal/x6/remap.go` should only expose the typed adapter/constants; it must not become a second policy authority.

## Catalog and UI

`internal/desktop/service.go` should continue returning the existing snapshot shape while constructing the ordered catalog with a new `Mouse Controls` category representation appropriate to the current frontend contract. Existing Basic and Multimedia membership/order must remain unchanged. `ButtonRemapPanel.tsx` should render exactly five Mouse Controls entries in the specified order. For Button 1, each entry is visible, accessible as disabled, and blocked for pointer and keyboard selection. Buttons 2–7 receive enabled entries. `GnomeSelect.tsx` should preserve existing ungrouped callers and existing keyboard/readiness behavior.

When `useDesktopWorkspace.ts` stages an explicit action for Button 6 or 7, it should clear only that button's `PreservedDefault` marker, matching the current behavior. No unrelated marker, reset default, or applied state may change during staging.

## Protocol and Lifecycle Preservation

Do not change the remap baseline, `remapGroupByButton` mapping `[1, 2, 3, 7, 8, 5, 6]`, report length, 18 wire groups, parameter bytes, checksum range/order, or `MatchesRemapACK`. DPI Cycle (`0x0d`) is assigned through an existing physical slot; wire group 4 remains internal and is not exposed as a button.

Staging MUST produce zero device writes. `ApplyRemap` MUST retain selected-binding validation, one bounded report write, exact ACK matching (`03 10 50 00 08`), ACK-gated applied state, persistence retry without a second hardware write, and discard semantics. Reset remains the existing Basic default path.

## Expected File Plan

| Path | Planned role |
|---|---|
| `internal/protocol/x6/remap.go` | Five constants/IDs, closed-set classifier, Button 1 validation. |
| `internal/protocol/x6/remap_test.go` | IDs, order-independent encoding, eligibility, Button 1 rejection, exclusions, report/ACK invariants. |
| `internal/x6/remap.go` | Typed re-exports and adapter coverage. |
| `internal/x6/remap_test.go` | Adapter delegation and validation coverage, if needed by existing conventions. |
| `internal/desktop/service.go` | Ordered categorized catalog only; preserve apply/persistence state machine. |
| `internal/desktop/service_explicit_apply_test.go` | Catalog, zero-I/O rejection, ACK/persistence/retry/discard and reset regressions. |
| `frontend/src/desktop-contract.ts` | Handwritten action/category contract updates only. |
| `frontend/src/components/panels/GnomeSelect.tsx` | Minimal category/disabled option behavior. |
| `frontend/src/components/panels/ButtonRemapPanel.tsx` | Mouse Controls rendering and Button 1 disabled metadata. |
| `frontend/src/components/panels/ButtonRemapPanel.test.tsx` | Selector order, accessibility, interaction, staging, and marker behavior. |
| `frontend/src/hooks/useDesktopWorkspace.ts` | Preserve established explicit-stage marker clearing. |

Generated bindings, `internal/mouse/**`, `internal/hidlinux/**`, `internal/transport/**`, persistence implementations, reset implementations, and report infrastructure are explicitly unchanged.

## TDD and Verification Shape

Follow `openspec/config.yaml` strict TDD phases: RED records failing behavioral tests; GREEN implements the smallest catalog/validation/UI change; TRIANGULATE covers the full action/button matrix and lifecycle invariants; REFACTOR consolidates helpers only after green. Use the configured Go, race, vet, frontend test, and frontend build commands during implementation; this planning change does not run implementation verification.

## Rollback

Remove only the five action/catalog/selector/validation additions and their tests. Do not alter report encoding, transport, persistence, reset, generated bindings, or hardware behavior.
