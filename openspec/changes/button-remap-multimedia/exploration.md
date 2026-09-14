# Exploration: Multimedia Button Remapping

## Status

This OpenSpec change ports the maintainer-approved planning state previously recorded in Engram. It makes no product-source, test, generated-binding, Git, or hardware change.

## Baseline

The delivered remap path already spans:

- `internal/protocol/x6/remap.go`
- `internal/x6/remap.go`
- `internal/desktop/service.go`
- `frontend/src/desktop-contract.ts`
- `frontend/src/wails-service.ts`
- `frontend/src/hooks/useDesktopWorkspace.ts`
- `frontend/src/components/panels/ButtonRemapPanel.tsx`

`RemapConfig` contains exactly seven physical buttons. Application button order maps to wire groups `[1, 2, 3, 7, 8, 5, 6]`; Buttons 6 and 7 may retain their `DPI+`/`DPI-` preserved-default markers. The default remap and factory reset remain Basic-only.

The existing remap feature report is exactly 59 bytes: header `08 3b 01`, eighteen three-byte action groups, and a big-endian additive checksum over bytes `[3:57]` in bytes `[57:59]`. The expected acknowledgement is `03 10 50 00 08`. The existing explicit `ApplyRemap` path validates before encoding or I/O, targets the selected binding, advances applied state only after the ACK, and retains the existing persistence-retry behavior without a second device write.

## Evidence

Checked-in capture evidence and the decoder establish the global multimedia IDs:

| Product action | Stable value | Wire ID |
| --- | --- | ---: |
| Media Player | `media_player` | `0x15` |
| Play/Pause | `play_pause` | `0x18` |
| Stop | `stop` | `0x19` |
| Previous Track | `previous_track` | `0x16` |
| Next Track | `next_track` | `0x17` |
| Volume Up | `volume_up` | `0x1b` |
| Volume Down | `volume_down` | `0x1c` |
| Mute | `mute` | `0x1a` |

Evidence locations: `captures/0x08-remap/btn6_multimedia.pcapng`, `docs/protocol-captures.md`, and `tools/decode_remap.py`. IDs are global across physical button slots; the product order intentionally differs from numeric wire-ID order.

## Approved Product Boundaries

- Include exactly the eight multimedia actions above.
- Every selector visibly presents Basic and Multimedia groups while retaining the Basic order.
- Buttons 2–7 may select all eight actions.
- Button 1 must show every Multimedia option disabled and the backend must reject a bypassed Button 1 multimedia assignment before encoding, state advance, persistence, or device I/O.
- Browser/system actions, shortcuts, macros, host multimedia integration, report-format changes, transport changes, persistence changes, reset-default changes, hardware writes, and generated-binding edits are excluded.

## Recommended Bounded Approach

1. Extend the pure closed remap action model and explicit action-ID mapping in `internal/protocol/x6/remap.go`.
2. Enforce the Button 1 multimedia restriction in `ValidateRemapConfig`, preserving it as the authoritative pre-I/O boundary.
3. Re-export the new action values through `internal/x6/remap.go` and add the ordered Basic-plus-Multimedia action catalog in `internal/desktop/service.go` without changing the snapshot shape.
4. Extend the handwritten TypeScript union and add grouped/disabled option support to the existing selector. The UI guard is defense in depth; it is not the authority.
5. Use hardware-free Go and frontend tests for exact IDs, all eligible buttons, Button 1 rejection, grouped presentation, disabled pointer/keyboard behavior, and report/lifecycle regressions.

## Risks

- A UI-only Button 1 restriction can be bypassed; validation must reject before encoding and I/O.
- Numeric sorting would break the requested product order.
- Grouped-selector keyboard behavior can broaden the frontend surface and approach the 400-line review budget.
- Host operating-system handling of media controls is not validated or claimed.

## Ready for Proposal

Yes. The protocol evidence, product decisions, source boundaries, safety rule, non-goals, and test direction are resolved.
