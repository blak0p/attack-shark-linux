# Proposal: Add Multimedia Button Remapping

## Why

Users of the Attack Shark X6 Linux configurator can already assign Basic remap actions but cannot configure the capture-backed multimedia actions exposed by the device. This change completes that next action category without changing the established report or apply lifecycle.

## What Changes

- Add exactly eight Multimedia actions to the existing closed remap catalog:
  - Media Player (`media_player`, `0x15`)
  - Play/Pause (`play_pause`, `0x18`)
  - Stop (`stop`, `0x19`)
  - Previous Track (`previous_track`, `0x16`)
  - Next Track (`next_track`, `0x17`)
  - Volume Up (`volume_up`, `0x1b`)
  - Volume Down (`volume_down`, `0x1c`)
  - Mute (`mute`, `0x1a`)
- Present Basic and Multimedia as visible groups in all seven remap selectors. Multimedia order follows the product order above, not numeric IDs.
- Allow Multimedia actions on Buttons 2–7.
- Keep the Multimedia group visible for Button 1 but render every entry unavailable. Reject any bypassed Button 1 Multimedia assignment in authoritative backend validation before encoding, state advance, persistence, or device I/O.

## Invariants

The following remain unchanged:

- Seven-button remap model and non-linear wire-group mapping `[1, 2, 3, 7, 8, 5, 6]`.
- 59-byte `0x08` report format, header, eighteen groups, simple-action parameters, checksum, and strict ACK.
- Explicit `ApplyRemap` as the only device-write authorization.
- ACK-gated applied state, selected binding validation, persistence retry, discard, DPI preserved-default markers, and Basic factory-reset defaults.
- Existing Basic actions and their order.
- Pure protocol layering, hidraw/transport behavior, Wails method shapes, and generated bindings.

## Out of Scope

Browser/system actions, keyboard shortcuts, macros, host-side media integration, protocol redesign, transport changes, persistence/reset semantic changes, generated binding edits, hardware writes, and unrelated UI redesign.

## Affected Areas

- `internal/protocol/x6/remap.go` and tests
- `internal/x6/remap.go` and tests
- `internal/desktop/service.go` and explicit-apply tests
- `frontend/src/desktop-contract.ts`
- `frontend/src/components/panels/GnomeSelect.tsx`
- `frontend/src/components/panels/ButtonRemapPanel.tsx` and tests
- `frontend/src/styles/controls.css`
- Existing frontend test fixtures only where the action union is exhaustive

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Button 1 policy exists only in the UI | Enforce it in pure protocol validation before report encoding and I/O. |
| Action order follows numeric IDs | Use explicit ordered catalogs and exact-order tests. |
| Selector grouping expands review scope | Keep the existing selector, add only optional group/disabled support, and stop for `ask-on-risk` if scope exceeds 400 lines. |
| Host behavior differs across environments | Claim only device configuration using capture-backed IDs. |

## Success Criteria

- Every selector retains the Basic actions and displays exactly eight ordered Multimedia actions.
- Buttons 2–7 can explicitly apply every Multimedia action using its exact wire ID.
- Button 1 cannot stage or apply any Multimedia action through either UI or backend.
- Existing report, ACK, apply, persistence, discard, and reset contracts pass regression tests unchanged.
- Excluded values remain unavailable and fail closed.
- Hardware-free protocol, desktop, and frontend tests cover the new catalog and safety rule.

## Rollback

Remove the Multimedia catalog, validation branch, selector grouping/disabled metadata, and related tests. Rollback performs no device write, persistence rewrite, reset change, or generated-binding edit. Persisted Multimedia values then fail closed until the user explicitly applies a supported Basic mapping.
