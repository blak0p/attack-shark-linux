# Design: Multimedia Button Remapping

## Technical Approach

Extend the existing closed remap action model and existing selector without changing report transport, persistence, Wails method shapes, or generated bindings.

### Authoritative Safety Rule

Add the eight Multimedia action constants and exact ID cases to `internal/protocol/x6/remap.go`. Keep `isRemapAction` as the closed-set gate and add an unexported `isMultimediaRemapAction` helper. In `ValidateRemapConfig`, after button-order and supported-action checks, reject Button 1 when its action is Multimedia.

This is the authority boundary: `EncodeRemapReport`, the X6 operation validator, desktop apply, and targeted HID operations all validate before encoding and I/O. A rejected Button 1 request therefore has no report, no device call, and no pending/applied/revision/persistence advance beyond existing failure-status handling.

### Protocol Preservation

Do not change `remapBaseline`, `remapGroupByButton`, report length, header, checksum implementation, ACK matcher, hidraw allowlist, transport, USB ownership, or timeout behavior. Only an eligible selected action byte and its checksum may differ.

### Desktop Catalog

Keep `RemapSnapshot.Actions []x6.RemapAction` unchanged. In `remapSnapshotLocked`, create a fresh Basic block followed by a fresh Multimedia block for every snapshot. This preserves backend ownership of membership/order and avoids a DTO-shape or generated-binding change.

### Frontend Presentation

Extend the handwritten `RemapAction` union in `frontend/src/desktop-contract.ts`. In `ButtonRemapPanel.tsx`, use exhaustive label/category records, preserve snapshot order, and pass `group` plus `disabled` metadata to the existing `GnomeSelect`. Button 1 marks only Multimedia entries disabled and locally guards `onChange`; backend validation remains authoritative.

Extend `GnomeSelectOption` with optional `group` and `disabled` fields. Ungrouped callers must remain unchanged. Grouped options render an accessible visible group heading. Disabled options use `aria-disabled`, reject click/Enter/Space selection, and navigation/Home/End skips them while retaining existing cleanup, Escape, readiness, and wrap behavior. Add only focused group/disabled CSS in `frontend/src/styles/controls.css`.

### File Plan

| Path | Change |
| --- | --- |
| `internal/protocol/x6/remap.go` | Eight constants/IDs, closed-set extension, Button 1 validator rule. |
| `internal/protocol/x6/remap_test.go` | Exact IDs, all eligible buttons, Button 1 rejection, report/checksum/ACK and exclusion tests. |
| `internal/x6/remap.go` and tests | Re-export values and retain typed delegation tests. |
| `internal/desktop/service.go` | Fresh Basic-then-Multimedia snapshot catalog only. |
| `internal/desktop/service_explicit_apply_test.go` | Catalog, binding, zero-I/O rejection, ACK/persistence/retry assertions. |
| `frontend/src/desktop-contract.ts` | Eight string-union values only. |
| `frontend/src/components/panels/GnomeSelect.tsx` | Optional group/disabled option support. |
| `frontend/src/components/panels/ButtonRemapPanel.tsx` and tests | Metadata, Button 1 mirror, grouped/disabled interaction coverage. |
| `frontend/src/hooks/useDesktopWorkspace.test.ts`, `frontend/src/App.test.tsx` | Exhaustive fixture updates and local-stage/discard regression coverage. |
| `frontend/src/styles/controls.css` | Minimal group and disabled styling. |

Explicitly unchanged: `frontend/src/wails-service.ts`, production `App.tsx`, `internal/mouse/**`, `internal/hidlinux/**`, `internal/transport/**`, `cmd/x6configurator/**`, and both generated binding trees.

## TDD Plan

1. **RED**: write behavioral protocol, desktop, and panel tests using raw action values where needed; record missing action/policy/group behavior rather than intentionally uncompilable tests.
2. **GREEN**: implement only the constants, validation, catalog, union/metadata, selector behavior, and CSS required by RED.
3. **TRIANGULATE**: cover all 48 eligible button/action combinations, all Button 1 rejections, non-linear buttons, unknown/excluded values, snapshot-copy safety, pointer/keyboard disabled behavior, DPI-marker clearing, ACK failure, persistence retry, discard, and reset.
4. **REFACTOR**: consolidate fixtures/helpers only after green boundary coverage; retain explicit product ordering.

## Verification

```text
go test ./internal/protocol/x6 -run Remap
go test ./internal/x6 -run Remap
go test ./internal/desktop -run 'Remap|Reset'
(cd frontend && npm test -- src/components/panels/ButtonRemapPanel.test.tsx src/hooks/useDesktopWorkspace.test.ts src/App.test.tsx)
go test ./...
go test -race ./internal/desktop/...
go vet ./...
(cd frontend && npm test)
(cd frontend && npm run build)
```

The change is forecast at 320–380 authored lines. Under `ask-on-risk`, stop before apply if actual scope exceeds 400 lines rather than silently choosing a chain or exception.

## Rollback

Remove only the eight action values/mappings, catalog entries, selector grouping/disabled support, panel metadata, CSS, and related tests. Do not write the device, rewrite persistence, change reset defaults, or edit generated bindings.
