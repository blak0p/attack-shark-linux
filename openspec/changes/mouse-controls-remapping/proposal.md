# Proposal: Add Mouse Controls Button Remapping

## Why

The X6 remap catalog has checked-in capture evidence for five device actions that are not yet offered by the configurator. This change exposes those actions without changing the established seven-button model, report lifecycle, or transport boundary.

## What Changes

- Add exactly five capture-backed actions to the remap catalog, in this product order:
  1. Scroll Up — `scroll_up` — `0x09`
  2. Scroll Down — `scroll_down` — `0x0a`
  3. DPI Cycle — `dpi_cycle` — `0x0d`
  4. DPI+ — `dpi_plus` — `0x0e`
  5. DPI− — `dpi_minus` — `0x0f`
- Display them in a new `Mouse Controls` selector category.
- Allow all five actions on Buttons 2–7.
- Keep Button 1's current assignment and show all five Mouse Controls entries as visibly disabled.
- Reject a bypassed Button 1 assignment in authoritative backend validation before state mutation, report encoding, persistence, transport, or I/O.
- Clear the existing Buttons 6/7 preserved-default markers only when a user stages an explicit replacement, using the established staging behavior.

## Invariants

The following remain unchanged:

- Exactly seven physical remap slots; DPI Cycle is an action for an existing slot, not an eighth button or exposed wire group 4.
- Existing Basic and Multimedia catalogs, labels, order, and behavior.
- The 59-byte `0x08` report, non-linear physical-to-wire mapping, parameters, checksum, and exact remap ACK.
- Explicit apply, ACK-gated applied state, persistence retry, discard, and reset lifecycle.
- Protocol, transport, HID, persistence, generated-binding, and reset contracts.

## Out of Scope

Browser/system actions, shortcuts, Easy Aim, macros, unknown values, internal wire group 4, an eighth button, report/infrastructure/lifecycle redesign, generated bindings, transport/HID/persistence/reset changes, and live hardware operations.

## Evidence and Affected Areas

All five IDs are documented in `docs/protocol-captures.md` and have checked-in capture support under `captures/0x08-remap/`, including `btn6_scroll_up.pcapng`, `btn6_scroll_down.pcapng`, and `btn6_dpi_cycle_plus_minus.pcapng`.

Expected implementation surfaces are `internal/protocol/x6/remap.go`, `internal/x6/remap.go`, `internal/desktop/service.go`, `frontend/src/desktop-contract.ts`, `frontend/src/components/panels/GnomeSelect.tsx`, `frontend/src/components/panels/ButtonRemapPanel.tsx`, `frontend/src/hooks/useDesktopWorkspace.ts`, and the corresponding existing protocol, desktop, and panel tests. These are forecast only; this change edits no production code or tests.

## Success Criteria

- Every selector retains existing Basic and Multimedia content and adds exactly one ordered Mouse Controls category.
- Buttons 2–7 can stage and apply each of the five exact IDs.
- Button 1 displays but cannot select any Mouse Controls entry, and backend bypasses fail before all listed side effects.
- Existing report, ACK, apply, retry, discard, reset, and DPI-marker behavior remains unchanged.

## Rollback and Hardware Safety

Rollback removes only the five catalog values, selector metadata/disabled presentation, validation rule, and their implementation tests. It performs no device write and does not rewrite persisted state. Planning and tests must use hardware-free fakes/fixtures: never run as root, claim USB interfaces, replay captures against hardware, or perform hidraw I/O. No code path may write without the existing explicit apply action and exact ACK gate.
